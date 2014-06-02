package hrotti

import (
	"bufio"
	//"errors"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	// Plugins currently don't work (they create a cycle). We could break the cycle
	// by fudging things through main.go, but I think the real solution is to use RPC
	// and run plugins in a separate process
	// . "github.com/alsm/hrotti/plugins"
)

type Client struct {
	sync.WaitGroup
	MessageIDs
	clientID         string
	conn             net.Conn
	bufferedConn     *bufio.ReadWriter
	rootNode         *Node
	keepAlive        uint16
	state            State
	topicSpace       string
	outboundMessages chan *publishPacket
	outboundPriority chan ControlPacket
	stop             chan struct{}
	stopOnce         *sync.Once
	resetTimer       chan bool
	cleanSession     bool
	willMessage      *publishPacket
	takeOver         bool
}

func NewClient(conn net.Conn, bufferedConn *bufio.ReadWriter, clientID string, maxQDepth int) *Client {
	return &Client{
		conn:             conn,
		bufferedConn:     bufferedConn,
		clientID:         clientID,
		stop:             make(chan struct{}),
		resetTimer:       make(chan bool, 1),
		outboundMessages: make(chan *publishPacket, maxQDepth),
		outboundPriority: make(chan ControlPacket, maxQDepth),
		rootNode:         rootNode,
		stopOnce:         new(sync.Once),
		MessageIDs: MessageIDs{
			idChan: make(chan msgID, 10),
			index:  make(map[msgID]bool),
		},
	}
}

func (c *Client) Connected() bool {
	return c.state.Value() == CONNECTED
}

func (c *Client) KeepAliveTimer(hrotti *Hrotti) {
	//this function is part of the Client's waitgroup so call Done() when the function exits
	defer c.Done()
	//In a continuous loop create a Timer for 1.5 * the keepAlive setting
	for {
		t := time.NewTimer(time.Duration(float64(c.keepAlive)*1.5) * time.Second)
		//this select will block on all 3 cases until one of them is ready
		select {
		//if we get a value in on the resetTimer channel we drop out, stop the Timer then loop round again
		case _ = <-c.resetTimer:
		//if the timer triggers then the client has failed to send us a packet in the keepAlive period so
		//must be disconnected, we call Stop() and the function returns.
		case <-t.C:
			ERROR.Println(c.clientID, "has timed out", c.keepAlive)
			go c.Stop(true, hrotti)
			return
		//the client sent a DISCONNECT or some error occurred that triggered the client to stop, so return.
		case <-c.stop:
			return
		}
		t.Stop()
	}
}

func (c *Client) StopForTakeover() {
	//close the stop channel, close the network connection, wait for all the goroutines in the waitgroup to
	//finish, set the conn and bufferedconn to nil
	c.takeOver = true
	close(c.stop)
	c.conn.Close()
	c.Wait()
	c.conn = nil
	c.bufferedConn = nil
	c.takeOver = false
}

func (c *Client) Stop(sendWill bool, hrotti *Hrotti) {
	//Its possible that error conditions with the network connection might cause both Send and Receive to
	//try and call Stop(), but we only want it to be called once, so using the sync.Once in the client we
	//run the embedded function, later calls with the same sync.Once will simply return.
	if !c.takeOver {
		c.stopOnce.Do(func() {
			//close the stop channel, close the network connection, wait for all the goroutines in the waitgroup
			//set the state as disconnected, close the message channels.
			close(c.stop)
			c.conn.Close()
			c.Wait()
			c.state.SetValue(DISCONNECTED)
			close(c.outboundMessages)
			close(c.outboundPriority)
			//If we've stopped in a situation where the will message should be sent, and there is a will
			//message, then send it.
			if sendWill && c.willMessage != nil {
				INFO.Println("Sending will message for", c.clientID)
				go c.rootNode.DeliverMessage(strings.Split(c.willMessage.topicName, "/"), c.willMessage, hrotti)
			}
			//if this client connected with cleansession true it means it does not need its state (such as
			//subscriptions, unreceived messages etc) kept around
			if c.cleanSession {
				//so we lock the clients map, delete the clientid and *Client from the map, remove all subscriptions
				//associated with this client, from the normal tree and any plugins. Then close the persistence
				//store that it was using.
				hrotti.clients.Lock()
				delete(hrotti.clients.list, c.clientID)
				hrotti.clients.Unlock()
				c.rootNode.DeleteSubAll(c)
				hrotti.inboundPersist.Close(c)
				hrotti.outboundPersist.Close(c)
			}
		})
	}
}

func (c *Client) Start(cp *connectPacket, hrotti *Hrotti) {
	//If cleansession was set to 1 in the CONNECT packet set as true in the Client.
	if cp.cleanSession == 1 {
		c.cleanSession = true
	}
	//There is a will message in the connect packet, so construct the publish packet that will be sent if
	//the will is triggered.
	if cp.willFlag == 1 {
		pp := New(PUBLISH).(*publishPacket)
		pp.FixedHeader.Qos = cp.willQos
		pp.FixedHeader.Retain = cp.willRetain
		pp.topicName = cp.willTopic
		pp.payload = cp.willMessage

		c.willMessage = pp
	} else {
		c.willMessage = nil
	}
	c.keepAlive = cp.keepaliveTimer

	//If cleansession true, or there doesn't already exist a persistence store for this client (ie a new
	//durable client), create the inbound and outbound persistence stores.
	if c.cleanSession || !hrotti.inboundPersist.Exists(c) || !hrotti.outboundPersist.Exists(c) {
		hrotti.inboundPersist.Open(c)
		hrotti.outboundPersist.Open(c)
	} else {
		//we have an existing inbound and outbound persistence store for this client already, so lets
		//get any messages still in outbound and attempt to send them.
		INFO.Println("Getting unacknowledged messages from persistence")
		for _, msg := range hrotti.outboundPersist.GetAll(c) {
			switch msg.Type() {
			//If the message in the store is a publish packet
			case PUBLISH:
				//It's possible we already sent this message and didn't remove it from the store because we
				//didn't get an acknowledgement, so set the dup flag to 1. (only for QoS > 0)
				if msg.(*publishPacket).Qos > 0 {
					msg.(*publishPacket).Dup = 1
				}
				c.outboundMessages <- msg.(*publishPacket)
			//If it's something else like a PUBACK etc send it to the priority outbound channel
			default:
				c.outboundPriority <- msg
			}
		}
	}

	//Prepare and write the CONNACK packet.
	ca := New(CONNACK).(*connackPacket)
	ca.returnCode = CONN_ACCEPTED
	c.conn.Write(ca.Pack())
	//getMsgIDs, Receive and Send are part of this WaitGroup, so add 3 to the waitgroup and run the goroutines.
	c.Add(3)
	go c.genMsgIDs()
	go c.Receive(hrotti)
	go c.Send(hrotti)
	c.state.SetValue(CONNECTED)
	//If keepalive value was set run the keepalive time and add 1 to the waitgroup.
	if c.keepAlive > 0 {
		c.Add(1)
		go c.KeepAliveTimer(hrotti)
	}
}

func validateClientID(clientID string) bool {
	return true
}

func (c *Client) ResetTimer() {
	//If we're using keepalive on this client attempt to reset the timer, if the channel blocks it's because
	//the timer is already being reset so we can safely drop the attempt here (the default case of the select)
	if c.keepAlive > 0 {
		select {
		case c.resetTimer <- true:
		default:
		}
	}
}

func (c *Client) SetRootNode(node *Node) {
	c.rootNode = node
}

func (c *Client) Receive(hrotti *Hrotti) {
	//part of the client waitgroup so call Done() when the function returns.
	defer c.Done()
	//loop forever...
	for {
		select {
		//if called to stop then return
		case <-c.stop:
			return
		//otherwise...
		default:
			var cph FixedHeader
			var err error
			var body []byte
			var typeByte byte
			//the msgType will always be the first byte read from the network.
			typeByte, err = c.bufferedConn.ReadByte()
			//if there was an error reading from the network, print it and call stop
			//true here means send the will message, if there is one, and return.
			if err != nil {
				ERROR.Println(err.Error(), c.clientID, c.conn.RemoteAddr())
				go c.Stop(true, hrotti)
				return
			}
			//we've received a message so reset the keepalive timer.
			c.ResetTimer()
			//unpack the first byte into the fixedHeader and read the remaining length
			cph.unpack(typeByte)
			cph.remainingLength = decodeLength(c.bufferedConn)
			//if the remaining length is > 0 then there is more to read for this packet so
			//make the body slice the size of the remaining data. readfull will not return
			//until the target slice is full or there was an error
			if cph.remainingLength > 0 {
				body = make([]byte, cph.remainingLength)
				_, err = io.ReadFull(c.bufferedConn, body)
				//if there was an error (such as broken network), call Stop (send will message)
				//and return.
				if err != nil {
					go c.Stop(true, hrotti)
					return
				}
			}
			//MQTT allows large messages that could take a long time to receive, ideally here
			//we should pause the keepalive timer, for now we just reset the timer again once
			//we've recevied the message.
			c.ResetTimer()
			//switch on the type of message we've received
			switch cph.MessageType {
			//a second CONNECT packet is a protocol violation, so Stop (send will) and return.
			case CONNECT:
				ERROR.Println("Received second CONNECT from", c.clientID)
				go c.Stop(true, hrotti)
				return
			//client wishes to disconnect so Stop (don't send will) and return.
			case DISCONNECT:
				INFO.Println("Received DISCONNECT from", c.clientID)
				go c.Stop(false, hrotti)
				return
			//client has sent us a PUBLISH message, unpack it persist (if QoS > 0) in the inbound store
			case PUBLISH:
				pp := New(PUBLISH).(*publishPacket)
				pp.FixedHeader = cph
				pp.Unpack(body)
				PROTOCOL.Println("Received PUBLISH from", c.clientID, pp.topicName)
				if pp.Qos > 0 {
					hrotti.inboundPersist.Add(c, pp)
				}
				//if this message has the retained flag set then set as the retained message for the
				//appropriate node in the topic tree
				if pp.Retain == 1 {
					c.rootNode.SetRetained(strings.Split(pp.topicName, "/"), pp)
				}
				//go and deliver the message to any subscribers.
				go c.rootNode.DeliverMessage(strings.Split(pp.topicName, "/"), pp, hrotti)
				//if the message was QoS1 or QoS2 start the acknowledgement flows.
				switch pp.Qos {
				case 1:
					pa := New(PUBACK).(*pubackPacket)
					pa.messageID = pp.messageID
					c.HandleFlow(pa, hrotti)
				case 2:
					pr := New(PUBREC).(*pubrecPacket)
					pr.messageID = pp.messageID
					c.HandleFlow(pr, hrotti)
				}
			//We received a PUBACK acknowledging a QoS1 PUBLISH we sent to the client
			case PUBACK:
				pa := New(PUBACK).(*pubackPacket)
				pa.FixedHeader = cph
				pa.Unpack(body)
				//Check that we also think this message id is in use, if it is remove the original
				//PUBLISH from the outbound persistence store and set the message id as free for reuse
				if c.inUse(pa.messageID) {
					hrotti.outboundPersist.Delete(c, pa.messageID)
					c.freeID(pa.messageID)
				} else {
					ERROR.Println("Received a PUBACK for unknown msgid", pa.messageID, "from", c.clientID)
				}
			//We received a PUBREC for a QoS2 PUBLISH we sent to the client.
			case PUBREC:
				pr := New(PUBREC).(*pubrecPacket)
				pr.FixedHeader = cph
				pr.Unpack(body)
				//Check that we also think this message id is in use, if it is run the next stage of the
				//message flows for QoS2 messages.
				if c.inUse(pr.messageID) {
					prel := New(PUBREL).(*pubrelPacket)
					prel.messageID = pr.messageID
					c.HandleFlow(prel, hrotti)
				} else {
					ERROR.Println("Received a PUBREC for unknown msgid", pr.messageID, "from", c.clientID)
				}
			//We received a PUBREL for a QoS2 PUBLISH from the client, hrotti delivers on PUBLISH though
			//so we've already sent the original message to any subscribers, so just create a new
			//PUBCOMP message with the correct message id and pass it to the HandleFlow function.
			case PUBREL:
				pr := New(PUBREL).(*pubrelPacket)
				pr.FixedHeader = cph
				pr.Unpack(body)
				pc := New(PUBCOMP).(*pubcompPacket)
				pc.messageID = pr.messageID
				c.HandleFlow(pc, hrotti)
			//Received a PUBCOMP for a QoS2 PUBLISH we originally sent the client. Check the messageid is
			//one we think is in use, if so delete the original PUBLISH from the outbound persistence store
			//and free the message id for reuse
			case PUBCOMP:
				pc := New(PUBCOMP).(*pubcompPacket)
				pc.FixedHeader = cph
				pc.Unpack(body)
				if c.inUse(pc.messageID) {
					hrotti.outboundPersist.Delete(c, pc.messageID)
					c.freeID(pc.messageID)
				} else {
					ERROR.Println("Received a PUBCOMP for unknown msgid", pc.messageID, "from", c.clientID)
				}
			//The client wishes to make a subscription, unpack the message and call AddSubscription with the
			//requested topics and QoS'. Create a new SUBACK message and put the granted QoS values in it
			//and send back to the client.
			case SUBSCRIBE:
				PROTOCOL.Println("Received SUBSCRIBE from", c.clientID)
				sp := New(SUBSCRIBE).(*subscribePacket)
				sp.FixedHeader = cph
				sp.Unpack(body)
				rQos := c.AddSubscription(sp.topics, sp.qoss)
				sa := New(SUBACK).(*subackPacket)
				sa.messageID = sp.messageID
				sa.grantedQoss = append(sa.grantedQoss, rQos...)
				c.outboundPriority <- sa
			//The client wants to unsubscribe from a topic.
			case UNSUBSCRIBE:
				PROTOCOL.Println("Received UNSUBSCRIBE from", c.clientID)
				up := New(UNSUBSCRIBE).(*unsubscribePacket)
				up.FixedHeader = cph
				up.Unpack(body)
				c.RemoveSubscription(up.topics[0])
				ua := New(UNSUBACK).(*unsubackPacket)
				ua.messageID = up.messageID
				c.outboundPriority <- ua
			//As part of the keepalive if the client doesn't have any messages to send us for as long as the
			//keepalive period it will send a ping request, so we send a ping response back
			case PINGREQ:
				presp := New(PINGRESP).(*pingrespPacket)
				c.outboundPriority <- presp
			}
		}
	}
}

func (c *Client) HandleFlow(msg ControlPacket, hrotti *Hrotti) {
	switch msg.Type() {
	case PUBREL:
		hrotti.outboundPersist.Replace(c, msg)
	case PUBACK, PUBCOMP:
		hrotti.inboundPersist.Delete(c, msg.MsgID())
	}
	//send to channel if open, silently drop if channel closed
	select {
	case c.outboundPriority <- msg:
	default:
	}
}

func (c *Client) Send(hrotti *Hrotti) {
	//Send is part of the Client waitgroup so call Done when the function returns.
	defer c.Done()
	for {
		//3 way blocking select
		select {
		//the stop channel has been closed so we should return
		case <-c.stop:
			return
		//the two value receive from a channel tells us whether the channel is closed
		//as reading from a closed channel always returns the empty value for the channel
		//type. ok == false means the channel is closed and the msg will be nil
		case msg, ok := <-c.outboundPriority:
			if ok {
				//If there is a durable client subscription the message id assigned will be a value
				//higher than the normal maximum message id according to the MQTT spec, this means
				//we need to get a valid MQTT message id before we send the message on.
				if msg.MsgID() >= internalIDMin {
					internalID := msg.MsgID()
					msg.SetMsgID(<-c.idChan)
					hrotti.outboundPersist.Add(c, msg)
					hrotti.outboundPersist.Delete(c, internalID)
					hrotti.internalMsgIDs.freeID(internalID)
				}
				_, err := c.conn.Write(msg.Pack())
				if err != nil {
					ERROR.Println("Error writing msg")
				}
			}
		case msg, ok := <-c.outboundMessages:
			//ok == false means we were triggered because the channel
			//is closed, and the msg will be nil
			if ok {
				if msg.MsgID() >= internalIDMin {
					internalID := msg.MsgID()
					PROTOCOL.Println("Internal Id message", internalID, "for", c.clientID)
					msg.SetMsgID(<-c.idChan)
					hrotti.outboundPersist.Add(c, msg)
					hrotti.outboundPersist.Delete(c, internalID)
					hrotti.internalMsgIDs.freeID(internalID)
				}
				_, err := c.conn.Write(msg.Pack())
				if err != nil {
					ERROR.Println("Error writing msg")
				}
			}
		}
	}
}

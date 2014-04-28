package main

import (
	"bufio"
	//"errors"
	"io"
	"net"
	"strings"
	"sync"
	"time"
)

type Client struct {
	sync.WaitGroup
	MessageIds
	clientId         string
	conn             net.Conn
	bufferedConn     *bufio.ReadWriter
	rootNode         *Node
	keepAlive        uint16
	state            State
	topicSpace       string
	outboundMessages chan *publishPacket
	outboundPriority chan ControlPacket
	stop             chan bool
	stopOnce         *sync.Once
	resetTimer       chan bool
	cleanSession     bool
	willMessage      *publishPacket
}

func NewClient(conn net.Conn, bufferedConn *bufio.ReadWriter, clientId string) *Client {
	c := &Client{}
	c.conn = conn
	c.bufferedConn = bufferedConn
	c.clientId = clientId
	c.stop = make(chan bool)
	c.resetTimer = make(chan bool, 1)
	c.outboundMessages = make(chan *publishPacket, config.maxQueueDepth)
	c.outboundPriority = make(chan ControlPacket, config.maxQueueDepth)
	c.rootNode = rootNode
	c.stopOnce = new(sync.Once)
	c.idChan = make(chan msgId, 10)
	c.index = make(map[msgId]bool)

	return c
}

func InitClient(conn net.Conn) {
	var cph FixedHeader

	//create a bufio conn from the network connection
	bufferedConn := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))
	//first byte off the wire should be the msg type
	typeByte, _ := bufferedConn.ReadByte()
	//unpack the first byte into the fixed header
	cph.unpack(typeByte)

	if cph.MessageType != CONNECT {
		//If the first packet isn't a CONNECT, it's not MQTT or not compliant, so kill the connection and we're done.
		conn.Close()
		return
	}

	//read the remaining length field from the network, this can be 1-3 bytes generally although in this case
	//it should always be 1 byte, but using the generic method.
	cph.remainingLength = decodeLength(bufferedConn)
	//a buffer to receive the rest of the connect packet
	body := make([]byte, cph.remainingLength)
	io.ReadFull(bufferedConn, body)
	//create a new empty CONNECT packet to unpack the body of the CONNECT into
	cp := New(CONNECT).(*connectPacket)
	cp.FixedHeader = cph
	cp.Unpack(body)
	//Validate the CONNECT, check fields, values etc.
	rc := cp.Validate()
	//If it didn't validate...
	if rc != CONN_ACCEPTED {
		//and it wasn't because of a protocol violation...
		if rc != CONN_PROTOCOL_VIOLATION {
			//create and send a CONNACK with the correct rc in it.
			ca := New(CONNACK).(*connackPacket)
			ca.returnCode = rc
			conn.Write(ca.Pack())
		}
		//Put up a local message indicating an errored connection attempt and close the connection
		ERROR.Println(connackReturnCodes[rc], conn.RemoteAddr())
		conn.Close()
		return
	} else {
		//Put up an INFO message with the client id and the address they're connecting from.
		INFO.Println(connackReturnCodes[rc], cp.clientIdentifier, conn.RemoteAddr())
	}
	//Lock the clients hashmap while we check if we already know this clientid.
	clients.Lock()
	c, ok := clients.list[cp.clientIdentifier]
	if ok {
		//and if we do, if the clientid is currently connected...
		if c.Connected() {
			INFO.Println("Clientid", c.clientId, "already connected, stopping first client")
			//stop the parts of it that need to stop before we can change the network connection it's using.
			c.StopForTakeover()
		} else {
			//if the clientid known but not connected, ie cleansession false
			INFO.Println("Durable client reconnecting", c.clientId)
			//disconnected client will no longer have the channels for messages
			c.outboundMessages = make(chan *publishPacket, config.maxQueueDepth)
			c.outboundPriority = make(chan ControlPacket, config.maxQueueDepth)
		}
		//this function stays running until the client disconnects as the function called by an http
		//Handler has to remain running until its work is complete. So add one to the client waitgroup.
		c.Add(1)
		//create a new sync.Once for stopping with later, set the connections and create the stop channel.
		c.stopOnce = new(sync.Once)
		c.conn = conn
		c.bufferedConn = bufferedConn
		c.stop = make(chan bool)
		//start the client.
		go c.Start(cp)
	} else {
		//This is a brand new client so create a NewClient and add to the clients map
		c = NewClient(conn, bufferedConn, cp.clientIdentifier)
		clients.list[cp.clientIdentifier] = c
		//As before this function has to remain running but to avoid races we want to make sure its finished
		//before doing anything else so add it to the waitgroup so we can wait on it later
		c.Add(1)
		go c.Start(cp)
	}
	//finished with the clients hashmap
	clients.Unlock()
	//wait on the stop channel, we never actually send values down this channel but a closed channel with
	//return the default empty value for it's type without blocking.
	<-c.stop
	//call Done() on the client waitgroup.
	c.Done()
}

func (c *Client) Connected() bool {
	return c.state.Value() == CONNECTED
}

func (c *Client) KeepAliveTimer() {
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
			ERROR.Println(c.clientId, "has timed out", c.keepAlive)
			go c.Stop(true)
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
	close(c.stop)
	c.conn.Close()
	c.Wait()
	c.conn = nil
	c.bufferedConn = nil
}

func (c *Client) Stop(sendWill bool) {
	//Its possible that error conditions with the network connection might cause both Send and Receive to
	//try and call Stop(), but we only want it to be called once, so using the sync.Once in the client we
	//run the embedded function, later calls with the same sync.Once will simply return.
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
			INFO.Println("Sending will message for", c.clientId)
			go c.rootNode.DeliverMessage(strings.Split(c.willMessage.topicName, "/"), c.willMessage)
		}
		//if this client connected with cleansession true it means it does not need its state (such as
		//subscriptions, unreceived messages etc) kept around
		if c.cleanSession {
			//so we lock the clients map, delete the clientid and *Client from the map, remove all subscriptions
			//associated with this client, from the normal tree and any plugins. Then close the persistence
			//store that it was using.
			clients.Lock()
			delete(clients.list, c.clientId)
			clients.Unlock()
			c.rootNode.DeleteSubAll(c)
			DeleteSubAllPlugins(c)
			inboundPersist.Close(c)
			outboundPersist.Close(c)
		}
	})
}

func (c *Client) Start(cp *connectPacket) {
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
	if c.cleanSession || !inboundPersist.Exists(c) || !outboundPersist.Exists(c) {
		inboundPersist.Open(c)
		outboundPersist.Open(c)
	} else {
		//we have an existing inbound and outbound persistence store for this client already, so lets
		//get any messages still in outbound and attempt to send them.
		INFO.Println("Getting unacknowledged messages from persistence")
		for _, msg := range outboundPersist.GetAll(c) {
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
	//getMsgIds, Receive and Send are part of this WaitGroup, so add 3 to the waitgroup and run the goroutines.
	c.Add(3)
	go c.genMsgIds()
	go c.Receive()
	go c.Send()
	c.state.SetValue(CONNECTED)
	//If keepalive value was set run the keepalive time and add 1 to the waitgroup.
	if c.keepAlive > 0 {
		c.Add(1)
		go c.KeepAliveTimer()
	}
}

func validateClientId(clientId string) bool {
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

func (c *Client) Receive() {
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
				ERROR.Println(err.Error(), c.clientId, c.conn.RemoteAddr())
				go c.Stop(true)
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
					go c.Stop(true)
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
				ERROR.Println("Received second CONNECT from", c.clientId)
				go c.Stop(true)
				return
			//client wishes to disconnect so Stop (don't send will) and return.
			case DISCONNECT:
				INFO.Println("Received DISCONNECT from", c.clientId)
				go c.Stop(false)
				return
			//client has sent us a PUBLISH message, unpack it persist (if QoS > 0) in the inbound store
			case PUBLISH:
				pp := New(PUBLISH).(*publishPacket)
				pp.FixedHeader = cph
				pp.Unpack(body)
				PROTOCOL.Println("Received PUBLISH from", c.clientId, pp.topicName)
				if pp.Qos > 0 {
					inboundPersist.Add(c, pp)
				}
				//if this message has the retained flag set then set as the retained message for the
				//appropriate node in the topic tree
				if pp.Retain == 1 {
					c.rootNode.SetRetained(strings.Split(pp.topicName, "/"), pp)
				}
				//go and deliver the message to any subscribers.
				go c.rootNode.DeliverMessage(strings.Split(pp.topicName, "/"), pp)
				//if the message was QoS1 or QoS2 start the acknowledgement flows.
				switch pp.Qos {
				case 1:
					pa := New(PUBACK).(*pubackPacket)
					pa.messageId = pp.messageId
					c.HandleFlow(pa)
				case 2:
					pr := New(PUBREC).(*pubrecPacket)
					pr.messageId = pp.messageId
					c.HandleFlow(pr)
				}
			//We received a PUBACK acknowledging a QoS1 PUBLISH we sent to the client
			case PUBACK:
				pa := New(PUBACK).(*pubackPacket)
				pa.FixedHeader = cph
				pa.Unpack(body)
				//Check that we also think this message id is in use, if it is remove the original
				//PUBLISH from the outbound persistence store and set the message id as free for reuse
				if c.inUse(pa.messageId) {
					outboundPersist.Delete(c, pa.messageId)
					c.freeId(pa.messageId)
				} else {
					ERROR.Println("Received a PUBACK for unknown msgid", pa.messageId, "from", c.clientId)
				}
			//We received a PUBREC for a QoS2 PUBLISH we sent to the client.
			case PUBREC:
				pr := New(PUBREC).(*pubrecPacket)
				pr.FixedHeader = cph
				pr.Unpack(body)
				//Check that we also think this message id is in use, if it is run the next stage of the
				//message flows for QoS2 messages.
				if c.inUse(pr.messageId) {
					prel := New(PUBREL).(*pubrelPacket)
					prel.messageId = pr.messageId
					c.HandleFlow(prel)
				} else {
					ERROR.Println("Received a PUBREC for unknown msgid", pr.messageId, "from", c.clientId)
				}
			//We received a PUBREL for a QoS2 PUBLISH from the client, hrotti delivers on PUBLISH though
			//so we've already sent the original message to any subscribers, so just create a new
			//PUBCOMP message with the correct message id and pass it to the HandleFlow function.
			case PUBREL:
				pr := New(PUBREL).(*pubrelPacket)
				pr.FixedHeader = cph
				pr.Unpack(body)
				pc := New(PUBCOMP).(*pubcompPacket)
				pc.messageId = pr.messageId
				c.HandleFlow(pc)
			//Received a PUBCOMP for a QoS2 PUBLISH we originally sent the client. Check the messageid is
			//one we think is in use, if so delete the original PUBLISH from the outbound persistence store
			//and free the message id for reuse
			case PUBCOMP:
				pc := New(PUBCOMP).(*pubcompPacket)
				pc.FixedHeader = cph
				pc.Unpack(body)
				if c.inUse(pc.messageId) {
					outboundPersist.Delete(c, pc.messageId)
					c.freeId(pc.messageId)
				} else {
					ERROR.Println("Received a PUBCOMP for unknown msgid", pc.messageId, "from", c.clientId)
				}
			//The client wishes to make a subscription, unpack the message and call AddSubscription with the
			//requested topics and QoS'. Create a new SUBACK message and put the granted QoS values in it
			//and send back to the client.
			case SUBSCRIBE:
				PROTOCOL.Println("Received SUBSCRIBE from", c.clientId)
				sp := New(SUBSCRIBE).(*subscribePacket)
				sp.FixedHeader = cph
				sp.Unpack(body)
				rQos := c.AddSubscription(sp.topics, sp.qoss)
				sa := New(SUBACK).(*subackPacket)
				sa.messageId = sp.messageId
				sa.grantedQoss = append(sa.grantedQoss, rQos...)
				c.outboundPriority <- sa
			//The client wants to unsubscribe from a topic.
			case UNSUBSCRIBE:
				PROTOCOL.Println("Received UNSUBSCRIBE from", c.clientId)
				up := New(UNSUBSCRIBE).(*unsubscribePacket)
				up.FixedHeader = cph
				up.Unpack(body)
				c.RemoveSubscription(up.topics[0])
				ua := New(UNSUBACK).(*unsubackPacket)
				ua.messageId = up.messageId
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

func (c *Client) HandleFlow(msg ControlPacket) {
	switch msg.Type() {
	case PUBREL:
		outboundPersist.Replace(c, msg)
	case PUBACK, PUBCOMP:
		inboundPersist.Delete(c, msg.MsgId())
	}
	//send to channel if open, silently drop if channel closed
	select {
	case c.outboundPriority <- msg:
	default:
	}
}

func (c *Client) Send() {
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
				if msg.MsgId() >= internalIdMin {
					internalId := msg.MsgId()
					msg.SetMsgId(<-c.idChan)
					outboundPersist.Add(c, msg)
					outboundPersist.Delete(c, internalId)
					freeInternalId(internalId)
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
				if msg.MsgId() >= internalIdMin {
					internalId := msg.MsgId()
					PROTOCOL.Println("Internal Id message", internalId, "for", c.clientId)
					msg.SetMsgId(<-c.idChan)
					outboundPersist.Add(c, msg)
					outboundPersist.Delete(c, internalId)
					freeInternalId(internalId)
				}
				_, err := c.conn.Write(msg.Pack())
				if err != nil {
					ERROR.Println("Error writing msg")
				}
			}
		}
	}
}

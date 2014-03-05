package main

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// the Client struct represent the information required to known about the client connection
type Client struct {
	sync.RWMutex
	MessageIds
	clientId         string
	conn             net.Conn
	rootNode         *Node
	keepAlive        uint
	connected        bool
	topicSpace       string
	outboundMessages chan ControlPacket
	sendStop         chan bool
	recvStop         chan bool
	lastSeen         time.Time
	cleanSession     bool
}

func NewClient(conn net.Conn, clientId string) *Client {
	//if !validateClientId(clientId) {
	//	return nil, errors.New("Invalid Client Id")
	//}
	c := &Client{}
	c.conn = conn
	c.clientId = clientId
	// c.subscriptions = list.New()
	c.sendStop = make(chan bool)
	c.recvStop = make(chan bool)
	c.outboundMessages = make(chan ControlPacket)
	c.rootNode = rootNode

	return c
}

func (c *Client) Remove() {
	if c.cleanSession {
		delete(clients, c.clientId)
	}
}

func (c *Client) Stop() {
	c.sendStop <- true
	c.recvStop <- true
	c.conn.Close()
}

func (c *Client) Start() {
	go c.Receive()
	go c.Send()
}

func (c *Client) resetLastSeenTime() {
	c.lastSeen = time.Now()
}

func validateClientId(clientId string) bool {
	return true
}

func (c *Client) SetRootNode(node *Node) {
	c.rootNode = node
}

func (c *Client) AddSubscription(topic string, qos uint) {
	// for s := c.subscriptions.Front(); s != nil; s = s.Next() {
	// 	if s.Value.(*Subscription).match(subscription.topicFilter) {
	// 		s.Value = subscription
	// 		return
	// 	}
	// }
	// c.subscriptions.PushBack(subscription)
	complete := make(chan bool)
	defer close(complete)
	c.rootNode.AddSub(c, strings.Split(topic, "/"), qos, complete)
	<-complete
	return
}

func (c *Client) RemoveSubscription(topic string) (bool, error) {
	// for s := c.subscriptions.Front(); s != nil; s = s.Next() {
	// 	if s.Value.(*Subscription).match(subscription.topicFilter) {
	// 		c.subscriptions.Remove(s)
	// 		return true, nil
	// 	}
	// }
	complete := make(chan bool)
	defer close(complete)
	c.rootNode.DeleteSub(c, strings.Split(topic, "/"), complete)
	<-complete
	return true, errors.New("Topic not found")
}

func (c *Client) Receive() {
	for {
		var cph FixedHeader
		//var cp ControlPacket

		header := make([]byte, 4)
		_, err := c.conn.Read(header)
		if err != nil {
			break
		}
		cph.unpack(header)

		body := make([]byte, cph.remainingLength-(4-uint32(cph.unpack(header))))
		if cph.remainingLength > 2 {
			_, err = c.conn.Read(body)
			if err != nil {
				break
			}
		}

		switch cph.MessageType {
		case DISCONNECT:
			fmt.Println("Received DISCONNECT from", c.clientId)
			dp := New(DISCONNECT).(*disconnectPacket)
			dp.Unpack(append(header, body...))
			c.Stop()
			fmt.Println("Stop has returned")
			if c.cleanSession {
				c.Remove()
			}
			break
		case PUBLISH:
			fmt.Println("Received PUBLISH from", c.clientId)
			pp := New(PUBLISH).(*publishPacket)
			pp.Unpack(append(header, body...))
			fmt.Println(pp.String())
			c.rootNode.DeliverMessage(strings.Split(pp.topicName, "/"), pp)
			switch pp.Qos {
			case 1:
				pa := New(PUBACK).(*pubackPacket)
				pa.messageId = pp.messageId
				c.outboundMessages <- pa
			case 2:
				pr := New(PUBREC).(*pubrecPacket)
				pr.messageId = pp.messageId
				c.outboundMessages <- pr
			}
		case PUBACK:
			pa := New(PUBACK).(*pubackPacket)
			pa.Unpack(append(header, body...))
		case PUBREC:
			pr := New(PUBREC).(*pubrecPacket)
			pr.Unpack(append(header, body...))
		case PUBREL:
			pr := New(PUBREL).(*pubrelPacket)
			pr.Unpack(append(header, body...))
			pc := New(PUBCOMP).(*pubcompPacket)
			pc.messageId = pr.messageId
			c.outboundMessages <- pc
		case PUBCOMP:
			pc := New(PUBCOMP).(*pubcompPacket)
			pc.Unpack(append(header, body...))
		case SUBSCRIBE:
			fmt.Println("Received SUBSCRIBE from", c.clientId)
			sp := New(SUBSCRIBE).(*subscribePacket)
			sp.Unpack(append(header, body...))
			c.AddSubscription(sp.topics[0], sp.qoss[0])
			sa := New(SUBACK).(*subackPacket)
			sa.messageId = sp.messageId
			sa.grantedQoss = append(sa.grantedQoss, byte(sp.qoss[0]))
			c.outboundMessages <- sa
		case UNSUBSCRIBE:
			fmt.Println("Received UNSUBSCRIBE from", c.clientId)
			up := New(UNSUBSCRIBE).(*unsubscribePacket)
			up.Unpack(append(header, body...))
			c.RemoveSubscription(up.topics[0])
			ua := New(UNSUBACK).(*unsubackPacket)
			ua.messageId = up.messageId
			c.outboundMessages <- ua
		case PINGREQ:
			preq := New(PINGREQ).(*pingreqPacket)
			preq.Unpack(append(header, body...))
			presp := New(PINGRESP).(*pingrespPacket)
			c.outboundMessages <- presp
		}
	}
	select {
	case <-c.recvStop:
		fmt.Println("Requested to stop")
		return
	default:
		fmt.Println("Error on socket read")
		return
	}
}

// receive a Message object on obound, and then
// actually send outgoing message to the wire
func (c *Client) Send() {
	for {
		select {
		case msg := <-c.outboundMessages:
			fmt.Println("Outbound message on channel", msg.String())
			c.conn.Write(msg.Pack())
		case <-c.sendStop:
			fmt.Println("Requested to stop")
			return
		}
	}
	/*for {
		c.trace_v(NET, "outgoing waiting for an outbound message")
		select {
		case out := <-c.obound:
			msg := out.m
			msgtype := msg.msgType()
			c.trace_v(NET, "obound got msg to write, type: %d", msgtype)
			if msg.QoS() != QOS_ZERO && msg.MsgId() == 0 {
				msg.setMsgId(c.options.mids.getId())
			}
			if out.r != nil {
				c.receipts.put(msg.MsgId(), out.r)
			}
			msg.setTime()
			persist_obound(c.persist, msg)
			_, err := c.conn.Write(msg.Bytes())
			if err != nil {
				c.trace_e(NET, "outgoing stopped with error")
				c.errors <- err
				return
			}

			if (msg.QoS() == QOS_ZERO) &&
				(msgtype == PUBLISH || msgtype == SUBSCRIBE || msgtype == UNSUBSCRIBE) {
				c.receipts.get(msg.MsgId()) <- Receipt{}
				c.receipts.end(msg.MsgId())
			}
			c.lastContact.update()
			c.trace_v(NET, "obound wrote msg, id: %v", msg.MsgId())
		case msg := <-c.oboundP:
			msgtype := msg.msgType()
			c.trace_v(NET, "obound priority msg to write, type %d", msgtype)
			_, err := c.conn.Write(msg.Bytes())
			if err != nil {
				c.trace_e(NET, "outgoing stopped with error")
				c.errors <- err
				return
			}
			c.lastContact.update()
			if msgtype == DISCONNECT {
				c.trace_v(NET, "outbound wrote disconnect, now closing connection")
				c.conn.Close()
				return
			}
		}
	}*/
}

// receive Message objects on ibound
// store messages if necessary
// send replies on obound
// delete messages from store if necessary
/*func alllogic(c *MqttClient) {

	c.trace_v(NET, "logic started")

	for {
		c.trace_v(NET, "logic waiting for msg on ibound")

		select {
		case msg := <-c.ibound:
			c.trace_v(NET, "logic got msg on ibound, type %v", msg.msgType())
			persist_ibound(c.persist, msg)
			switch msg.msgType() {
			case PINGRESP:
				c.trace_v(NET, "received pingresp")
				c.pingOutstanding = false
			case CONNACK:
				c.trace_v(NET, "received connack")
				c.begin <- msg.connRC()
				close(c.begin)
			case SUBACK:
				c.trace_v(NET, "received suback, id: %v", msg.MsgId())
				c.receipts.get(msg.MsgId()) <- Receipt{}
				c.receipts.end(msg.MsgId())
				go c.options.mids.freeId(msg.MsgId())
			case UNSUBACK:
				c.trace_v(NET, "received unsuback, id: %v", msg.MsgId())
				c.receipts.get(msg.MsgId()) <- Receipt{}
				c.receipts.end(msg.MsgId())
				go c.options.mids.freeId(msg.MsgId())
			case PUBLISH:
				c.trace_v(NET, "received publish, msgId: %v", msg.MsgId())
				c.trace_v(NET, "putting msg on onPubChan")
				switch msg.QoS() {
				case QOS_TWO:
					c.options.pubChanTwo <- msg
					c.trace_v(NET, "done putting msg on pubChanTwo")
					pubrecMsg := newPubRecMsg()
					pubrecMsg.setMsgId(msg.MsgId())
					c.trace_v(NET, "putting pubrec msg on obound")
					c.obound <- sendable{pubrecMsg, nil}
					c.trace_v(NET, "done putting pubrec msg on obound")
				case QOS_ONE:
					c.options.pubChanOne <- msg
					c.trace_v(NET, "done putting msg on pubChanOne")
					pubackMsg := newPubAckMsg()
					pubackMsg.setMsgId(msg.MsgId())
					c.trace_v(NET, "putting puback msg on obound")
					c.obound <- sendable{pubackMsg, nil}
					c.trace_v(NET, "done putting puback msg on obound")
				case QOS_ZERO:
					c.options.pubChanZero <- msg
					c.trace_v(NET, "done putting msg on pubChanZero")
				}
			case PUBACK:
				c.trace_v(NET, "received puback, id: %v", msg.MsgId())
				c.receipts.get(msg.MsgId()) <- Receipt{}
				c.receipts.end(msg.MsgId())
				go c.options.mids.freeId(msg.MsgId())
			case PUBREC:
				c.trace_v(NET, "received pubrec, id: %v", msg.MsgId())
				id := msg.MsgId()
				pubrelMsg := newPubRelMsg()
				pubrelMsg.setMsgId(id)
				select {
				case c.obound <- sendable{pubrelMsg, nil}:
				case <-time.After(time.Second):
				}
			case PUBREL:
				c.trace_v(NET, "received pubrel, id: %v", msg.MsgId())
				pubcompMsg := newPubCompMsg()
				pubcompMsg.setMsgId(msg.MsgId())
				select {
				case c.obound <- sendable{pubcompMsg, nil}:
				case <-time.After(time.Second):
				}
			case PUBCOMP:
				c.trace_v(NET, "received pubcomp, id: %v", msg.MsgId())
				c.receipts.get(msg.MsgId()) <- Receipt{}
				c.receipts.end(msg.MsgId())
				go c.options.mids.freeId(msg.MsgId())
			}
		case <-c.stopNet:
			c.trace_w(NET, "logic stopped")
			return
		case err := <-c.errors:
			c.trace_e(NET, "logic got error")
			// clean up go routines
			c.stopPing <- true
			// incoming most likely stopped if outgoing stopped,
			// but let it know to stop anyways.
			c.stopNet <- true
			c.options.stopRouter <- true

			close(c.stopPing)
			close(c.stopNet)
			c.conn.Close()

			// Call onConnectionLost or default error handler
			go c.options.onconnlost(err)
			return
		}
	}
}*/

package main

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

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
}

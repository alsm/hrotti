package main

import (
	"errors"
	"fmt"
	"io"
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
	connReader       io.Reader
	rootNode         *Node
	keepAlive        uint
	connected        bool
	topicSpace       string
	outboundMessages chan ControlPacket
	stop             chan bool
	lastSeen         time.Time
	cleanSession     bool
}

func NewClient(conn net.Conn, connReader io.Reader, clientId string) *Client {
	//if !validateClientId(clientId) {
	//	return nil, errors.New("Invalid Client Id")
	//}
	c := &Client{}
	c.conn = conn
	c.connReader = connReader
	c.clientId = clientId
	// c.subscriptions = list.New()
	c.stop = make(chan bool)
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
	close(c.stop)
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
		var err error
		var body []byte
		var length int
		typeByte := make([]byte, 1)
		//var cp ControlPacket

		length, err = io.ReadFull(c.connReader, typeByte)
		if err != nil {
			break
		}
		cph.unpack(typeByte[0])
		fmt.Println("Message type", cph.MessageType)
		cph.remainingLength = decodeLength(c.connReader)

		if cph.remainingLength > 0 {
			body = make([]byte, cph.remainingLength)
			length, err = io.ReadFull(c.connReader, body)
			if err != nil {
				break
			}
			fmt.Println("Read", length, "bytes")
		}

		switch cph.MessageType {
		case DISCONNECT:
			fmt.Println("Received DISCONNECT from", c.clientId)
			dp := New(DISCONNECT).(*disconnectPacket)
			dp.FixedHeader = cph
			dp.Unpack(body)
			c.Stop()
			fmt.Println("Stop has returned")
			if c.cleanSession {
				c.Remove()
			}
			break
		case PUBLISH:
			//fmt.Println("Received PUBLISH from", c.clientId)
			pp := New(PUBLISH).(*publishPacket)
			pp.FixedHeader = cph
			pp.Unpack(body)
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
			pa.FixedHeader = cph
			pa.Unpack(body)
		case PUBREC:
			pr := New(PUBREC).(*pubrecPacket)
			pr.FixedHeader = cph
			pr.Unpack(body)
		case PUBREL:
			pr := New(PUBREL).(*pubrelPacket)
			pr.FixedHeader = cph
			pr.Unpack(body)
			pc := New(PUBCOMP).(*pubcompPacket)
			pc.messageId = pr.messageId
			c.outboundMessages <- pc
		case PUBCOMP:
			pc := New(PUBCOMP).(*pubcompPacket)
			pc.FixedHeader = cph
			pc.Unpack(body)
		case SUBSCRIBE:
			fmt.Println("Received SUBSCRIBE from", c.clientId)
			sp := New(SUBSCRIBE).(*subscribePacket)
			sp.FixedHeader = cph
			sp.Unpack(body)
			c.AddSubscription(sp.topics[0], sp.qoss[0])
			sa := New(SUBACK).(*subackPacket)
			sa.messageId = sp.messageId
			sa.grantedQoss = append(sa.grantedQoss, byte(sp.qoss[0]))
			c.outboundMessages <- sa
		case UNSUBSCRIBE:
			fmt.Println("Received UNSUBSCRIBE from", c.clientId)
			up := New(UNSUBSCRIBE).(*unsubscribePacket)
			up.FixedHeader = cph
			up.Unpack(body)
			c.RemoveSubscription(up.topics[0])
			ua := New(UNSUBACK).(*unsubackPacket)
			ua.messageId = up.messageId
			c.outboundMessages <- ua
		case PINGREQ:
			presp := New(PINGRESP).(*pingrespPacket)
			c.outboundMessages <- presp
		}
	}
	select {
	case <-c.stop:
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
		case <-c.stop:
			fmt.Println("Requested to stop")
			return
		}
	}
}

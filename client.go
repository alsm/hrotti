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
	sync.RWMutex
	MessageIds
	clientId         string
	conn             net.Conn
	bufferedConn     *bufio.ReadWriter
	rootNode         *Node
	keepAlive        uint16
	connected        bool
	topicSpace       string
	outboundMessages chan *publishPacket
	outboundPriority chan ControlPacket
	stop             chan bool
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

	return c
}

func InitClient(conn net.Conn) {
	var cph FixedHeader
	var takeover bool
	var ok bool
	var c *Client
	var typeByte byte

	bufferedConn := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

	typeByte, _ = bufferedConn.ReadByte()
	cph.unpack(typeByte)

	if cph.MessageType != CONNECT {
		conn.Close()
		return
	}

	cph.remainingLength = decodeLength(bufferedConn)

	//a buffer to receive the rest of the connect packet
	body := make([]byte, cph.remainingLength)
	io.ReadFull(bufferedConn, body)

	cp := New(CONNECT).(*connectPacket)
	cp.FixedHeader = cph
	cp.Unpack(body)

	rc := cp.Validate()
	if rc != CONN_ACCEPTED {
		if rc != CONN_PROTOCOL_VIOLATION {
			ca := New(CONNACK).(*connackPacket)
			ca.returnCode = rc
			bufferedConn.Write(ca.Pack())
			bufferedConn.Flush()
		}
		ERROR.Println(connackReturnCodes[rc], conn.RemoteAddr())
		conn.Close()
		return
	} else {
		INFO.Println(connackReturnCodes[rc], cp.clientIdentifier, conn.RemoteAddr())
	}

	clients.RLock()
	if c, ok = clients.list[cp.clientIdentifier]; ok {
		takeover = true
		c.Lock()
		if c.connected {
			INFO.Println("Clientid", c.clientId, "already connected, stopping first client")
			close(c.stop)
			c.conn.Close()
			time.Sleep(100 * time.Millisecond)
		} else {
			INFO.Println("Durable client reconnecting", c.clientId)
		}
		c.conn = conn
		c.bufferedConn = bufferedConn
		c.stop = make(chan bool)
		c.outboundMessages = make(chan *publishPacket, config.maxQueueDepth)
		c.outboundPriority = make(chan ControlPacket, config.maxQueueDepth)
		c.Unlock()
		go c.Start(cp)
	}
	clients.RUnlock()

	if !takeover {
		clients.Lock()
		c = NewClient(conn, bufferedConn, cp.clientIdentifier)
		clients.list[cp.clientIdentifier] = c
		clients.Unlock()
		go c.Start(cp)
	}
	<-c.stop
}

func (c *Client) Remove() {
	if c.cleanSession {
		clients.Lock()
		delete(clients.list, c.clientId)
		clients.Unlock()
		c.rootNode.DeleteSubAll(c)
		DeleteSubAllPlugins(c)
		inboundPersist.Close(c)
		outboundPersist.Close(c)
	}
}

func (c *Client) KeepAliveTimer() {
	for {
		t := time.NewTimer(time.Duration(float64(c.keepAlive)*1.5) * time.Second)
		select {
		case _ = <-c.resetTimer:
		case <-t.C:
			ERROR.Println(c.clientId, "has timed out", c.keepAlive)
			c.Stop(true)
			c.Remove()
		case <-c.stop:
			return
		}
		t.Stop()
	}
}

func (c *Client) Stop(sendWill bool) {
	c.connected = false
	close(c.stop)
	c.conn.Close()
	close(c.outboundMessages)
	close(c.outboundPriority)
	if sendWill && c.willMessage != nil {
		INFO.Println("Sending will message for", c.clientId)
		go c.rootNode.DeliverMessage(strings.Split(c.willMessage.topicName, "/"), c.willMessage)
	}
}

func (c *Client) Start(cp *connectPacket) {
	if cp.cleanSession == 1 {
		c.cleanSession = true
	}
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

	if c.cleanSession || !inboundPersist.Exists(c) || !outboundPersist.Exists(c) {
		inboundPersist.Open(c)
		outboundPersist.Open(c)
	} else {
		INFO.Println("Getting unacknowledged messages from persistence")
		for _, msg := range outboundPersist.GetAll(c) {
			switch msg.Type() {
			case PUBLISH:
				c.outboundMessages <- msg.(*publishPacket)
			default:
				c.outboundPriority <- msg
			}
		}
	}

	c.genMsgIds()
	go c.Receive()
	ca := New(CONNACK).(*connackPacket)
	ca.returnCode = CONN_ACCEPTED
	c.bufferedConn.Write(ca.Pack())
	c.bufferedConn.Flush()
	go c.Send()
	c.connected = true
	if c.keepAlive > 0 {
		go c.KeepAliveTimer()
	}
}

func validateClientId(clientId string) bool {
	return true
}

func (c *Client) ResetTimer() {
	if c.keepAlive > 0 {
		c.resetTimer <- true
	}
}

func (c *Client) SetRootNode(node *Node) {
	c.rootNode = node
}

func (c *Client) AddSubscription(topics []string, qoss []byte) []byte {
	var subWorkers sync.WaitGroup
	rQos := make([]byte, len(qoss))

	for i, topic := range topics {
		subWorkers.Add(1)
		go func(topic string, qos byte, entry *byte, wg *sync.WaitGroup) {
			defer wg.Done()
			complete := make(chan byte, 1)
			defer close(complete)
			topicArr := strings.Split(topic, "/")
			if plugin, ok := pluginNodes[topicArr[0]]; ok {
				go plugin.AddSub(c, topicArr, qos, complete)
			} else {
				c.rootNode.AddSub(c, topicArr, qos, complete)
			}
			*entry = <-complete
			if strings.ContainsAny(topic, "+") {
				c.rootNode.FindRetainedForPlus(c, topicArr)
			}
			INFO.Println("Subscription made for", c.clientId, topic)
		}(topic, qoss[i], &rQos[i], &subWorkers)
	}
	subWorkers.Wait()

	return rQos
}

func (c *Client) RemoveSubscription(topic string) (bool, error) {
	complete := make(chan bool)
	defer close(complete)
	topicArr := strings.Split(topic, "/")
	if plugin, ok := pluginNodes[topicArr[0]]; ok {
		go plugin.DeleteSub(c, topicArr, complete)
	} else {
		c.rootNode.DeleteSub(c, topicArr, complete)
	}
	<-complete
	return true, nil
}

func (c *Client) Receive() {
	for {
		var cph FixedHeader
		var err error
		var body []byte
		var typeByte byte

		typeByte, err = c.bufferedConn.ReadByte()
		if err != nil {
			ERROR.Println(err.Error(), c.clientId, c.conn.RemoteAddr())
			break
		}
		c.ResetTimer()
		cph.unpack(typeByte)
		cph.remainingLength = decodeLength(c.bufferedConn)

		if cph.remainingLength > 0 {
			body = make([]byte, cph.remainingLength)
			_, err = io.ReadFull(c.bufferedConn, body)
			if err != nil {
				break
			}
		}
		c.ResetTimer()

		switch cph.MessageType {
		case CONNECT:
			ERROR.Println("Received second CONNECT from", c.clientId)
			c.Stop(true)
			c.Remove()
			continue
		case DISCONNECT:
			INFO.Println("Received DISCONNECT from", c.clientId)
			dp := New(DISCONNECT).(*disconnectPacket)
			dp.FixedHeader = cph
			dp.Unpack(body)
			c.Stop(false)
			c.Remove()
			continue
		case PUBLISH:
			pp := New(PUBLISH).(*publishPacket)
			pp.FixedHeader = cph
			pp.Unpack(body)
			PROTOCOL.Println("Received PUBLISH from", c.clientId, pp.topicName)
			inboundPersist.Add(c, pp.messageId, pp)
			if pp.Retain == 1 {
				c.rootNode.SetRetained(strings.Split(pp.topicName, "/"), pp)
			}
			go c.rootNode.DeliverMessage(strings.Split(pp.topicName, "/"), pp)
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
		case PUBACK:
			pa := New(PUBACK).(*pubackPacket)
			pa.FixedHeader = cph
			pa.Unpack(body)
			if c.inUse(pa.messageId) {
				outboundPersist.Delete(c, pa.messageId)
				c.freeId(pa.messageId)
			} else {
				ERROR.Println("Received a PUBACK for unknown msgid", pa.messageId, "from", c.clientId)
			}
		case PUBREC:
			pr := New(PUBREC).(*pubrecPacket)
			pr.FixedHeader = cph
			pr.Unpack(body)
			if c.inUse(pr.messageId) {
				prel := New(PUBREL).(*pubrelPacket)
				prel.messageId = pr.messageId
				c.HandleFlow(prel)
			} else {
				ERROR.Println("Received a PUBREC for unknown msgid", pr.messageId, "from", c.clientId)
			}
		case PUBREL:
			pr := New(PUBREL).(*pubrelPacket)
			pr.FixedHeader = cph
			pr.Unpack(body)
			pc := New(PUBCOMP).(*pubcompPacket)
			pc.messageId = pr.messageId
			c.HandleFlow(pc)
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
		case UNSUBSCRIBE:
			PROTOCOL.Println("Received UNSUBSCRIBE from", c.clientId)
			up := New(UNSUBSCRIBE).(*unsubscribePacket)
			up.FixedHeader = cph
			up.Unpack(body)
			c.RemoveSubscription(up.topics[0])
			ua := New(UNSUBACK).(*unsubackPacket)
			ua.messageId = up.messageId
			c.outboundPriority <- ua
		case PINGREQ:
			presp := New(PINGRESP).(*pingrespPacket)
			c.outboundPriority <- presp
		}
	}
	select {
	case <-c.stop:
		return
	default:
		ERROR.Println("Receive error on socket read", c.clientId, c.conn.RemoteAddr())
		c.Stop(true)
		c.Remove()
		return
	}
}

func (c *Client) HandleFlow(msg ControlPacket) {
	switch msg.Type() {
	case PUBREL:
		outboundPersist.Replace(c, msg.GetMsgId(), msg)
	case PUBACK, PUBCOMP:
		inboundPersist.Delete(c, msg.GetMsgId())
	}
	//send to channel if open, silently drop if channel closed
	select {
	case c.outboundPriority <- msg:
	default:
	}
}

func (c *Client) Send() {
	for {
		select {
		case <-c.stop:
			//closing c.stop signals we should return
			return
		case msg, ok := <-c.outboundPriority:
			//ok == false means we were triggered because the channel
			//is closed, and the msg will be nil
			if ok {
				_, err := c.conn.Write(msg.Pack())
				if err != nil {
					ERROR.Println("Error writing msg")
				}
			}
		case msg, ok := <-c.outboundMessages:
			//ok == false means we were triggered because the channel
			//is closed, and the msg will be nil
			if ok {
				_, err := c.conn.Write(msg.Pack())
				if err != nil {
					ERROR.Println("Error writing msg")
				}
			}
		}
	}
}

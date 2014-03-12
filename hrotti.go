package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"runtime"
	"sync"
)

var clients struct {
	sync.Mutex
	list map[string]*Client
}

func init() {
	clients.list = make(map[string]*Client)
	runtime.GOMAXPROCS(4)
}

func main() {
	ln, err := net.Listen("tcp", ":1883")
	if err != nil {
		fmt.Println(err.Error())
	}
	for {
		var cph FixedHeader
		var takeover bool
		var c *Client
		var typeByte byte

		conn, err := ln.Accept()
		bufferedConn := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))
		fmt.Println("New incoming connection", conn.LocalAddr())
		if err != nil {
			fmt.Println(err.Error())
			continue
		}

		typeByte, _ = bufferedConn.ReadByte()
		cph.unpack(typeByte)

		if cph.MessageType != CONNECT {
			conn.Close()
			continue
		}

		cph.remainingLength = decodeLength(bufferedConn)

		//a buffer to receive the rest of the connect packet
		body := make([]byte, cph.remainingLength)
		io.ReadFull(bufferedConn, body)

		cp := New(CONNECT).(*connectPacket)
		cp.FixedHeader = cph
		cp.Unpack(body)
		if rc := cp.Validate(); rc != CONN_ACCEPTED {
			if rc != CONN_PROTOCOL_VIOLATION {
				ca := New(CONNACK).(*connackPacket)
				ca.returnCode = rc
				conn.Write(ca.Pack())
			}
			conn.Close()
			continue
		}

		clients.Lock()
		if c, ok := clients.list[cp.clientIdentifier]; ok {
			takeover = true
			go func() {
				fmt.Println("Takeover!")
				c.Lock()
				if c.connected {
					fmt.Println("Client is connected, stopping", c.clientId)
					c.Stop(false)
				} else {
					fmt.Println("Durable client reconnecting", c.clientId)
				}
				c.conn = conn
				c.bufferedConn = bufferedConn
				c.Unlock()
				go c.Start(cp)
			}()
		}

		if !takeover {
			c = NewClient(conn, bufferedConn, cp.clientIdentifier)
			clients.list[cp.clientIdentifier] = c
			go c.Start(cp)
		}
		clients.Unlock()
	}
}

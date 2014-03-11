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
		//read only the first  byte to check for connect
		//io.ReadFull(bufferedConn, typeByte)
		typeByte, _ = bufferedConn.ReadByte()
		cph.unpack(typeByte)
		//_, err = conn.Read(typeByte)
		//fmt.Println("read bytes:", length)

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
		//fmt.Println(cp.String())
		if cp.Validate() {
			//fmt.Println("CONNECT packet validated")
		}

		ca := New(CONNACK).(*connackPacket)
		ca.returnCode = CONN_ACCEPTED
		conn.Write(ca.Pack())

		clients.Lock()
		if c, ok := clients.list[cp.clientIdentifier]; ok {
			takeover = true
			go func() {
				c.Lock()
				c.Stop()
				c.conn = conn
				c.bufferedConn = bufferedConn
				c.Unlock()
				c.Start()
			}()
		}

		if !takeover {
			c = NewClient(conn, bufferedConn, cp.clientIdentifier)
			if cp.cleanSession > 0 {
				c.cleanSession = true
			}
			clients.list[cp.clientIdentifier] = c
			go c.Start()
		}
		clients.Unlock()
		//go handleConnection(conn)
	}
}

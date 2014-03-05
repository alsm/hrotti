package main

import (
	"fmt"
	"io"
	"net"
)

var clients map[string]*Client

func init() {
	clients = make(map[string]*Client)
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
		typeByte := make([]byte, 1)

		conn, err := ln.Accept()
		connReader := io.Reader(conn)
		fmt.Println("New incoming connection", conn.LocalAddr())
		if err != nil {
			fmt.Println(err.Error())
			continue
		}
		//read only the first  byte to check for connect
		io.ReadFull(connReader, typeByte)
		cph.unpack(typeByte[0])
		//_, err = conn.Read(typeByte)
		//fmt.Println("read bytes:", length)

		if cph.MessageType != CONNECT {
			conn.Close()
			continue
		}

		cph.remainingLength = decodeLength(connReader)

		//a buffer to receive the rest of the connect packet
		body := make([]byte, cph.remainingLength)
		io.ReadFull(connReader, body)

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

		if c, ok := clients[cp.clientIdentifier]; ok {
			takeover = true
			go func() {
				c.Lock()
				c.Stop()
				c.conn = conn
				c.connReader = connReader
				c.Unlock()
			}()
		}

		if !takeover {
			c = NewClient(conn, connReader, cp.clientIdentifier)
			if cp.cleanSession > 0 {
				c.cleanSession = true
			}
			clients[cp.clientIdentifier] = c
		}

		go c.Start()
		//go handleConnection(conn)
	}
}

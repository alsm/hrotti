package main

import (
	"fmt"
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

		conn, err := ln.Accept()
		fmt.Println("New incoming connection", conn.LocalAddr())
		if err != nil {
			fmt.Println(err.Error())
			continue
		}
		//read only the first 4 bytes to check for connect and the size of the connect packet
		header := make([]byte, 4)
		length, err := conn.Read(header)
		fmt.Println("read bytes:", length)
		cph.unpack(header)

		if cph.MessageType != CONNECT {
			conn.Close()
			continue
		}

		//a buffer to receive the rest of the connect packet, header may have been less than 4 bytes
		body := make([]byte, cph.remainingLength-(4-uint32(cph.unpack(header))))
		length, err = conn.Read(body)
		fmt.Println("read bytes:", length)

		cp := New(CONNECT).(*connectPacket)
		cp.Unpack(append(header, body...))
		fmt.Println(cp.String())
		if cp.Validate() {
			fmt.Println("CONNECT packet validated")
		}

		ca := New(CONNACK).(*connackPacket)
		ca.returnCode = CONN_ACCEPTED
		conn.Write(ca.Pack())
		fmt.Println("Sent CONNACK")

		if c, ok := clients[cp.clientIdentifier]; ok {
			takeover = true
			go func() {
				c.Lock()
				c.Stop()
				c.conn = conn
				c.Unlock()
			}()
		}

		// for cl := clients.List.Front(); cl != nil; cl = cl.Next() {
		// 	c = cl.Value.(*Client)
		// 	fmt.Println(cp.clientIdentifier, c.clientId)
		// 	if cp.clientIdentifier == c.clientId {
		// 		takeover = true
		// 		go func() {
		// 			c.Lock()
		// 			c.Stop()
		// 			c.conn = conn
		// 			c.Unlock()
		// 		}()
		// 		break
		// 	}
		// }

		if !takeover {
			c = NewClient(conn, cp.clientIdentifier)
			if cp.cleanSession > 0 {
				c.cleanSession = true
			}
			clients[cp.clientIdentifier] = c
		}

		fmt.Println(len(clients))

		go c.Start()
		//go handleConnection(conn)
	}
}

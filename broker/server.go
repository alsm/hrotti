package hrotti

import (
	"bufio"
	"errors"
	"io"
	"net"
	"net/http"
	"sync"

	"code.google.com/p/go.net/websocket"
)

type Hrotti struct {
	inboundPersist  Persistence
	outboundPersist Persistence
	listeners       map[string]ListenerConfig
	maxQueueDepth   int
	clients         Clients
	internalMsgIds  *internalIds
}

func NewHrotti(maxQueueDepth int) *Hrotti {
	h := &Hrotti{
		inboundPersist:  NewMemoryPersistence(),
		outboundPersist: NewMemoryPersistence(),
		listeners:       make(map[string]ListenerConfig),
		maxQueueDepth:   maxQueueDepth,
		clients:         NewClients(),
		internalMsgIds:  &internalIds{},
	}
	//start the goroutine that generates internal message ids for when clients receive messages
	//but are not connected.
	h.internalMsgIds.generateIds()
	return h
}

func (h *Hrotti) AddListener(name string, config *ListenerConfig) error {
	listener := *config
	listener.stop = make(chan struct{})

	h.listeners[name] = listener

	ln, err := net.Listen("tcp", listener.URL.Host)
	if err != nil {
		ERROR.Println(err.Error())
		return err
	}

	if listener.URL.Scheme == "ws" && listener.URL.Path == "" {
		return errors.New("WebSocket listeners must have a path")
	}

	INFO.Println("Starting MQTT listener on", listener.URL.String())

	//if this is a WebSocket listener
	if listener.URL.Scheme == "ws" {
		var server websocket.Server
		//override the Websocket handshake to accept any protocol name
		server.Handshake = func(c *websocket.Config, req *http.Request) (err error) {
			INFO.Println(c.Protocol)
			return err
		}
		//set up the ws connection handler, ie what we do when we get a new websocket connection
		server.Handler = func(ws *websocket.Conn) {
			ws.PayloadType = websocket.BinaryFrame
			INFO.Println("New incoming websocket connection", ws.RemoteAddr())
			h.InitClient(ws)
		}
		//set the path that the http server will recognise as related to this websocket
		//server, needs to be configurable really.
		http.Handle(listener.URL.Path, server.Handler)
		//ListenAndServe loops forever receiving connections and initiating the handler
		//for each one.
		go func(ln net.Listener) {
			err := http.Serve(ln, nil)
			if err != nil {
				ERROR.Println(err.Error())
				return
			}
		}(ln)
	} else {
		//loop forever accepting connections and launch InitClient as a goroutine with the connection
		go func(l *ListenerConfig) {
			for {
				conn, err := ln.Accept()
				INFO.Println("New incoming connection", conn.RemoteAddr())
				if err != nil {
					ERROR.Println(err.Error())
					continue
				}
				go h.InitClient(conn)
			}
		}(&listener)
	}
	return nil
}

func (h *Hrotti) Stop() {
	INFO.Println("Exiting...")
}

func (h *Hrotti) InitClient(conn net.Conn) {
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
	h.clients.Lock()
	c, ok := h.clients.list[cp.clientIdentifier]
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
			c.outboundMessages = make(chan *publishPacket, h.maxQueueDepth)
			c.outboundPriority = make(chan ControlPacket, h.maxQueueDepth)
		}
		//this function stays running until the client disconnects as the function called by an http
		//Handler has to remain running until its work is complete. So add one to the client waitgroup.
		c.Add(1)
		//create a new sync.Once for stopping with later, set the connections and create the stop channel.
		c.stopOnce = new(sync.Once)
		c.conn = conn
		c.bufferedConn = bufferedConn
		c.stop = make(chan struct{})
		//start the client.
		go c.Start(cp, h)
	} else {
		//This is a brand new client so create a NewClient and add to the clients map
		c = NewClient(conn, bufferedConn, cp.clientIdentifier, h.maxQueueDepth)
		h.clients.list[cp.clientIdentifier] = c
		//As before this function has to remain running but to avoid races we want to make sure its finished
		//before doing anything else so add it to the waitgroup so we can wait on it later
		c.Add(1)
		go c.Start(cp, h)
	}
	//finished with the clients hashmap
	h.clients.Unlock()
	//wait on the stop channel, we never actually send values down this channel but a closed channel with
	//return the default empty value for it's type without blocking.
	<-c.stop
	//call Done() on the client waitgroup.
	c.Done()
}

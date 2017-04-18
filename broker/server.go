package hrotti

import (
	"errors"
	"net"
	"net/http"
	"net/url"
	"sync"

	. "github.com/alsm/hrotti/packets"
	"github.com/google/uuid"
	"golang.org/x/net/websocket"
)

type Hrotti struct {
	PersistStore       Persistence
	listeners          map[string]*internalListener
	listenersWaitGroup sync.WaitGroup
	maxQueueDepth      int
	clients            *clients
	subs               *subscriptionMap
}

type internalListener struct {
	name        string
	url         url.URL
	connections []net.Conn
	stop        chan struct{}
}

func NewHrotti(maxQueueDepth int, persistence Persistence) *Hrotti {
	h := &Hrotti{
		PersistStore:  persistence,
		listeners:     make(map[string]*internalListener),
		maxQueueDepth: maxQueueDepth,
		clients:       newClients(),
		subs:          newSubMap(),
	}
	//start the goroutine that generates internal message ids for when clients receive messages
	//but are not connected.
	h.PersistStore.Init()
	return h
}

func (h *Hrotti) getClient(id string) *Client {
	h.clients.RLock()
	defer h.clients.RUnlock()
	return h.clients.list[id]
}

func (h *Hrotti) AddListener(name string, config *ListenerConfig) error {
	listener := &internalListener{name: name, url: *config.URL}
	listener.stop = make(chan struct{})

	h.listeners[name] = listener

	ln, err := net.Listen("tcp", listener.url.Host)
	if err != nil {
		ERROR.Println(err.Error())
		return err
	}

	if listener.url.Scheme == "ws" && len(listener.url.Path) == 0 {
		listener.url.Path = "/"
	}

	h.listenersWaitGroup.Add(1)
	INFO.Println("Starting MQTT listener on", listener.url.String())

	go func() {
		<-listener.stop
		INFO.Println("Listener", name, "is stopping...")
		ln.Close()
	}()
	//if this is a WebSocket listener
	if listener.url.Scheme == "ws" {
		var server websocket.Server
		//override the Websocket handshake to accept any protocol name
		server.Handshake = func(c *websocket.Config, req *http.Request) error {
			c.Origin, _ = url.Parse(req.RemoteAddr)
			c.Protocol = []string{"mqtt"}
			return nil
		}
		//set up the ws connection handler, ie what we do when we get a new websocket connection
		server.Handler = func(ws *websocket.Conn) {
			ws.PayloadType = websocket.BinaryFrame
			INFO.Println("New incoming websocket connection", ws.RemoteAddr())
			listener.connections = append(listener.connections, ws)
			h.InitClient(ws)
		}
		//set the path that the http server will recognise as related to this websocket
		//server, needs to be configurable really.
		http.Handle(listener.url.Path, server)
		//ListenAndServe loops forever receiving connections and initiating the handler
		//for each one.
		go func(ln net.Listener) {
			defer h.listenersWaitGroup.Done()
			err := http.Serve(ln, nil)
			if err != nil {
				ERROR.Println(err.Error())
				return
			}
		}(ln)
	} else {
		//loop forever accepting connections and launch InitClient as a goroutine with the connection
		go func() {
			defer h.listenersWaitGroup.Done()
			for {
				conn, err := ln.Accept()
				if err != nil {
					ERROR.Println(err.Error())
					return
				}
				INFO.Println("New incoming connection", conn.RemoteAddr())
				listener.connections = append(listener.connections, conn)
				go h.InitClient(conn)
			}
		}()
	}
	return nil
}

func (h *Hrotti) StopListener(name string) error {
	if listener, ok := h.listeners[name]; ok {
		close(listener.stop)
		for _, conn := range listener.connections {
			conn.Close()
		}
		delete(h.listeners, name)
		return nil
	}
	return errors.New("Listener not found")
}

func (h *Hrotti) Stop() {
	INFO.Println("Exiting...")
	for _, listener := range h.listeners {
		close(listener.stop)
	}
	h.listenersWaitGroup.Wait()
}

func (h *Hrotti) InitClient(conn net.Conn) {
	var sendSessionID bool
	/*var cph fixedHeader

	//create a bufio conn from the network connection
	bufferedConn := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))
	//first byte off the wire should be the msg type
	typeByte, _ := bufferedConn.ReadByte()
	//unpack the first byte into the fixed header
	cph.unpack(typeByte)

	if cph.messageType != CONNECT {
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
	cp := newControlPacket(CONNECT).(*connectPacket)
	cp.fixedHeader = cph
	cp.unpack(body)*/
	rp, _ := ReadPacket(conn)
	cp := rp.(*ConnectPacket)

	//Validate the CONNECT, check fields, values etc.
	rc := cp.Validate()
	//If it didn't validate...
	if rc != CONN_ACCEPTED {
		//and it wasn't because of a protocol violation...
		if rc != CONN_PROTOCOL_VIOLATION {
			//create and send a CONNACK with the correct rc in it.
			ca := NewControlPacket(CONNACK).(*ConnackPacket)
			ca.ReturnCode = rc
			ca.Write(conn)
		}
		//Put up a local message indicating an errored connection attempt and close the connection
		ERROR.Println(ConnackReturnCodes[rc], conn.RemoteAddr())
		conn.Close()
		return
	} else {
		//Put up an INFO message with the client id and the address they're connecting from.
		INFO.Println(ConnackReturnCodes[rc], cp.ClientIdentifier, conn.RemoteAddr())
	}

	//check for a zero length client id and if it exists create one from the UUID library and return
	//it on $SYS/session_identifier
	if len(cp.ClientIdentifier) == 0 {
		cp.ClientIdentifier = uuid.New().String()
		sendSessionID = true
	}
	//Lock the clients hashmap while we check if we already know this clientid.
	h.clients.Lock()
	c, ok := h.clients.list[cp.ClientIdentifier]
	if ok && cp.CleanSession {
		//and if we do, if the clientid is currently connected...
		if c.Connected() {
			INFO.Println("Clientid", c.clientID, "already connected, stopping first client")
			//stop the parts of it that need to stop before we can change the network connection it's using.
			c.StopForTakeover()
		} else {
			//if the clientid known but not connected, ie cleansession false
			INFO.Println("Durable client reconnecting", c.clientID)
			//disconnected client will no longer have the channels for messages
			c.outboundMessages = make(chan *PublishPacket, h.maxQueueDepth)
			c.outboundPriority = make(chan ControlPacket, h.maxQueueDepth)
		}
		//this function stays running until the client disconnects as the function called by an http
		//Handler has to remain running until its work is complete. So add one to the client waitgroup.
		c.Add(1)
		//create a new sync.Once for stopping with later, set the connections and create the stop channel.
		c.stopOnce = new(sync.Once)
		c.conn = conn
		//c.bufferedConn = bufferedConn
		c.stop = make(chan struct{})
		//start the client.
		go c.Start(cp, h)
	} else {
		//This is a brand new client so create a NewClient and add to the clients map
		c = newClient(conn, cp.ClientIdentifier, h.maxQueueDepth)
		h.clients.list[cp.ClientIdentifier] = c
		if sendSessionID {
			go func() {
				sessionIDPacket := NewControlPacket(PUBLISH).(*PublishPacket)
				sessionIDPacket.TopicName = "$SYS/session_identifier"
				sessionIDPacket.Payload = []byte(cp.ClientIdentifier)
				sessionIDPacket.Qos = 1
				c.outboundMessages <- sessionIDPacket
			}()
		}
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

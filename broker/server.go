package hrotti

import (
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"code.google.com/p/go.net/websocket"
)

type Hrotti struct {
	inboundPersist  Persistence
	outboundPersist Persistence
	config          ConfigObject
	clients         Clients
	internalMsgIds  *internalIds
}

func NewHrotti(config ConfigObject) *Hrotti {
	h := &Hrotti{
		NewMemoryPersistence(),
		NewMemoryPersistence(),
		config,
		NewClients(),
		&internalIds{},
	}
	return h
}

func (h *Hrotti) Run() {
	//start the goroutine that generates internal message ids for when clients receive messages
	//but are not connected.
	h.internalMsgIds.generateIds()

	//for each configured listener start a go routine that is listening on the port set for
	//that listener
	for _, listener := range h.config.Listeners {
		go func(l *ListenerConfig) {
			serverAddress := l.Host + ":" + l.Port
			//if this is a WebSocket listener
			if l.WS {
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
					InitClient(ws, h)
				}
				//set the path that the http server will recognise as related to this websocket
				//server, needs to be configurable really.
				http.Handle("/mqtt", server.Handler)
				INFO.Println("Starting MQTT WebSocket listener on", serverAddress)
				//ListenAndServe loops forever receiving connections and initiating the handler
				//for each one.
				err := http.ListenAndServe(serverAddress, nil)
				if err != nil {
					ERROR.Println(err.Error())
					os.Exit(1)
				}
			} else {
				//this is a tcp listener, so listen on the server address given (combo of host and port)
				ln, err := net.Listen("tcp", serverAddress)
				if err != nil {
					ERROR.Println(err.Error())
					os.Exit(1)
				}
				INFO.Println("Starting MQTT TCP listener on", serverAddress)
				//loop forever accepting connections and launch InitClient as a goroutine with the connection
				for {
					conn, err := ln.Accept()
					INFO.Println("New incoming connection", conn.RemoteAddr())
					if err != nil {
						ERROR.Println(err.Error())
						continue
					}
					go InitClient(conn, h)
				}
			}
		}(listener)
	}
	//listen for ctrl+c and use that a signal to exit the program
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	INFO.Println("Exiting...")
}

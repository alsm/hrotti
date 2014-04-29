package main

import (
	"code.google.com/p/go.net/websocket"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
)

var (
	inboundPersist  Persistence
	outboundPersist Persistence
	config          ConfigObject
)

//global struct with a map of clientid to Client pointer and a RW Mutex to protect access.
var clients struct {
	sync.RWMutex
	list map[string]*Client
}

//init functions run before everything else
func init() {
	var (
		configFile = flag.String("conf", "", "A configuration file")
	)
	flag.Parse()

	clients.list = make(map[string]*Client)

	if *configFile == "" {
		host := os.Getenv("HROTTI_HOST")
		port := os.Getenv("HROTTI_PORT")
		ws, _ := strconv.ParseBool(os.Getenv("HROTTI_USE_WEBSOCKETS"))
		if port == "" {
			port = "1883"
		}
		config.Listeners = append(config.Listeners, &ListenerConfig{host, port, ws})
	} else {
		err := ParseConfig(*configFile, &config)
		if err != nil {
			os.Stderr.WriteString(fmt.Sprintf("%s\n", err.Error()))
		}
	}
	configureLogger(config.GetLogTarget("info"),
		config.GetLogTarget("protocol"),
		config.GetLogTarget("error"),
		config.GetLogTarget("debug"))

	//currently the only persistence mechanism provided is the MemoryPersistence.
	inboundPersist = NewMemoryPersistence()
	outboundPersist = NewMemoryPersistence()

	//start the goroutine that generates internal message ids for when clients receive messages
	//but are not connected.
	genInternalIds()
}

func main() {
	StartPlugins()
	//for each configured listener start a go routine that is listening on the port set for
	//that listener
	for _, listener := range config.Listeners {
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
					InitClient(ws)
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
					go InitClient(conn)
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

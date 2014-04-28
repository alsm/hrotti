package main

import (
	"code.google.com/p/go.net/websocket"
	"flag"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
)

//loggers and the global inbound/outbound persistence. Persistence is an interface
var (
	INFO            *log.Logger
	PROTOCOL        *log.Logger
	ERROR           *log.Logger
	DEBUG           *log.Logger
	inboundPersist  Persistence
	outboundPersist Persistence
)

//global struct with a map of clientid to Client pointer and a RW Mutex to protect access.
var clients struct {
	sync.RWMutex
	list map[string]*Client
}

var config ConfigObject

//set up the endpoints for the various loggers, needs to be made configurable.
func configureLogger(infoHandle io.Writer, protocolHandle io.Writer, errorHandle io.Writer, debugHandle io.Writer) {
	INFO = log.New(infoHandle,
		"INFO: ",
		log.Ldate|log.Ltime)

	PROTOCOL = log.New(protocolHandle,
		"PROTOCOL: ",
		log.Ldate|log.Ltime)

	ERROR = log.New(errorHandle,
		"ERROR: ",
		log.Ldate|log.Ltime|log.Lshortfile)

	DEBUG = log.New(debugHandle,
		"DEBUG: ",
		log.Ldate|log.Ltime|log.Lshortfile)
}

//init functions run before everything else
func init() {
	configureLogger(os.Stdout, os.Stdout, os.Stderr, ioutil.Discard)
	clients.list = make(map[string]*Client)

	var configFile string
	flag.StringVar(&configFile, "conf", "", "A configuration file")
	flag.IntVar(&config.maxQueueDepth, "maxqd", 200, "Maximum unacknowledged message queue depth")
	flag.Parse()

	if configFile == "" {
		host := os.Getenv("HROTTI_HOST")
		port := os.Getenv("HROTTI_PORT")
		ws, _ := strconv.ParseBool(os.Getenv("HROTTI_USE_WEBSOCKETS"))
		if port == "" {
			port = "1883"
		}
		config.Listeners = append(config.Listeners, &ListenerConfig{host, port, ws})
	} else {
		err := ParseConfig(configFile, &config)
		if err != nil {
			ERROR.Println(err.Error())
		}
	}

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

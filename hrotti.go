package main

import (
	"code.google.com/p/go.net/websocket"
	//"code.google.com/p/leveldb-go/leveldb"
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

var (
	INFO            *log.Logger
	PROTOCOL        *log.Logger
	ERROR           *log.Logger
	WebSocket       bool
	inboundPersist  Persistence
	outboundPersist Persistence
)

var clients struct {
	sync.RWMutex
	list map[string]*Client
}

type ConfigObject struct {
	Listeners []*ListenerConfig `json:"listeners"`
}

var config ConfigObject

func configureLogger(infoHandle io.Writer, protocolHandle io.Writer, errorHandle io.Writer) {
	INFO = log.New(infoHandle,
		"INFO: ",
		log.Ldate|log.Ltime)

	PROTOCOL = log.New(protocolHandle,
		"PROTOCOL: ",
		log.Ldate|log.Ltime|log.Lshortfile)

	ERROR = log.New(errorHandle,
		"ERROR: ",
		log.Ldate|log.Ltime|log.Lshortfile)
}

func init() {
	configureLogger(os.Stdout, ioutil.Discard, os.Stderr)
	clients.list = make(map[string]*Client)

	var configFile string
	flag.StringVar(&configFile, "conf", "", "A configuration file")
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

	inboundPersist = NewMemoryPersistence()
	outboundPersist = NewMemoryPersistence()
}

func main() {
	StartPlugins()
	for _, listener := range config.Listeners {
		go func(l *ListenerConfig) {
			serverAddress := l.Host + ":" + l.Port
			if l.WS {
				var server websocket.Server
				server.Handshake = func(c *websocket.Config, req *http.Request) (err error) {
					INFO.Println(c.Protocol)
					return err
				}
				server.Handler = func(ws *websocket.Conn) {
					ws.PayloadType = websocket.BinaryFrame
					INFO.Println("New incoming websocket connection", ws.RemoteAddr())
					InitClient(ws)
				}
				http.Handle("/mqtt", server.Handler)
				INFO.Println("Starting MQTT WebSocket listener on", serverAddress)
				err := http.ListenAndServe(serverAddress, nil)
				if err != nil {
					ERROR.Println(err.Error())
					os.Exit(1)
				}
			} else {
				ln, err := net.Listen("tcp", serverAddress)
				if err != nil {
					ERROR.Println(err.Error())
					os.Exit(1)
				}
				INFO.Println("Starting MQTT TCP listener on", serverAddress)
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
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	INFO.Println("Exiting...")
}

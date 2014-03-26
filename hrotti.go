package main

import (
	"code.google.com/p/go.net/websocket"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
)

var (
	INFO      *log.Logger
	PROTOCOL  *log.Logger
	ERROR     *log.Logger
	WebSocket bool
)

var clients struct {
	sync.RWMutex
	list map[string]*Client
}

var config struct {
	Server string
}

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
	clients.list = make(map[string]*Client)
	Host := os.Getenv("HROTTI_HOST")
	Port := os.Getenv("HROTTI_PORT")
	WebSocket, _ = strconv.ParseBool(os.Getenv("HROTTI_USE_WEBSOCKETS"))
	if Port == "" {
		Port = "1883"
	}
	config.Server = Host + ":" + Port
}

func main() {
	configureLogger(os.Stdout, ioutil.Discard, os.Stderr)
	if WebSocket {
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
		err := http.ListenAndServe(config.Server, nil)
		if err != nil {
			panic("ListenAndServe: " + err.Error())
		}
	} else {
		ln, err := net.Listen("tcp", config.Server)
		if err != nil {
			ERROR.Println(err.Error())
			os.Exit(1)
		}
		INFO.Println("Started MQTT Broker on", config.Server)
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
}

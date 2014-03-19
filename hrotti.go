package main

import (
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"sync"
)

var (
	INFO     *log.Logger
	PROTOCOL *log.Logger
	ERROR    *log.Logger
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
		log.Ldate|log.Ltime|log.Lshortfile)

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
	config.Server = Host + ":" + Port
}

func main() {
	configureLogger(os.Stdout, ioutil.Discard, os.Stderr)
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

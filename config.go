package main

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"net/url"
	"os"

	. "github.com/alsm/hrotti/broker"
)

type ListenerEntry struct {
	URL string `json:"url"`
}

//Current configuration struct, maxQueueDepth sets the maximum number of unacknowledged mesages
//for a client. Listeners is a slice of ListenerConfigs
type BrokerConfig struct {
	MaxQueueDepth   int                       `json:"maxQueueDepth"`
	ListenerEntries map[string]*ListenerEntry `json:"listeners"`
	Listeners       map[string]*ListenerConfig
	Logging         struct {
		Info     string `json:"info"`
		Protocol string `json:"protocol"`
		Errlog   string `json:"error"`
		Debug    string `json:"debug"`
	}
}

var logTargets map[string]io.Writer = map[string]io.Writer{
	"stdout":  os.Stdout,
	"stderr":  os.Stderr,
	"discard": ioutil.Discard,
}

func (c *BrokerConfig) SetLogTargets() {
	target, ok := logTargets[c.Logging.Info]
	if !ok {
		target = os.Stdout
	}
	INFO = log.New(target, "INFO: ", log.Ldate|log.Ltime)
	target, ok = logTargets[c.Logging.Protocol]
	if !ok {
		target = ioutil.Discard
	}
	PROTOCOL = log.New(target, "PROTOCOL: ", log.Ldate|log.Ltime)
	target, ok = logTargets[c.Logging.Errlog]
	if !ok {
		target = os.Stderr
	}
	ERROR = log.New(target, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)
	target, ok = logTargets[c.Logging.Debug]
	if !ok {
		target = ioutil.Discard
	}
	DEBUG = log.New(target, "DEBUG: ", log.Ldate|log.Ltime|log.Lshortfile)
}

func ParseConfig(confFile string, confVar *BrokerConfig) error {
	file, err := os.Open(confFile)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(file)

	err = decoder.Decode(confVar)
	if err != nil {
		return err
	}

	for name, entry := range confVar.ListenerEntries {
		url, err := url.Parse(entry.URL)
		if err != nil {
			return err
		}
		confVar.Listeners[name] = &ListenerConfig{URL: url}
	}
	return nil
}

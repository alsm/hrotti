package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	. "github.com/alsm/hrotti/broker"
)

func createConfig() BrokerConfig {
	configFile := flag.String("conf", "", "A configuration file")

	flag.Parse()

	var config BrokerConfig
	config.ListenerEntries = make(map[string]*ListenerEntry)
	config.Listeners = make(map[string]*ListenerConfig)

	if *configFile == "" {
		listener := NewListenerConfig(os.Getenv("HROTTI_URL"))
		if listener.URL.Host == "" {
			listener = NewListenerConfig("tcp://0.0.0.0:1883")
		}
		config.Listeners["envconfig"] = listener
		config.MaxQueueDepth = 100
	} else {
		fmt.Println("Reading config file", *configFile)
		err := ParseConfig(*configFile, &config)
		if err != nil {
			os.Stderr.WriteString(fmt.Sprintf("%s\n", err.Error()))
		}
	}
	config.SetLogTargets()
	return config
}

func main() {
	config := createConfig()

	//r := &RedisPersistence{Server: ":6379"}
	r := &MemoryPersistence{}
	h := NewHrotti(config.MaxQueueDepth, r)

	for name, listener := range config.Listeners {
		h.AddListener(name, listener)
	}
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	h.Stop()
}

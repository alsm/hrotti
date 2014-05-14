package main

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"syscall"

	. "github.com/alsm/hrotti/broker"
)

func createConfig() BrokerConfig {
	configFile := flag.String("conf", "", "A configuration file")

	flag.Parse()

	var config BrokerConfig
	config.Listeners = make(map[string]ListenerConfig)

	if *configFile == "" {
		server, err := url.Parse(os.Getenv("HROTTI_URL"))
		if err != nil {
			panic(err.Error())
		}
		if server.Host == "" {
			server, _ = url.Parse("tcp://0.0.0.0:1883")
		}
		config.Listeners["envconfig"] = *(&ListenerConfig{URL: server})
	} else {
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

	h := NewHrotti(config.MaxQueueDepth)

	for name, listener := range config.Listeners {
		h.AddListener(name, &listener)
	}
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	h.Stop()
}

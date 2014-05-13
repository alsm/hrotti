package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	. "github.com/alsm/hrotti/broker"
)

func createConfig() ConfigObject {
	configFile := flag.String("conf", "", "A configuration file")

	flag.Parse()

	var config ConfigObject

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
	ConfigureLogger(config.GetLogTarget("info"),
		config.GetLogTarget("protocol"),
		config.GetLogTarget("error"),
		config.GetLogTarget("debug"))
	return config
}

func main() {
	config := createConfig()

	h := NewHrotti(config)

	go h.Run()
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	h.Stop()
}

package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/alsm/hrotti/broker"
)

func main() {
	h := hrotti.NewHrotti(100, &hrotti.MemoryPersistence{})
	hrotti.INFO.SetOutput(os.Stdout)
	hrotti.DEBUG.SetOutput(os.Stdout)
	hrotti.ERROR.SetOutput(os.Stdout)
	h.AddListener("test", hrotti.NewListenerConfig("tcp://0.0.0.0:1883"))

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	h.Stop()
}

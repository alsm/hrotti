Hrotti
======

Hrotti is both a library that provides an MQTT server and a wrapper program around that library that provides a standalone MQTT server.

When used as a library you create a broker with the NewHrotti(maxQueueDepth int) function. This returns a broker with no listeners, the maxQueueDepth option is the number of messages that will be allowed to queue up for a client before any new messages that would be sent to that client are thrown away.

To add a new listener to the broker you use AddListener(name string, config *ListenerConfig) :) which takes a pointer to a broker as the receiver. name is just a string to identify this listener, config is a pointer to a ListenerConfig currently the only important field in a ListenerConfig is URL which is a [url.URL](http://golang.org/pkg/net/url/#URL)

To stop a listener use StopListener(name string) which again takes a broker as the receiver, name is the name of the listener as given in AddListener(), it returns a nil error on success, otherwise an error to indicate it could not find the named listener. Calling StopListener() will disconnect all clients currently connected to that listener.

Here's a simple example that implements a tcp MQTT server on port 1883
```
package main

import (
	"github.com/alsm/hrotti/broker"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	h := hrotti.NewHrotti(100)
	hrotti.INFO = log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime)
	h.AddListener("test", hrotti.NewListenerConfig("tcp://0.0.0.0:1883"))

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	h.Stop()
}
```

A slightly more extensive implementation is provided with this library, running go build in the project directory will produce a binary called hrotti which allows for configuration of multiple listeners with a json config file. If only a single listener is required though you can just set the HROTTI_URL environment variable.
Only tcp and ws URL schemes are supported, eg: tcp://0.0.0.0:1883 or ws://0.0.0.0:1883/mqtt
With a websocket URL if no path is specified it will automatically serve on /

Alternatively a configuration file in json can be provided allowing the creation of multiple listeners, currently all listeners share the same root node in the topic tree. To pass a configuration file use the command line option "-conf", for example;
```
hrotti -conf config.json
```
The configuration expects an object called "listeners" which is a map of the listener name to a json representation of a ListenerConfig, currently only the url can be specified.

A listener only listens via tcp or websockets and not both on the same port.

An example configuration file is shown below
```
{
	"maxQueueDepth": 100,
	"listeners":{
		"tcp":{
			"url":"tcp://0.0.0.0:1883"
		},
		"websockets":{
			"url":"ws://0.0.0.0:2000/mqtt"
		}
	}
}
```

The current persistence mechanism is in memory only.
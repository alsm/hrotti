hrotti
======

An MQTT 3.1 broker written in Go

If only a single listener is required configuration can be done using environment variables;

HROTTI_HOST - the local host or ip address to bind to (0.0.0.0 or unset for all interfaces)  
HROTTI_PORT - the port to listen on  
HROTTI_USE_WEBSOCKETS - true or false (default) to listen for websocket connections rather than tcp  

Alternatively a configuration file in json can be provided allowing the creation of multiple listeners, currently all listeners share the same root node in the topic tree, in the future this should be configurable. To pass a configuration file use the command line option "-conf", for example;
```
hrotti -conf config.json
```
The configuration expects an array called "Listeners" with each element in the array being an object that can have the following defined;

host - a string of the hostname/ip to bind to  
port - a string of the port to bind to  
enable_ws - a boolean value indicating whether this listener should use websockets  

A listener only listens via tcp or websockets, not both on the same port.

An example configuration file is shown below
```
{
	"Listeners": [
	{
		"host":"0.0.0.0",
		"port":"1884",
		"enable_ws":false
	},
	{
		"host":"0.0.0.0",
		"port":"1885",
		"enable_ws":true
	}]
}
```

The current persistence mechanism is in memory only.
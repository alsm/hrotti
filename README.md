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

I've just added a new feature, it's raw but there is a plugin that will allow you to subscribe to a search term from the twitter streams api [](https://dev.twitter.com/docs/api/streaming). Currently the plugin only allows you track keywords. Also the twitter api only allows you to make one connection to the streams endpoints at a time, each client that subscribes will change the messages coming through and previous clients will stay subscribed to the new feed.

To enable the plugin you need to create a configuration file called twitter_plugin_config.json that goes in the same directory that Hrotti is being run from, currently 4 things are required to be defined in a single object as per the example below;
```
{
	"consumerKey":"<secret>",
	"consumerSecret":"<secret>",
	"accessToken":"<secret>",
	"accessSecret":"<secret>"
}
```
To use the functionality make a subscription to $twitter/<search terms>
Messages are delivered to subscribers at QoS0 the topic of the message is the screen name of the tweeter and the content of the message is the body of the tweet.
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

I've just added a new feature, it's raw but there is a plugin that will allow you to subscribe to a search term from the twitter streams api [https://dev.twitter.com/docs/api/streaming](https://dev.twitter.com/docs/api/streaming). Currently the plugin only allows you track keywords. Also the twitter api only allows you to make one connection to the streams endpoints at a time, each client that subscribes will change the messages coming through and previous clients will stay subscribed to the new feed.

To enable the plugin you need to create a configuration file called twitter_plugin_config.json that goes in the same directory that Hrotti is being run from, currently 4 things are required to be defined in a single object as per the example below;
```
{
	"consumerKey":"<secret>",
	"consumerSecret":"<secret>",
	"accessToken":"<secret>",
	"accessSecret":"<secret>"
}
```
To use the functionality make a subscription to $twitter/_keyword_  
Messages are delivered to subscribers at QoS0 the topic of the message is the screen name of the tweeter and the content of the message is the body of the tweet.

And on the topic of plugins there is another new one, a redirect plugin; messages published to one topic will be republished on another. For example a message published to hampshire/trees would also be delivered to subscribers to england/trees. Why would you want to do this? You want to set up a consolidation endpoint without having all your clients have to make multiple subscriptions. Perhaps extending the last example you know that there will be new relevant topics coming along and you can't change your currently deployed client configurations.
The messages received by subscribers to the 2nd topic retain the original topic name they were published to, ie in the example above a message published to hampshire/trees and delivered to subscribers to england/trees would still have hampshire/trees in the topic, particularly useful if the topic structure has some implied meaning about the message.

To enable the redirect plugin it looks for a configuration file call redirect_plugin_config.json that also goes in the same directory that Hrotti is being run from. The configuration is a list of mappings, source topic to destination topic;
```
{
	"hampshire/trees":"england/trees"
}
```
The plugin does support multiple redirect entries but will not protect you from setting up circular redirects. Also in theory wildcards on the left hand side should work but I haven't tested it yet. Wildcards on the right will not work but right now they are not validated.
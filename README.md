hrotti
======

An MQTT broker written in Go

So this is really just getting started and is just the barest of bones atm. There's no configuration, it listens on TCP 1883 and that's it.
It'll do the packet flows for QoS 1 and 2 publishes from a client but it won't persist the messages.
~~It doesn't support sending Qos 1 or 2 publishes.~~
~~It doesn't yet do retained messages.~~
Hrotti does now handle Qos 1 and 2 publishes, and handles retained messages! :)
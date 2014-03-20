hrotti
======

An MQTT 3.1 broker written in Go

Only serves on a single host and a single port atm, and only handles tcp connections (no TLS or Websockets).
This is configured by setting two environment variables;

HROTTI_HOST - the local host or ip address to bind to (0.0.0.0 or unset for all interfaces)  
HROTTI_PORT - the port to listen on

Hrotti supports;
* QoS 0, 1 and 2 for sending/receiving/subscriptions
* Retained messages
* Topic wildcards

It does not do any persistence of messages, QoS 1 and 2 support is only for completing the packet flows.
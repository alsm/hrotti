package hrotti

import (
	"sync/atomic"
)

type stat int64

type BrokerStats struct {
	bytesReceived           int64
	bytesSent               int64
	clientsConnected        int64
	clientsDisconnected     int64
	clientsMaximum          int64
	clientsTotal            int64
	messagesInflight        int64
	messagesReceived        int64
	messagesSent            int64
	messagesStored          int64
	publishMessagesDropped  int64
	publishMessagesReceived int64
	publishMessagesSent     int64
	messagesRetained        int64
	subscriptions           int64
	brokerTime              int64
	brokerUptime            int64
}

func (b *BrokerStats) AddClient() {
	atomic.AddInt64(&b.clientsConnected, 1)
}

package main

import (
	"sync"
)

//a persistence entry is a map of msgIds and ControlPackets
type MemoryPersistenceEntry struct {
	sync.Mutex
	messages map[msgId]ControlPacket
}

//the MemoryPersistence struct is a map of Client pointers to pointers
//to a Persistence Entry. So each Client has its own map of msgIds/packets.
type MemoryPersistence struct {
	sync.RWMutex
	clients map[*Client]*MemoryPersistenceEntry
}

func NewMemoryPersistence() *MemoryPersistence {
	//init the Memory persistence, we haven't created an persistenceentrys yet
	p := &MemoryPersistence{
		clients: make(map[*Client]*MemoryPersistenceEntry),
	}
	return p
}

func (p *MemoryPersistence) Open(client *Client) {
	//lock the whole persistence store while we add a new Client entry
	p.Lock()
	defer p.Unlock()
	DEBUG.Println("Opening memory persistence for", client.clientId)
	//init the MemoryPersistenceEntry for this client
	p.clients[client] = &MemoryPersistenceEntry{messages: make(map[msgId]ControlPacket)}
}

func (p *MemoryPersistence) Close(client *Client) {
	//lock the whole persistence store while we delete a client from the map
	p.Lock()
	defer p.Unlock()
	delete(p.clients, client)
}

func (p *MemoryPersistence) Add(client *Client, message ControlPacket) bool {
	//only need to get a read lock on the persistence store, but lock the underlying
	//persistenceentry for the client we're working with.
	p.RLock()
	p.clients[client].Lock()
	defer p.clients[client].Unlock()
	defer p.RUnlock()
	//the msgid is the key in the persistence entry
	id := message.MsgId()
	DEBUG.Println("Persisting", packetNames[message.Type()], "packet for", client.clientId, id)
	//if there is already an entry for this message id return false
	if _, ok := p.clients[client].messages[id]; ok {
		return false
	}
	//otherwise insert this message into the map
	p.clients[client].messages[id] = message
	return true
}

func (p *MemoryPersistence) Replace(client *Client, message ControlPacket) bool {
	//only need to get a read lock on the persistence store, but lock the underlying
	//persistenceentry for the client we're working with.
	p.RLock()
	p.clients[client].Lock()
	defer p.clients[client].Unlock()
	defer p.RUnlock()

	id := message.MsgId()
	DEBUG.Println("Replacing persisted message for", client.clientId, id, "with", packetNames[message.Type()])
	//For QoS2 flows we want to replace the original PUBLISH with the related PUBREL
	//as it maintains the same message id
	p.clients[client].messages[id] = message
	return true
}

func (p *MemoryPersistence) AddBatch(batch map[*Client]*publishPacket) {
	//adding messages to many different client entries at the same time, as we're doing
	//this grabbing a full lock on the whole persistence mechanism
	p.Lock()
	defer p.Unlock()
	//the batch is a map keyed by Client and value is a pointer to a PUBLISH
	//for each create an appropriate entry
	for client, message := range batch {
		p.clients[client].messages[message.messageId] = message
	}
}

func (p *MemoryPersistence) Delete(client *Client, id msgId) bool {
	//only need to get a read lock on the persistence store, but lock the underlying
	//persistenceentry for the client we're working with.
	p.RLock()
	p.clients[client].Lock()
	defer p.clients[client].Unlock()
	defer p.RUnlock()
	//checks that there is actually an entry for the message id we're being asked to
	//delete, if there isn't return false, otherwise delete the entry.
	DEBUG.Println("Removing persisted message for", client.clientId)
	if _, ok := p.clients[client].messages[id]; !ok {
		return false
	}
	delete(p.clients[client].messages, id)
	return true
}

func (p *MemoryPersistence) GetAll(client *Client) (messages []ControlPacket) {
	//only need to get a read lock on the persistence store, but lock the underlying
	//persistenceentry for the client we're working with.
	p.RLock()
	p.clients[client].Lock()
	defer p.clients[client].Unlock()
	defer p.RUnlock()
	//Get every message in the persistence store for a given Client, create a slice
	//of the ControlPackets (not just PUBLISHES in there)
	for _, message := range p.clients[client].messages {
		messages = append(messages, message)
	}
	return messages
}

func (p *MemoryPersistence) Exists(client *Client) bool {
	//grab a read lock on the persistence and check if there is already an entry
	//for this client.
	p.RLock()
	defer p.RUnlock()
	_, ok := p.clients[client]
	return ok
}

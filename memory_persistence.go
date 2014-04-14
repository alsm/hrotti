package main

import (
	"sync"
)

type MemoryPersistenceEntry struct {
	sync.Mutex
	messages map[msgId]ControlPacket
}

type MemoryPersistence struct {
	sync.RWMutex
	clients map[*Client]*MemoryPersistenceEntry
}

func NewMemoryPersistence() *MemoryPersistence {
	p := &MemoryPersistence{
		clients: make(map[*Client]*MemoryPersistenceEntry),
	}
	return p
}

func (p *MemoryPersistence) Open(client *Client) {
	p.Lock()
	defer p.Unlock()
	DEBUG.Println("Opening memory persistence for", client.clientId)
	p.clients[client] = &MemoryPersistenceEntry{messages: make(map[msgId]ControlPacket)}
}

func (p *MemoryPersistence) Close(client *Client) {
	p.Lock()
	defer p.Unlock()

	delete(p.clients, client)
}

func (p *MemoryPersistence) Add(client *Client, id msgId, message ControlPacket) bool {
	p.RLock()
	p.clients[client].Lock()
	defer p.clients[client].Unlock()
	defer p.RUnlock()

	DEBUG.Println("Persisting", packetNames[message.Type()], "packet for", client.clientId, id)
	if _, ok := p.clients[client].messages[id]; ok {
		return false
	}
	p.clients[client].messages[id] = message
	return true
}

func (p *MemoryPersistence) Replace(client *Client, id msgId, message ControlPacket) bool {
	p.RLock()
	p.clients[client].Lock()
	defer p.clients[client].Unlock()
	defer p.RUnlock()

	DEBUG.Println("Replacing persisted message for", client.clientId, id, "with", packetNames[message.Type()])
	p.clients[client].messages[id] = message
	return true
}

func (p *MemoryPersistence) AddBatch(batch map[*Client]*publishPacket) {
	p.Lock()
	defer p.Unlock()

	for client, message := range batch {
		p.clients[client].messages[message.messageId] = message
	}
}

func (p *MemoryPersistence) Delete(client *Client, id msgId) bool {
	p.RLock()
	p.clients[client].Lock()
	defer p.clients[client].Unlock()
	defer p.RUnlock()

	DEBUG.Println("Removing persisted message for", client.clientId)
	if _, ok := p.clients[client].messages[id]; !ok {
		return false
	}
	delete(p.clients[client].messages, id)
	return true
}

func (p *MemoryPersistence) GetAll(client *Client) (messages []ControlPacket) {
	p.RLock()
	p.clients[client].Lock()
	defer p.clients[client].Unlock()
	defer p.RUnlock()

	for _, message := range p.clients[client].messages {
		messages = append(messages, message)
	}
	return messages
}

func (p *MemoryPersistence) Exists(client *Client) bool {
	_, ok := p.clients[client]
	return ok
}

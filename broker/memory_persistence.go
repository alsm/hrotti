package hrotti

import (
	. "github.com/alsm/hrotti/packets"
	"github.com/google/uuid"
	"sync"
)

//a persistence entry is a map of msgIds and ControlPackets
type MemoryPersistenceEntry struct {
	sync.Mutex
	messages map[string]ControlPacket
}

//the MemoryPersistence struct is a map of client pointers to pointers
//to a Persistence Entry. So each client has its own map of msgIds/packets.
type MemoryPersistence struct {
	sync.RWMutex
	inbound  map[string]*MemoryPersistenceEntry
	outbound map[string]*MemoryPersistenceEntry
}

func (p *MemoryPersistence) Init() error {
	//init the Memory persistence, we haven't created our persistenceentrys yet
	p.inbound = make(map[string]*MemoryPersistenceEntry)
	p.outbound = make(map[string]*MemoryPersistenceEntry)
	return nil
}

func (p *MemoryPersistence) Open(client string) {
	//lock the whole persistence store while we add a new client entry
	p.Lock()
	defer p.Unlock()
	DEBUG.Println("Opening memory persistence for", client)
	//init the MemoryPersistenceEntry for this client
	p.inbound[client] = &MemoryPersistenceEntry{messages: make(map[string]ControlPacket)}
	p.outbound[client] = &MemoryPersistenceEntry{messages: make(map[string]ControlPacket)}
}

func (p *MemoryPersistence) Close(client string) {
	//lock the whole persistence store while we delete a client from the map
	p.Lock()
	defer p.Unlock()
	delete(p.inbound, client)
	delete(p.outbound, client)
}

func (p *MemoryPersistence) Add(client string, direction dirFlag, message ControlPacket) bool {
	//only need to get a read lock on the persistence store, but lock the underlying
	//persistenceentry for the client we're working with.
	p.RLock()
	defer p.RUnlock()
	//the uuid is the key in the persistence entry
	id := message.UUID().String()
	switch direction {
	case INBOUND:
		p.inbound[client].Lock()
		defer p.inbound[client].Unlock()
		DEBUG.Println("Persisting inbound packet for", client, id)
		//if there is already an entry for this message id return false
		if _, ok := p.inbound[client].messages[id]; ok {
			return false
		}
		//otherwise insert this message into the map
		p.inbound[client].messages[id] = message
	case OUTBOUND:
		p.outbound[client].Lock()
		defer p.outbound[client].Unlock()
		DEBUG.Println("Persisting outbound packet for", client, id)
		//if there is already an entry for this message id return false
		if _, ok := p.outbound[client].messages[id]; ok {
			return false
		}
		//otherwise insert this message into the map
		p.outbound[client].messages[id] = message
	}
	return true
}

func (p *MemoryPersistence) Replace(client string, direction dirFlag, message ControlPacket) bool {
	//only need to get a read lock on the persistence store, but lock the underlying
	//persistenceentry for the client we're working with.
	p.RLock()
	defer p.RUnlock()

	id := message.UUID().String()
	switch direction {
	//For QoS2 flows we want to replace the original PUBLISH with the related PUBREL
	//as it maintains the same message id
	case INBOUND:
		p.inbound[client].Lock()
		defer p.inbound[client].Unlock()
		DEBUG.Println("Replacing persisted message for", client, id)
		//if there is already an entry for this message id return false
		if _, ok := p.inbound[client].messages[id]; ok {
			return false
		}
		//otherwise insert this message into the map
		p.inbound[client].messages[id] = message
	case OUTBOUND:
		p.outbound[client].Lock()
		defer p.outbound[client].Unlock()
		DEBUG.Println("Replacing persisted message for", client, id)
		//if there is already an entry for this message id return false
		if _, ok := p.outbound[client].messages[id]; ok {
			return false
		}
		//otherwise insert this message into the map
		p.outbound[client].messages[id] = message
	}
	return true
}

func (p *MemoryPersistence) AddBatch(batch map[string]*PublishPacket) {
	//adding messages to many different client entries at the same time, as we're doing
	//this grabbing a full lock on the whole persistence mechanism
	p.Lock()
	defer p.Unlock()
	//the batch is a map keyed by client and value is a pointer to a PUBLISH
	//for each create an appropriate entry
	for client, message := range batch {
		p.inbound[client].messages[message.UUID().String()] = message
	}
}

func (p *MemoryPersistence) Delete(client string, direction dirFlag, uid uuid.UUID) bool {
	//only need to get a read lock on the persistence store, but lock the underlying
	//persistenceentry for the client we're working with.
	p.RLock()
	defer p.RUnlock()
	//checks that there is actually an entry for the message id we're being asked to
	//delete, if there isn't return false, otherwise delete the entry.
	id := uid.String()
	DEBUG.Println("Removing persisted message for", client)
	switch direction {
	case INBOUND:
		p.inbound[client].Lock()
		defer p.inbound[client].Unlock()
		//if there is already an entry for this message id return false
		if _, ok := p.inbound[client].messages[id]; !ok {
			return false
		}
		delete(p.inbound[client].messages, id)
	case OUTBOUND:
		p.outbound[client].Lock()
		defer p.outbound[client].Unlock()
		//if there is already an entry for this message id return false
		if _, ok := p.outbound[client].messages[id]; !ok {
			return false
		}
		delete(p.outbound[client].messages, id)
	}
	return true
}

func (p *MemoryPersistence) GetAll(client string) (messages []ControlPacket) {
	//only need to get a read lock on the persistence store, but lock the underlying
	//persistenceentry for the client we're working with.
	p.RLock()
	p.outbound[client].Lock()
	defer p.outbound[client].Unlock()
	defer p.RUnlock()
	//Get every message in the persistence store for a given client, create a slice
	//of the ControlPackets (not just PUBLISHES in there)
	for _, message := range p.outbound[client].messages {
		messages = append(messages, message)
	}
	return messages
}

func (p *MemoryPersistence) Exists(client string) bool {
	//grab a read lock on the persistence and check if there is already an entry
	//for this client.
	p.RLock()
	defer p.RUnlock()
	_, okInbound := p.inbound[client]
	_, okOutbound := p.outbound[client]
	return okInbound && okOutbound
}

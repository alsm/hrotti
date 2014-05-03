package hrotti

import (
	"sync"
)

// A map of clientid to Client pointer and a RW Mutex to protect access.
type Clients struct {
	sync.RWMutex
	list map[string]*Client
}

// Return empty Clients (value type)
func NewClients() Clients {
	c := Clients{
		sync.RWMutex{},
		make(map[string]*Client),
	}
	return c
}

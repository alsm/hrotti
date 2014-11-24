package hrotti

import (
	"sync"
)

// A map of clientid to Client pointer and a RW Mutex to protect access.
type clients struct {
	sync.RWMutex
	list map[string]*Client
}

// Return empty Clients (value type)
func newClients() *clients {
	c := &clients{
		sync.RWMutex{},
		make(map[string]*Client),
	}
	return c
}

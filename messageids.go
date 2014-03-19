package main

import (
	"sync"
)

type msgId uint16

type MessageIds struct {
	sync.RWMutex
	idChan chan msgId
	index  map[msgId]bool
}

const (
	msgId_MAX msgId = 65535
	msgId_MIN msgId = 1
)

func (m *MessageIds) genMsgIds() {
	m.idChan = make(chan msgId, 10)
	go func() {
		for {
			m.Lock()
			for i := msgId_MIN; i < msgId_MAX; i++ {
				if !m.index[i] {
					m.index[i] = true
					m.Unlock()
					m.idChan <- i
					break
				}
			}
		}
	}()
}

func (m *MessageIds) inUse(id msgId) bool {
	m.RLock()
	defer m.RUnlock()
	return m[id]
}

func (m *MessageIds) freeId(id msgId) {
	defer m.Unlock()
	m.Lock()
	m.index[id] = false
}

func (m *MessageIds) getId() msgId {
	return <-m.idChan
}

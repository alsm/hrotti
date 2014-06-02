package hrotti

import (
	"sync"
)

type msgID uint32

type MessageIDs struct {
	sync.RWMutex
	idChan chan msgID
	index  map[msgID]bool
}

const (
	msgIDMax msgID = 65535
	msgIDMin msgID = 1
)

func (c *Client) genMsgIDs() {
	defer c.Done()
	m := &c.MessageIDs
	for {
		m.Lock()
		for i := msgIDMin; i < msgIDMax; i++ {
			if !m.index[i] {
				m.index[i] = true
				m.Unlock()
				select {
				case m.idChan <- i:
				case <-c.stop:
					return
				}
				break
			}
		}
	}
}

func (m *MessageIDs) inUse(id msgID) bool {
	m.RLock()
	defer m.RUnlock()
	return m.index[id]
}

func (m *MessageIDs) freeID(id msgID) {
	defer m.Unlock()
	m.Lock()
	m.index[id] = false
}

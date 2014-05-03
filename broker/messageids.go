package hrotti

import (
	"sync"
)

type msgId uint32

type MessageIds struct {
	sync.RWMutex
	idChan chan msgId
	index  map[msgId]bool
}

const (
	msgIdMax msgId = 65535
	msgIdMin msgId = 1
)

func (c *Client) genMsgIds() {
	defer c.Done()
	m := &c.MessageIds
	for {
		m.Lock()
		for i := msgIdMin; i < msgIdMax; i++ {
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

func (m *MessageIds) inUse(id msgId) bool {
	m.RLock()
	defer m.RUnlock()
	return m.index[id]
}

func (m *MessageIds) freeId(id msgId) {
	defer m.Unlock()
	m.Lock()
	m.index[id] = false
}

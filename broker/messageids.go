package hrotti

import (
	"github.com/google/uuid"
	"sync"
)

type messageIDs struct {
	sync.RWMutex
	//idChan chan uint16
	index map[uint16]*uuid.UUID
}

const (
	msgIDMax uint16 = 65535
	msgIDMin uint16 = 1
)

/*func (c *Client) genMsgIDs() {
	defer c.Done()
	m := &c.messageIDs
	for {
		m.Lock()
		for i := msgIDMin; i < msgIDMax; i++ {
			if m.index[i] == nil {
				m.index[i] =
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
}*/

func (m *messageIDs) getMsgID(id uuid.UUID) uint16 {
	m.Lock()
	defer m.Unlock()
	for i := msgIDMin; i < msgIDMax; i++ {
		if m.index[i] == nil {
			m.index[i] = &id
			return i
		}
	}
	return 0
}

func (m *messageIDs) inUse(id uint16) bool {
	m.RLock()
	defer m.RUnlock()
	return m.index[id] != nil
}

func (m *messageIDs) freeID(id uint16) {
	m.Lock()
	defer m.Unlock()
	m.index[id] = nil
}

package hrotti

import (
	"sync"
)

type internalIds struct {
	sync.RWMutex
	idChan chan msgId
	index  map[msgId]bool
}

var internalMsgIds internalIds

//internal message ids are between 65535 and 2147483648
const (
	internalIdMin msgId = 65536
	internalIdMax msgId = 2147483648
)

func genInternalIds() {
	//setup the channel and the map, set m as a pointer to internalIds, if you don't use
	//& you get a copy of internalIds, and no one wants that.
	internalMsgIds.idChan = make(chan msgId, 10)
	internalMsgIds.index = make(map[msgId]bool)
	m := &internalMsgIds
	go func() {
		for {
			m.Lock()
			for i := internalIdMin; i < internalIdMax; i++ {
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

func internalIdInUse(id msgId) bool {
	m := &internalMsgIds
	m.RLock()
	defer m.RUnlock()
	return m.index[id]
}

func freeInternalId(id msgId) {
	m := &internalMsgIds
	defer m.Unlock()
	m.Lock()
	m.index[id] = false
}

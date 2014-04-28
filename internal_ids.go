package main

import (
	"sync"
)

type internalIds struct {
	sync.RWMutex
	idChan chan msgId
	index  map[msgId]bool
}

var internalMsgIds internalIds

const (
	internalIdMax msgId = 2147483648
	internalIdMin msgId = 65536
)

func genInternalIds() {
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

package main

import (
	"sync"
)

type msgId uint16

type MessageIds struct {
	sync.Mutex
	idChan chan msgId
	index  map[msgId]bool
}

const (
	msgId_MAX msgId = 65535
	msgId_MIN msgId = 1
)

func (mids *MessageIds) genMsgIds() {
	mids.idChan = make(chan msgId, 10)
	go func() {
		for {
			mids.Lock()
			for i := msgId_MIN; i < msgId_MAX; i++ {
				if !mids.index[i] {
					mids.index[i] = true
					mids.Unlock()
					mids.idChan <- i
					break
				}
			}
		}
	}()
}

func (mids *MessageIds) freeId(id msgId) {
	defer mids.Unlock()
	mids.Lock()
	mids.index[id] = false
}

func (mids *MessageIds) getId() msgId {
	return <-mids.idChan
}

package hrotti

import (
	"sync"
)

type internalIDs struct {
	sync.RWMutex
	idChan chan msgID
	index  map[msgID]bool
}

//internal message ids are between 65535 and 2147483648
const (
	internalIDMin msgID = 65536
	internalIDMax msgID = 2147483648
)

func (i *internalIDs) generateIDs() {
	//setup the channel and the map, set m as a pointer to internalIds, if you don't use
	//& you get a copy of internalIds, and no one wants that.
	i.idChan = make(chan msgID, 10)
	i.index = make(map[msgID]bool)
	go func() {
		for {
			i.Lock()
			for j := internalIDMin; j < internalIDMax; j++ {
				if !i.index[j] {
					i.index[j] = true
					i.Unlock()
					i.idChan <- j
					break
				}
			}
		}
	}()
}

func (i *internalIDs) idInUse(id msgID) bool {
	i.RLock()
	defer i.RUnlock()
	return i.index[id]
}

func (i *internalIDs) freeID(id msgID) {
	defer i.Unlock()
	i.Lock()
	i.index[id] = false
}

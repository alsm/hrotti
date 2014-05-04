package hrotti

import (
	"sync"
)

type internalIds struct {
	sync.RWMutex
	idChan chan msgId
	index  map[msgId]bool
}

//internal message ids are between 65535 and 2147483648
const (
	internalIdMin msgId = 65536
	internalIdMax msgId = 2147483648
)

func (i *internalIds) generateIds() {
	//setup the channel and the map, set m as a pointer to internalIds, if you don't use
	//& you get a copy of internalIds, and no one wants that.
	i.idChan = make(chan msgId, 10)
	i.index = make(map[msgId]bool)
	go func() {
		for {
			i.Lock()
			for j := internalIdMin; j < internalIdMax; j++ {
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

func (i *internalIds) idInUse(id msgId) bool {
	i.RLock()
	defer i.RUnlock()
	return i.index[id]
}

func (i *internalIds) freeId(id msgId) {
	defer i.Unlock()
	i.Lock()
	i.index[id] = false
}

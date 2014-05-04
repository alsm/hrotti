package hrotti

import (
	"sync"
)

type State struct {
	sync.RWMutex
	value StateVal
}

type StateVal uint8

const (
	DISCONNECTED  StateVal = 0x00
	CONNECTING    StateVal = 0x01
	CONNECTED     StateVal = 0x02
	DISCONNECTING StateVal = 0x03
)

func (s *State) SetValue(value StateVal) {
	s.Lock()
	defer s.Unlock()
	s.value = value
}

func (s *State) Value() StateVal {
	s.RLock()
	defer s.RUnlock()
	return s.value
}

package hrotti

import (
	"code.google.com/p/go-uuid/uuid"
	. "github.com/alsm/hrotti/packets"
)

type dirFlag byte

const (
	INBOUND  = 1
	OUTBOUND = 2
)

type Persistence interface {
	Init() error
	Open(string)
	Close(string)
	Add(string, dirFlag, ControlPacket) bool
	Replace(string, dirFlag, ControlPacket) bool
	AddBatch(map[string]*PublishPacket)
	Delete(string, dirFlag, uuid.UUID) bool
	GetAll(string) []ControlPacket
	Exists(string) bool
}

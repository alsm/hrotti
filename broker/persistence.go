package hrotti

import (
	. "github.com/alsm/hrotti/packets"
	"github.com/google/uuid"
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

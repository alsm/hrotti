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
	Open(*Client)
	Close(*Client)
	Add(*Client, dirFlag, ControlPacket) bool
	Replace(*Client, dirFlag, ControlPacket) bool
	AddBatch(map[*Client]*PublishPacket)
	Delete(*Client, dirFlag, uuid.UUID) bool
	GetAll(*Client) []ControlPacket
	Exists(*Client) bool
}

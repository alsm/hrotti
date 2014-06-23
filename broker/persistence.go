package hrotti

type dirFlag byte

const (
	INBOUND  = 1
	OUTBOUND = 2
)

type Persistence interface {
	Init() Persistence
	Open(*Client)
	Close(*Client)
	Add(*Client, dirFlag, ControlPacket) bool
	Replace(*Client, dirFlag, ControlPacket) bool
	AddBatch(map[*Client]*publishPacket)
	Delete(*Client, dirFlag, msgID) bool
	GetAll(*Client) []ControlPacket
	Exists(*Client) bool
}

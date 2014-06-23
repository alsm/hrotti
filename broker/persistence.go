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
	Add(*Client, dirFlag, controlPacket) bool
	Replace(*Client, dirFlag, controlPacket) bool
	AddBatch(map[*Client]*publishPacket)
	Delete(*Client, dirFlag, msgID) bool
	GetAll(*Client) []controlPacket
	Exists(*Client) bool
}

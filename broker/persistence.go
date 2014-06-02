package hrotti

type PersistenceBatchEntry struct {
	id      msgID
	message publishPacket
}

type Persistence interface {
	Open(*Client)
	Close(*Client)
	Add(*Client, ControlPacket) bool
	Replace(*Client, ControlPacket) bool
	AddBatch(map[*Client]*publishPacket)
	Delete(*Client, msgID) bool
	GetAll(*Client) []ControlPacket
	Exists(*Client) bool
}

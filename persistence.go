package main

type PersistenceBatchEntry struct {
	id      msgId
	message publishPacket
}

type Persistence interface {
	Open(*Client)
	Close(*Client)
	Add(*Client, msgId, ControlPacket) bool
	Replace(*Client, msgId, ControlPacket) bool
	AddBatch(map[*Client]*publishPacket)
	Delete(*Client, msgId) bool
	GetAll(*Client) []ControlPacket
	Exists(*Client) bool
}

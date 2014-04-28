package main

type PersistenceBatchEntry struct {
	id      msgId
	message publishPacket
}

type Persistence interface {
	Open(*Client)
	Close(*Client)
	Add(*Client, ControlPacket) bool
	Replace(*Client, ControlPacket) bool
	AddBatch(map[*Client]*publishPacket)
	Delete(*Client, msgId) bool
	GetAll(*Client) []ControlPacket
	Exists(*Client) bool
}

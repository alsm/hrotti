package packets

import (
	"bytes"
	"fmt"
	"io"
)

//SUBSCRIBE packet

type SubscribePacket struct {
	FixedHeader
	MessageID uint16
	Topics    []string
	Qoss      []byte
}

func (s *SubscribePacket) String() string {
	str := fmt.Sprintf("%s\n", s.FixedHeader)
	str += fmt.Sprintf("MessageID: %d topics: %s", s.MessageID)
	return str
}

func (s *SubscribePacket) Write(w io.Writer) {
	var body bytes.Buffer

	body.Write(encodeUint16(s.MessageID))
	for i, topic := range s.Topics {
		body.Write(encodeUint16(uint16(len(topic))))
		body.Write(encodeString(topic))
		body.WriteByte(s.Qoss[i])
	}
	s.FixedHeader.RemainingLength = body.Len()
	header := s.FixedHeader.pack()

	w.Write(header.Bytes())
	w.Write(body.Bytes())
}

func (s *SubscribePacket) Unpack(b *bytes.Buffer) {
	s.MessageID = decodeUint16(b)
	var topic string
	for topic = decodeString(b); topic != ""; topic = decodeString(b) {
		s.Topics = append(s.Topics, topic)
		qos, _ := b.ReadByte()
		s.Qoss = append(s.Qoss, qos)
	}
}

func (s *SubscribePacket) MsgID() uint16 {
	return s.MessageID
}

func (s *SubscribePacket) SetMsgID(id uint16) {
	s.MessageID = id
}

func (s *SubscribePacket) PacketType() uint8 {
	return SUBSCRIBE
}

func (s *SubscribePacket) RequiresMsgID() bool {
	return true
}

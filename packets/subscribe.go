package packets

import (
	"bytes"
	"code.google.com/p/go-uuid/uuid"
	"fmt"
	"io"
)

//SUBSCRIBE packet

type SubscribePacket struct {
	FixedHeader
	MessageID uint16
	Topics    []string
	Qoss      []byte
	UUID      uuid.UUID
}

func (s *SubscribePacket) String() string {
	str := fmt.Sprintf("%s\n", s.FixedHeader)
	str += fmt.Sprintf("MessageID: %d topics: %s", s.MessageID, s.Topics)
	return str
}

func (s *SubscribePacket) Write(w io.Writer) error {
	var body bytes.Buffer
	var err error

	body.Write(encodeUint16(s.MessageID))
	for i, topic := range s.Topics {
		body.Write(encodeString(topic))
		body.WriteByte(s.Qoss[i])
	}
	s.FixedHeader.RemainingLength = body.Len()
	header := s.FixedHeader.pack()

	_, err = w.Write(header.Bytes())
	_, err = w.Write(body.Bytes())

	return err
}

func (s *SubscribePacket) Unpack(b io.Reader) {
	s.MessageID = decodeUint16(b)
	payloadLength := s.FixedHeader.RemainingLength - 2
	for payloadLength > 0 {
		topic := decodeString(b)
		s.Topics = append(s.Topics, topic)
		qos := decodeByte(b)
		s.Qoss = append(s.Qoss, qos)
		payloadLength -= len(topic) + 1
	}
}

func (s *SubscribePacket) Details() Details {
	return Details{Qos: 1, MessageID: s.MessageID}
}

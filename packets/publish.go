package packets

import (
	"bytes"
	"code.google.com/p/go-uuid/uuid"
	"fmt"
	"io"
)

//PUBLISH packet

type PublishPacket struct {
	FixedHeader
	TopicName string
	MessageID uint16
	Payload   []byte
	UUID      uuid.UUID
}

func (p *PublishPacket) String() string {
	str := fmt.Sprintf("%s\n", p.FixedHeader)
	str += fmt.Sprintf("topicName: %s MessageID: %d\n", p.TopicName, p.MessageID)
	str += fmt.Sprintf("payload: %s\n", string(p.Payload))
	return str
}

func (p *PublishPacket) Write(w io.Writer) error {
	var body bytes.Buffer
	var err error

	body.Write(encodeString(p.TopicName))
	if p.Qos > 0 {
		body.Write(encodeUint16(p.MessageID))
	}
	p.FixedHeader.RemainingLength = body.Len() + len(p.Payload)
	header := p.FixedHeader.pack()

	_, err = w.Write(header.Bytes())
	_, err = w.Write(body.Bytes())
	_, err = w.Write(p.Payload)

	return err
}

func (p *PublishPacket) Unpack(b io.Reader) {
	var payloadLength = p.FixedHeader.RemainingLength
	p.TopicName = decodeString(b)
	if p.Qos > 0 {
		p.MessageID = decodeUint16(b)
		payloadLength -= len(p.TopicName) + 4
	} else {
		payloadLength -= len(p.TopicName) + 2
	}
	p.Payload = make([]byte, payloadLength)
	b.Read(p.Payload)
}

func (p *PublishPacket) Copy() *PublishPacket {
	newP := NewControlPacket(PUBLISH).(*PublishPacket)
	newP.TopicName = p.TopicName
	newP.Payload = p.Payload

	return newP
}

func (p *PublishPacket) Details() Details {
	return Details{Qos: p.Qos, MessageID: p.MessageID}
}

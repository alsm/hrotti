package packets

import (
	"bytes"
	"fmt"
	"io"
)

//PUBLISH packet

type PublishPacket struct {
	FixedHeader
	TopicName string
	MessageID uint16
	Payload   []byte
}

func (p *PublishPacket) String() string {
	str := fmt.Sprintf("%s\n", p.FixedHeader)
	str += fmt.Sprintf("topicName: %s MessageID: %d\n", p.TopicName, p.MessageID)
	str += fmt.Sprintf("payload: %s\n", string(p.Payload))
	return str
}

func (p *PublishPacket) Write(w io.Writer) {
	var body bytes.Buffer

	body.Write(encodeString(p.TopicName))
	if p.Qos > 0 {
		body.Write(encodeUint16(p.MessageID))
	}
	p.FixedHeader.RemainingLength = body.Len() + len(p.Payload)
	header := p.FixedHeader.pack()

	w.Write(header.Bytes())
	w.Write(body.Bytes())
	w.Write(p.Payload)

}

func (p *PublishPacket) Unpack(b *bytes.Buffer) {
	p.TopicName = decodeString(b)
	if p.Qos > 0 {
		p.MessageID = decodeUint16(b)
		p.Payload = b.Bytes()
	} else {
		p.Payload = b.Bytes()
	}
}

func (p *PublishPacket) Copy() *PublishPacket {
	newP := NewControlPacket(PUBLISH).(*PublishPacket)
	newP.TopicName = p.TopicName
	newP.Payload = p.Payload

	return newP
}

func (p *PublishPacket) MsgID() uint16 {
	return p.MessageID
}

func (p *PublishPacket) SetMsgID(id uint16) {
	p.MessageID = id
}

func (p *PublishPacket) PacketType() uint8 {
	return PUBLISH
}

func (p *PublishPacket) RequiresMsgID() bool {
	if p.Qos > 0 {
		return true
	}
	return false
}

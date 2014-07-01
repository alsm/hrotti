package packets

import (
	"bytes"
	"fmt"
	"io"
)

//UNSUBSCRIBE packet

type UnsubscribePacket struct {
	FixedHeader
	MessageID uint16
	Topics    []string
}

func (u *UnsubscribePacket) String() string {
	str := fmt.Sprintf("%s\n", u.FixedHeader)
	str += fmt.Sprintf("MessageID: %d", u.MessageID)
	return str
}

func (u *UnsubscribePacket) Write(w io.Writer) {
	var body bytes.Buffer
	body.Write(encodeUint16(u.MessageID))
	for _, topic := range u.Topics {
		body.Write(encodeString(topic))
	}
	u.FixedHeader.RemainingLength = body.Len()
	header := u.FixedHeader.pack()

	w.Write(header.Bytes())
	w.Write(body.Bytes())
}

func (u *UnsubscribePacket) Unpack(b *bytes.Buffer) {
	u.MessageID = decodeUint16(b)
	var topic string
	for topic = decodeString(b); topic != ""; topic = decodeString(b) {
		u.Topics = append(u.Topics, topic)
	}
}

func (u *UnsubscribePacket) MsgID() uint16 {
	return u.MessageID
}

func (u *UnsubscribePacket) SetMsgID(id uint16) {
	u.MessageID = id
}

func (u *UnsubscribePacket) PacketType() uint8 {
	return UNSUBSCRIBE
}

func (u *UnsubscribePacket) RequiresMsgID() bool {
	return true
}

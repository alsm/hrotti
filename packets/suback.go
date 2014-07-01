package packets

import (
	"bytes"
	"fmt"
	"io"
)

//SUBACK packet

type SubackPacket struct {
	FixedHeader
	MessageID   uint16
	GrantedQoss []byte
}

func (sa *SubackPacket) String() string {
	str := fmt.Sprintf("%s\n", sa.FixedHeader)
	str += fmt.Sprintf("MessageID: %d", sa.MessageID)
	return str
}

func (sa *SubackPacket) Write(w io.Writer) {
	var body bytes.Buffer
	body.Write(encodeUint16(sa.MessageID))
	body.Write(sa.GrantedQoss)
	sa.FixedHeader.RemainingLength = body.Len()
	header := sa.FixedHeader.pack()

	w.Write(header.Bytes())
	w.Write(body.Bytes())
}

func (sa *SubackPacket) Unpack(b *bytes.Buffer) {
	sa.MessageID = decodeUint16(b)
}

func (sa *SubackPacket) MsgID() uint16 {
	return sa.MessageID
}

func (sa *SubackPacket) SetMsgID(id uint16) {
	sa.MessageID = id
}

func (sa *SubackPacket) PacketType() uint8 {
	return SUBACK
}

func (sa *SubackPacket) RequiresMsgID() bool {
	return true
}

package packets

import (
	"bytes"
	"fmt"
	"io"
)

//UNSUBACK packet

type UnsubackPacket struct {
	FixedHeader
	MessageID uint16
}

func (ua *UnsubackPacket) String() string {
	str := fmt.Sprintf("%s\n", ua.FixedHeader)
	str += fmt.Sprintf("MessageID: %d", ua.MessageID)
	return str
}

func (ua *UnsubackPacket) Write(w io.Writer) {
	ua.FixedHeader.RemainingLength = 2
	header := ua.FixedHeader.pack()
	w.Write(header.Bytes())
	w.Write(encodeUint16(ua.MessageID))
}

func (ua *UnsubackPacket) Unpack(b *bytes.Buffer) {
	ua.MessageID = decodeUint16(b)
}

func (ua *UnsubackPacket) MsgID() uint16 {
	return ua.MessageID
}

func (ua *UnsubackPacket) SetMsgID(id uint16) {
	ua.MessageID = id
}

func (ua *UnsubackPacket) PacketType() uint8 {
	return UNSUBACK
}

func (ua *UnsubackPacket) RequiresMsgID() bool {
	return true
}

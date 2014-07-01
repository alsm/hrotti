package packets

import (
	"bytes"
	"fmt"
	"io"
)

//PUBACK packet

type PubackPacket struct {
	FixedHeader
	MessageID uint16
}

func (pa *PubackPacket) String() string {
	str := fmt.Sprintf("%s\n", pa.FixedHeader)
	str += fmt.Sprintf("messageID: %d", pa.MessageID)
	return str
}

func (pa *PubackPacket) Write(w io.Writer) {
	pa.FixedHeader.RemainingLength = 2
	header := pa.FixedHeader.pack()
	w.Write(header.Bytes())
	w.Write(encodeUint16(pa.MessageID))
}

func (pa *PubackPacket) Unpack(b *bytes.Buffer) {
	pa.MessageID = decodeUint16(b)
}

func (pa *PubackPacket) MsgID() uint16 {
	return pa.MessageID
}

func (pa *PubackPacket) SetMsgID(id uint16) {
	pa.MessageID = id
}

func (pa *PubackPacket) PacketType() uint8 {
	return PUBACK
}

func (pa *PubackPacket) RequiresMsgID() bool {
	return true
}

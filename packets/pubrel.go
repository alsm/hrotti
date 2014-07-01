package packets

import (
	"bytes"
	"fmt"
	"io"
)

//PUBREL packet

type PubrelPacket struct {
	FixedHeader
	MessageID uint16
}

func (pr *PubrelPacket) String() string {
	str := fmt.Sprintf("%s\n", pr.FixedHeader)
	str += fmt.Sprintf("MessageID: %d", pr.MessageID)
	return str
}

func (pr *PubrelPacket) Write(w io.Writer) {
	pr.FixedHeader.RemainingLength = 2
	header := pr.FixedHeader.pack()
	w.Write(header.Bytes())
	w.Write(encodeUint16(pr.MessageID))
}

func (pr *PubrelPacket) Unpack(b *bytes.Buffer) {
	pr.MessageID = decodeUint16(b)
}

func (pr *PubrelPacket) MsgID() uint16 {
	return pr.MessageID
}

func (pr *PubrelPacket) SetMsgID(id uint16) {
	pr.MessageID = id
}

func (pr *PubrelPacket) PacketType() uint8 {
	return PUBREL
}

func (pr *PubrelPacket) RequiresMsgID() bool {
	return true
}

package packets

import (
	"bytes"
	"fmt"
	"io"
)

//PUBREC packet

type PubrecPacket struct {
	FixedHeader
	MessageID uint16
}

func (pr *PubrecPacket) String() string {
	str := fmt.Sprintf("%s\n", pr.FixedHeader)
	str += fmt.Sprintf("MessageID: %d", pr.MessageID)
	return str
}

func (pr *PubrecPacket) Write(w io.Writer) {
	pr.FixedHeader.RemainingLength = 2
	header := pr.FixedHeader.pack()
	w.Write(header.Bytes())
	w.Write(encodeUint16(pr.MessageID))
}

func (pr *PubrecPacket) Unpack(b *bytes.Buffer) {
	pr.MessageID = decodeUint16(b)
}

func (pr *PubrecPacket) MsgID() uint16 {
	return pr.MessageID
}

func (pr *PubrecPacket) SetMsgID(id uint16) {
	pr.MessageID = id
}

func (pr *PubrecPacket) PacketType() uint8 {
	return PUBREC
}

func (pr *PubrecPacket) RequiresMsgID() bool {
	return true
}

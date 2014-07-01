package packets

import (
	"bytes"
	"fmt"
	"io"
)

//PUBCOMP packet

type PubcompPacket struct {
	FixedHeader
	MessageID uint16
}

func (pc *PubcompPacket) String() string {
	str := fmt.Sprintf("%s\n", pc.FixedHeader)
	str += fmt.Sprintf("MessageID: %d", pc.MessageID)
	return str
}

func (pc *PubcompPacket) Write(w io.Writer) {
	pc.FixedHeader.RemainingLength = 2
	header := pc.FixedHeader.pack()
	w.Write(header.Bytes())
	w.Write(encodeUint16(pc.MessageID))
}

func (pc *PubcompPacket) Unpack(b *bytes.Buffer) {
	pc.MessageID = decodeUint16(b)
}

func (pc *PubcompPacket) MsgID() uint16 {
	return pc.MessageID
}

func (pc *PubcompPacket) SetMsgID(id uint16) {
	pc.MessageID = id
}

func (pc *PubcompPacket) PacketType() uint8 {
	return PUBCOMP
}

func (pc *PubcompPacket) RequiresMsgID() bool {
	return true
}

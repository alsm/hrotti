package packets

import (
	"bytes"
	"fmt"
	"io"
)

//PINGREQ packet

type PingreqPacket struct {
	FixedHeader
}

func (pr *PingreqPacket) String() string {
	str := fmt.Sprintf("%s", pr.FixedHeader)
	return str
}

func (pr *PingreqPacket) Write(w io.Writer) {
	header := pr.FixedHeader.pack()
	w.Write(header.Bytes())
}

func (pr *PingreqPacket) Unpack(b *bytes.Buffer) {
}

func (pr *PingreqPacket) MsgID() uint16 {
	return 0
}

func (pr *PingreqPacket) SetMsgID(id uint16) {
}

func (pr *PingreqPacket) PacketType() uint8 {
	return PINGREQ
}

func (pr *PingreqPacket) RequiresMsgID() bool {
	return false
}

package packets

import (
	"bytes"
	"fmt"
	"io"
)

//PINGRESP packet

type PingrespPacket struct {
	FixedHeader
}

func (pr *PingrespPacket) String() string {
	str := fmt.Sprintf("%s", pr.FixedHeader)
	return str
}

func (pr *PingrespPacket) Write(w io.Writer) {
	header := pr.FixedHeader.pack()
	w.Write(header.Bytes())
}

func (pr *PingrespPacket) Unpack(b *bytes.Buffer) {
}

func (pr *PingrespPacket) MsgID() uint16 {
	return 0
}

func (pr *PingrespPacket) SetMsgID(id uint16) {
}

func (pr *PingrespPacket) PacketType() uint8 {
	return PINGRESP
}

func (pr *PingrespPacket) RequiresMsgID() bool {
	return false
}

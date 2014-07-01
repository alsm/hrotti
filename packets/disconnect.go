package packets

import (
	"bytes"
	"fmt"
	"io"
)

//DISCONNECT packet

type DisconnectPacket struct {
	FixedHeader
}

func (d *DisconnectPacket) String() string {
	str := fmt.Sprintf("%s\n", d.FixedHeader)
	return str
}

func (d *DisconnectPacket) Write(w io.Writer) {
	header := d.FixedHeader.pack()
	w.Write(header.Bytes())
}

func (d *DisconnectPacket) Unpack(b *bytes.Buffer) {
}

func (d *DisconnectPacket) MsgID() uint16 {
	return 0
}

func (d *DisconnectPacket) SetMsgID(id uint16) {
}

func (d *DisconnectPacket) PacketType() uint8 {
	return DISCONNECT
}

func (d *DisconnectPacket) RequiresMsgID() bool {
	return false
}

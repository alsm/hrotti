package packets

import (
	"code.google.com/p/go-uuid/uuid"
	"fmt"
	"io"
)

//DISCONNECT packet

type DisconnectPacket struct {
	FixedHeader
	UUID uuid.UUID
}

func (d *DisconnectPacket) String() string {
	str := fmt.Sprintf("%s\n", d.FixedHeader)
	return str
}

func (d *DisconnectPacket) Write(w io.Writer) error {
	header := d.FixedHeader.pack()
	_, err := w.Write(header.Bytes())

	return err
}

func (d *DisconnectPacket) Unpack(b io.Reader) {
}

func (d *DisconnectPacket) Details() Details {
	return Details{Qos: 0, MessageID: 0}
}

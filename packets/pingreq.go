package packets

import (
	"code.google.com/p/go-uuid/uuid"
	"fmt"
	"io"
)

//PINGREQ packet

type PingreqPacket struct {
	FixedHeader
	UUID uuid.UUID
}

func (pr *PingreqPacket) String() string {
	str := fmt.Sprintf("%s", pr.FixedHeader)
	return str
}

func (pr *PingreqPacket) Write(w io.Writer) error {
	header := pr.FixedHeader.pack()
	_, err := w.Write(header.Bytes())

	return err
}

func (pr *PingreqPacket) Unpack(b io.Reader) {
}

func (pr *PingreqPacket) Details() Details {
	return Details{Qos: 0, MessageID: 0}
}

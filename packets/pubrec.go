package packets

import (
	"code.google.com/p/go-uuid/uuid"
	"fmt"
	"io"
)

//PUBREC packet

type PubrecPacket struct {
	FixedHeader
	MessageID uint16
	UUID      uuid.UUID
}

func (pr *PubrecPacket) String() string {
	str := fmt.Sprintf("%s\n", pr.FixedHeader)
	str += fmt.Sprintf("MessageID: %d", pr.MessageID)
	return str
}

func (pr *PubrecPacket) Write(w io.Writer) error {
	var err error
	pr.FixedHeader.RemainingLength = 2
	header := pr.FixedHeader.pack()
	_, err = w.Write(header.Bytes())
	_, err = w.Write(encodeUint16(pr.MessageID))

	return err
}

func (pr *PubrecPacket) Unpack(b io.Reader) {
	pr.MessageID = decodeUint16(b)
}

func (pr *PubrecPacket) Details() Details {
	return Details{Qos: pr.Qos, MessageID: pr.MessageID}
}

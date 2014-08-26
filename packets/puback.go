package packets

import (
	"code.google.com/p/go-uuid/uuid"
	"fmt"
	"io"
)

//PUBACK packet

type PubackPacket struct {
	FixedHeader
	MessageID uint16
	UUID      uuid.UUID
}

func (pa *PubackPacket) String() string {
	str := fmt.Sprintf("%s\n", pa.FixedHeader)
	str += fmt.Sprintf("messageID: %d", pa.MessageID)
	return str
}

func (pa *PubackPacket) Write(w io.Writer) error {
	var err error
	pa.FixedHeader.RemainingLength = 2
	header := pa.FixedHeader.pack()
	_, err = w.Write(header.Bytes())
	_, err = w.Write(encodeUint16(pa.MessageID))

	return err
}

func (pa *PubackPacket) Unpack(b io.Reader) {
	pa.MessageID = decodeUint16(b)
}

func (pa *PubackPacket) Details() Details {
	return Details{Qos: pa.Qos, MessageID: pa.MessageID}
}

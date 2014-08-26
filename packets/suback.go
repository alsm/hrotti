package packets

import (
	"bytes"
	"code.google.com/p/go-uuid/uuid"
	"fmt"
	"io"
)

//SUBACK packet

type SubackPacket struct {
	FixedHeader
	MessageID   uint16
	GrantedQoss []byte
	UUID        uuid.UUID
}

func (sa *SubackPacket) String() string {
	str := fmt.Sprintf("%s\n", sa.FixedHeader)
	str += fmt.Sprintf("MessageID: %d", sa.MessageID)
	return str
}

func (sa *SubackPacket) Write(w io.Writer) error {
	var body bytes.Buffer
	var err error
	body.Write(encodeUint16(sa.MessageID))
	body.Write(sa.GrantedQoss)
	sa.FixedHeader.RemainingLength = body.Len()
	header := sa.FixedHeader.pack()

	_, err = w.Write(header.Bytes())
	_, err = w.Write(body.Bytes())

	return err
}

func (sa *SubackPacket) Unpack(b io.Reader) {
	sa.MessageID = decodeUint16(b)
}

func (sa *SubackPacket) Details() Details {
	return Details{Qos: 0, MessageID: sa.MessageID}
}

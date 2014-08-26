package packets

import (
	"bytes"
	"code.google.com/p/go-uuid/uuid"
	"fmt"
	"io"
)

//CONNACK packet

type ConnackPacket struct {
	FixedHeader
	TopicNameCompression byte
	ReturnCode           byte
	UUID                 uuid.UUID
}

func (ca *ConnackPacket) String() string {
	str := fmt.Sprintf("%s\n", ca.FixedHeader)
	str += fmt.Sprintf("returncode: %d", ca.ReturnCode)
	return str
}

func (ca *ConnackPacket) Write(w io.Writer) error {
	var body bytes.Buffer
	var err error

	body.WriteByte(ca.TopicNameCompression)
	body.WriteByte(ca.ReturnCode)
	ca.FixedHeader.RemainingLength = 2
	header := ca.FixedHeader.pack()

	_, err = w.Write(header.Bytes())
	_, err = w.Write(body.Bytes())

	return err
}

func (ca *ConnackPacket) Unpack(b io.Reader) {
	ca.TopicNameCompression = decodeByte(b)
	ca.ReturnCode = decodeByte(b)
}

func (ca *ConnackPacket) Details() Details {
	return Details{Qos: 0, MessageID: 0}
}

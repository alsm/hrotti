package packets

import (
	"bytes"
	"fmt"
	"io"
)

//CONNACK packet

type ConnackPacket struct {
	FixedHeader
	TopicNameCompression byte
	ReturnCode           byte
}

func (ca *ConnackPacket) String() string {
	str := fmt.Sprintf("%s\n", ca.FixedHeader)
	str += fmt.Sprintf("returncode: %d", ca.ReturnCode)
	return str
}

func (ca *ConnackPacket) Write(w io.Writer) {
	var body bytes.Buffer

	body.WriteByte(ca.TopicNameCompression)
	body.WriteByte(ca.ReturnCode)
	ca.FixedHeader.RemainingLength = 2
	header := ca.FixedHeader.pack()

	w.Write(header.Bytes())
	w.Write(body.Bytes())
}

func (ca *ConnackPacket) Unpack(b *bytes.Buffer) {
	ca.TopicNameCompression, _ = b.ReadByte()
	ca.ReturnCode, _ = b.ReadByte()
}

func (ca *ConnackPacket) MsgID() uint16 {
	return 0
}

func (ca *ConnackPacket) SetMsgID(id uint16) {
}

func (ca *ConnackPacket) PacketType() uint8 {
	return CONNACK
}
func (ca *ConnackPacket) RequiresMsgID() bool {
	return false
}

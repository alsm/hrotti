package packets

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

type ControlPacket interface {
	Write(io.Writer)
	Unpack(*bytes.Buffer)
	PacketType() uint8
	SetMsgID(uint16)
	MsgID() uint16
	String() string
	RequiresMsgID() bool
	//Validate() bool
}

var PacketNames = map[uint8]string{
	1:  "CONNECT",
	2:  "CONNACK",
	3:  "PUBLISH",
	4:  "PUBACK",
	5:  "PUBREC",
	6:  "PUBREL",
	7:  "PUBCOMP",
	8:  "SUBSCRIBE",
	9:  "SUBACK",
	10: "UNSUBSCRIBE",
	11: "UNSUBACK",
	12: "PINGREQ",
	13: "PINGRESP",
	14: "DISCONNECT",
}

const (
	CONNECT     = 1
	CONNACK     = 2
	PUBLISH     = 3
	PUBACK      = 4
	PUBREC      = 5
	PUBREL      = 6
	PUBCOMP     = 7
	SUBSCRIBE   = 8
	SUBACK      = 9
	UNSUBSCRIBE = 10
	UNSUBACK    = 11
	PINGREQ     = 12
	PINGRESP    = 13
	DISCONNECT  = 14
)

const (
	CONN_ACCEPTED           = 0x00
	CONN_REF_BAD_PROTO_VER  = 0x01
	CONN_REF_ID_REJ         = 0x02
	CONN_REF_SERV_UNAVAIL   = 0x03
	CONN_REF_BAD_USER_PASS  = 0x04
	CONN_REF_NOT_AUTH       = 0x05
	CONN_PROTOCOL_VIOLATION = 0xFF
)

var ConnackReturnCodes = map[uint8]string{
	0:   "Connection Accepted",
	1:   "Connection Refused: Bad Protocol Version",
	2:   "Connection Refused: Client Identifier Rejected",
	3:   "Connection Refused: Server Unavailable",
	4:   "Connection Refused: Username or Password in unknown format",
	5:   "Connection Refused: Not Authorised",
	255: "Connection Refused: Protocol Violation",
}

func ReadPacket(r io.Reader) (cp ControlPacket, err error) {
	var fh FixedHeader
	var b [1]byte

	_, err = io.ReadFull(r, b[:])
	if err != nil {
		return nil, err
	}
	fh.unpack(b[0], r)
	cp = NewControlPacketWithHeader(fh)
	packetBytes := make([]byte, fh.RemainingLength)
	_, err = io.ReadFull(r, packetBytes)
	if err != nil {
		return nil, err
	}
	cp.Unpack(bytes.NewBuffer(packetBytes))
	return cp, nil
}

func NewControlPacket(packetType byte) ControlPacket {
	switch packetType {
	case CONNECT:
		return &ConnectPacket{FixedHeader: FixedHeader{MessageType: CONNECT}}
	case CONNACK:
		return &ConnackPacket{FixedHeader: FixedHeader{MessageType: CONNACK}}
	case DISCONNECT:
		return &DisconnectPacket{FixedHeader: FixedHeader{MessageType: DISCONNECT}}
	case PUBLISH:
		return &PublishPacket{FixedHeader: FixedHeader{MessageType: PUBLISH}}
	case PUBACK:
		return &PubackPacket{FixedHeader: FixedHeader{MessageType: PUBACK}}
	case PUBREC:
		return &PubrecPacket{FixedHeader: FixedHeader{MessageType: PUBREC}}
	case PUBREL:
		return &PubrelPacket{FixedHeader: FixedHeader{MessageType: PUBREL, Qos: 1}}
	case PUBCOMP:
		return &PubcompPacket{FixedHeader: FixedHeader{MessageType: PUBCOMP}}
	case SUBSCRIBE:
		return &SubscribePacket{FixedHeader: FixedHeader{MessageType: SUBSCRIBE, Qos: 1}}
	case SUBACK:
		return &SubackPacket{FixedHeader: FixedHeader{MessageType: SUBACK}}
	case UNSUBSCRIBE:
		return &UnsubscribePacket{FixedHeader: FixedHeader{MessageType: UNSUBSCRIBE}}
	case UNSUBACK:
		return &UnsubackPacket{FixedHeader: FixedHeader{MessageType: UNSUBACK}}
	case PINGREQ:
		return &PingreqPacket{FixedHeader: FixedHeader{MessageType: PINGREQ}}
	case PINGRESP:
		return &PingrespPacket{FixedHeader: FixedHeader{MessageType: PINGRESP}}
	default:
		break
	}
	return nil
}

func NewControlPacketWithHeader(fh FixedHeader) ControlPacket {
	switch fh.MessageType {
	case CONNECT:
		return &ConnectPacket{FixedHeader: fh}
	case CONNACK:
		return &ConnackPacket{FixedHeader: fh}
	case DISCONNECT:
		return &DisconnectPacket{FixedHeader: fh}
	case PUBLISH:
		return &PublishPacket{FixedHeader: fh}
	case PUBACK:
		return &PubackPacket{FixedHeader: fh}
	case PUBREC:
		return &PubrecPacket{FixedHeader: fh}
	case PUBREL:
		return &PubrelPacket{FixedHeader: fh}
	case PUBCOMP:
		return &PubcompPacket{FixedHeader: fh}
	case SUBSCRIBE:
		return &SubscribePacket{FixedHeader: fh}
	case SUBACK:
		return &SubackPacket{FixedHeader: fh}
	case UNSUBSCRIBE:
		return &UnsubscribePacket{FixedHeader: fh}
	case UNSUBACK:
		return &UnsubackPacket{FixedHeader: fh}
	case PINGREQ:
		return &PingreqPacket{FixedHeader: fh}
	case PINGRESP:
		return &PingrespPacket{FixedHeader: fh}
	default:
		break
	}
	return nil
}

type FixedHeader struct {
	MessageType     byte
	Dup             bool
	Qos             byte
	Retain          bool
	RemainingLength int
}

func (fh FixedHeader) String() string {
	return fmt.Sprintf("%s: dup: %d qos: %d retain: %d rLength: %d", PacketNames[fh.MessageType], fh.Dup, fh.Qos, fh.Retain, fh.RemainingLength)
}

func boolToByte(b bool) byte {
	switch b {
	case true:
		return 1
	default:
		return 0
	}
}

func (fh *FixedHeader) pack() bytes.Buffer {
	var header bytes.Buffer
	header.WriteByte(fh.MessageType<<4 | boolToByte(fh.Dup)<<3 | fh.Qos<<1 | boolToByte(fh.Retain))
	header.Write(encodeLength(fh.RemainingLength))
	return header
}

func (fh *FixedHeader) unpack(typeAndFlags byte, r io.Reader) {
	fh.MessageType = typeAndFlags >> 4
	fh.Dup = (typeAndFlags>>3)&0x01 > 0
	fh.Qos = (typeAndFlags >> 1) & 0x03
	fh.Retain = typeAndFlags&0x01 > 0
	fh.RemainingLength = decodeLength(r)
}

func encodeString(field string) []byte {
	fieldLength := make([]byte, 2)
	binary.BigEndian.PutUint16(fieldLength, uint16(len(field)))
	return append(fieldLength, []byte(field)...)
}

func encodeBytes(field []byte) []byte {
	fieldLength := make([]byte, 2)
	binary.BigEndian.PutUint16(fieldLength, uint16(len(field)))
	return append(fieldLength, field...)
}

func decodeUint16(b *bytes.Buffer) uint16 {
	num := make([]byte, 2)
	b.Read(num)
	return binary.BigEndian.Uint16(num)
}

func encodeUint16(num uint16) []byte {
	bytes := make([]byte, 2)
	binary.BigEndian.PutUint16(bytes, num)
	return bytes
}

func decodeString(b *bytes.Buffer) string {
	fieldLength := decodeUint16(b)
	field := make([]byte, fieldLength)
	b.Read(field)
	return string(field)
}

func decodeBytes(b *bytes.Buffer) []byte {
	fieldLength := decodeUint16(b)
	field := make([]byte, fieldLength)
	b.Read(field)
	return field
}

func encodeLength(length int) []byte {
	var encLength []byte
	for {
		digit := byte(length % 128)
		length /= 128
		if length > 0 {
			digit |= 0x80
		}
		encLength = append(encLength, digit)
		if length == 0 {
			break
		}
	}
	return encLength
}

func decodeLength(r io.Reader) int {
	var rLength uint32
	var multiplier uint32 = 0
	b := make([]byte, 1)
	for {
		io.ReadFull(r, b)
		digit := b[0]
		rLength |= uint32(digit&127) << multiplier
		if (digit & 128) == 0 {
			break
		}
		multiplier += 7
	}
	return int(rLength)
}

func packetType(mType byte) byte {
	return mType >> 4
}

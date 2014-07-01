package packets

import (
	"bytes"
	"fmt"
	"io"
)

//CONNECT packet

type ConnectPacket struct {
	FixedHeader
	ProtocolName    string
	ProtocolVersion byte
	CleanSession    byte
	WillFlag        byte
	WillQos         byte
	WillRetain      bool
	UsernameFlag    byte
	PasswordFlag    byte
	ReservedBit     byte
	KeepaliveTimer  uint16

	ClientIdentifier string
	WillTopic        string
	WillMessage      []byte
	Username         string
	Password         []byte
}

func (c *ConnectPacket) String() string {
	str := fmt.Sprintf("%s\n", c.FixedHeader)
	str += fmt.Sprintf("protocolversion: %d protocolname: %s cleansession: %d willflag: %d WillQos: %d WillRetain: %d Usernameflag: %d Passwordflag: %d keepalivetimer: %d\nclientId: %s\nwilltopic: %s\nwillmessage: %s\nUsername: %s\nPassword: %s\n", c.ProtocolVersion, c.ProtocolName, c.CleanSession, c.WillFlag, c.WillQos, c.WillRetain, c.UsernameFlag, c.PasswordFlag, c.KeepaliveTimer, c.ClientIdentifier, c.WillTopic, c.WillMessage, c.Username, c.Password)
	return str
}

func (c *ConnectPacket) Write(w io.Writer) {
	var header bytes.Buffer
	var body bytes.Buffer

	body.Write(encodeString(c.ProtocolName))
	body.WriteByte(c.ProtocolVersion)
	body.WriteByte(c.CleanSession<<1 | c.WillFlag<<2 | c.WillQos<<3 | boolToByte(c.WillRetain)<<5 | c.PasswordFlag<<6 | c.UsernameFlag<<7)
	body.Write(encodeUint16(c.KeepaliveTimer))
	body.Write(encodeString(c.ClientIdentifier))
	if c.WillFlag == 1 {
		body.Write(encodeString(c.WillTopic))
		body.Write(encodeBytes(c.WillMessage))
	}
	if c.UsernameFlag == 1 {
		body.Write(encodeString(c.Username))
	}
	if c.PasswordFlag == 1 {
		body.Write(encodeBytes(c.Password))
	}
	c.FixedHeader.RemainingLength = body.Len()
	header = c.FixedHeader.pack()

	w.Write(header.Bytes())
	w.Write(body.Bytes())
}

func (c *ConnectPacket) Unpack(b *bytes.Buffer) {
	c.ProtocolName = decodeString(b)
	c.ProtocolVersion, _ = b.ReadByte()
	options, _ := b.ReadByte()
	c.ReservedBit = 1 & options
	c.CleanSession = 1 & (options >> 1)
	c.WillFlag = 1 & (options >> 2)
	c.WillQos = 3 & (options >> 3)
	c.WillRetain = 1&(options>>5) > 0
	c.PasswordFlag = 1 & (options >> 6)
	c.UsernameFlag = 1 & (options >> 7)
	c.KeepaliveTimer = decodeUint16(b)
	c.ClientIdentifier = decodeString(b)
	if c.WillFlag == 1 {
		c.WillTopic = decodeString(b)
		c.WillMessage = decodeBytes(b)
	}
	if c.UsernameFlag == 1 {
		c.Username = decodeString(b)
	}
	if c.PasswordFlag == 1 {
		c.Password = decodeBytes(b)
	}
}

func (c *ConnectPacket) Validate() byte {
	if c.PasswordFlag == 1 && c.UsernameFlag != 1 {
		return CONN_REF_BAD_USER_PASS
	}
	if c.ReservedBit != 0 {
		return CONN_PROTOCOL_VIOLATION
	}
	if (c.ProtocolName == "MQIsdp" && c.ProtocolVersion != 3) || (c.ProtocolName == "MQTT" && c.ProtocolVersion != 4) {
		return CONN_REF_BAD_PROTO_VER
	}
	if c.ProtocolName != "MQIsdp" && c.ProtocolName != "MQTT" {
		return CONN_PROTOCOL_VIOLATION
	}
	if len(c.ClientIdentifier) > 65535 || len(c.Username) > 65535 || len(c.Password) > 65535 {
		return CONN_PROTOCOL_VIOLATION
	}
	return CONN_ACCEPTED
}

func (c *ConnectPacket) MsgID() uint16 {
	return 0
}

func (c *ConnectPacket) SetMsgID(id uint16) {
}

func (c *ConnectPacket) PacketType() uint8 {
	return CONNECT
}

func (c *ConnectPacket) RequiresMsgID() bool {
	return false
}

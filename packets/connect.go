package packets

import (
	"bytes"
	"fmt"
	"github.com/google/uuid"
	"io"
)

//CONNECT packet

type ConnectPacket struct {
	FixedHeader
	ProtocolName    string
	ProtocolVersion byte
	CleanSession    bool
	WillFlag        bool
	WillQos         byte
	WillRetain      bool
	UsernameFlag    bool
	PasswordFlag    bool
	ReservedBit     byte
	KeepaliveTimer  uint16

	ClientIdentifier string
	WillTopic        string
	WillMessage      []byte
	Username         string
	Password         []byte
	uuid             uuid.UUID
}
//////////////////////修改//////////////////////////
var userName string
var password string
func  SetUserName(name string) {
	userName = name
}

func SetPassWord(pwd string) {
	password = pwd
}
//////////////////////修改//////////////////////////
func (c *ConnectPacket) String() string {
	str := fmt.Sprintf("%s\n", c.FixedHeader)
	str += fmt.Sprintf("protocolversion: %d protocolname: %s cleansession: %t willflag: %t WillQos: %d WillRetain: %t Usernameflag: %t Passwordflag: %t keepalivetimer: %d\nclientId: %s\nwilltopic: %s\nwillmessage: %s\nUsername: %s\nPassword: %s\n", c.ProtocolVersion, c.ProtocolName, c.CleanSession, c.WillFlag, c.WillQos, c.WillRetain, c.UsernameFlag, c.PasswordFlag, c.KeepaliveTimer, c.ClientIdentifier, c.WillTopic, c.WillMessage, c.Username, c.Password)
	return str
}

func (c *ConnectPacket) Write(w io.Writer) error {
	var body bytes.Buffer
	var err error

	body.Write(encodeString(c.ProtocolName))
	body.WriteByte(c.ProtocolVersion)
	body.WriteByte(boolToByte(c.CleanSession)<<1 | boolToByte(c.WillFlag)<<2 | c.WillQos<<3 | boolToByte(c.WillRetain)<<5 | boolToByte(c.PasswordFlag)<<6 | boolToByte(c.UsernameFlag)<<7)
	body.Write(encodeUint16(c.KeepaliveTimer))
	body.Write(encodeString(c.ClientIdentifier))
	if c.WillFlag {
		body.Write(encodeString(c.WillTopic))
		body.Write(encodeBytes(c.WillMessage))
	}
	if c.UsernameFlag {
		body.Write(encodeString(c.Username))
	}
	if c.PasswordFlag {
		body.Write(encodeBytes(c.Password))
	}
	c.FixedHeader.RemainingLength = body.Len()
	packet := c.FixedHeader.pack()
	packet.Write(body.Bytes())
	_, err = packet.WriteTo(w)

	return err
}

func (c *ConnectPacket) Unpack(b io.Reader) {
	c.ProtocolName = decodeString(b)
	c.ProtocolVersion = decodeByte(b)
	options := decodeByte(b)
	c.ReservedBit = 1 & options
	c.CleanSession = 1&(options>>1) > 0
	c.WillFlag = 1&(options>>2) > 0
	c.WillQos = 3 & (options >> 3)
	c.WillRetain = 1&(options>>5) > 0
	c.PasswordFlag = 1&(options>>6) > 0
	c.UsernameFlag = 1&(options>>7) > 0
	c.KeepaliveTimer = decodeUint16(b)
	c.ClientIdentifier = decodeString(b)
	if c.WillFlag {
		c.WillTopic = decodeString(b)
		c.WillMessage = decodeBytes(b)
	}
	if c.UsernameFlag {
		c.Username = decodeString(b)
	}
	if c.PasswordFlag {
		c.Password = decodeBytes(b)
	}
}

func (c *ConnectPacket) Validate() byte {
	//////////////////////修改//////////////////////////
	if string(c.Password) != userName || c.Username != password{
		return CONN_REF_BAD_USER_PASS
	}
	//////////////////////修改//////////////////////////
	if c.PasswordFlag && !c.UsernameFlag {
		return CONN_REF_BAD_USER_PASS
	}
	if c.ReservedBit != 0 {
		fmt.Println("Bad reserved bit")
		return CONN_PROTOCOL_VIOLATION
	}
	if (c.ProtocolName == "MQIsdp" && c.ProtocolVersion != 3) || (c.ProtocolName == "MQTT" && c.ProtocolVersion != 4) {
		return CONN_REF_BAD_PROTO_VER
	}
	if c.ProtocolName != "MQIsdp" && c.ProtocolName != "MQTT" {
		fmt.Println("Bad protocol name")
		return CONN_PROTOCOL_VIOLATION
	}
	if len(c.ClientIdentifier) > 65535 || len(c.Username) > 65535 || len(c.Password) > 65535 {
		fmt.Println("Bad size field")
		return CONN_PROTOCOL_VIOLATION
	}
	return CONN_ACCEPTED
}

func (c *ConnectPacket) Details() Details {
	return Details{Qos: 0, MessageID: 0}
}

func (c *ConnectPacket) UUID() uuid.UUID {
	return c.uuid
}

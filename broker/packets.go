package hrotti

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
)

type controlPacket interface {
	pack() []byte
	unpack([]byte)
	packetType() uint8
	setMsgID(msgID)
	msgID() msgID
	String() string
	//Validate() bool
}

var packetNames = map[uint8]string{
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

var connackReturnCodes = map[uint8]string{
	0:   "Connection Accepted",
	1:   "Connection Refused: Bad Protocol Version",
	2:   "Connection Refused: Client Identifier Rejected",
	3:   "Connection Refused: Server Unavailable",
	4:   "Connection Refused: Username or Password in unknown format",
	5:   "Connection Refused: Not Authorised",
	255: "Connection Refused: Protocol Violation",
}

func msgIDToBytes(messageID msgID) []byte {
	msgIDBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(msgIDBytes, uint16(messageID))
	return msgIDBytes
}

func bytesToMsgID(bytes []byte) msgID {
	return msgID(binary.BigEndian.Uint16(bytes))
}

func newControlPacket(packetType byte) controlPacket {
	switch packetType {
	case CONNECT:
		return &connectPacket{fixedHeader: fixedHeader{messageType: CONNECT}, protocolName: "MQIsdp", protocolVersion: 3}
	case CONNACK:
		return &connackPacket{fixedHeader: fixedHeader{messageType: CONNACK}}
	case DISCONNECT:
		return &disconnectPacket{fixedHeader: fixedHeader{messageType: DISCONNECT}}
	case PUBLISH:
		return &publishPacket{fixedHeader: fixedHeader{messageType: PUBLISH}}
	case PUBACK:
		return &pubackPacket{fixedHeader: fixedHeader{messageType: PUBACK}}
	case PUBREC:
		return &pubrecPacket{fixedHeader: fixedHeader{messageType: PUBREC}}
	case PUBREL:
		return &pubrelPacket{fixedHeader: fixedHeader{messageType: PUBREL, qos: 1}}
	case PUBCOMP:
		return &pubcompPacket{fixedHeader: fixedHeader{messageType: PUBCOMP}}
	case SUBSCRIBE:
		return &subscribePacket{fixedHeader: fixedHeader{messageType: SUBSCRIBE, qos: 1}}
	case SUBACK:
		return &subackPacket{fixedHeader: fixedHeader{messageType: SUBACK}}
	case UNSUBSCRIBE:
		return &unsubscribePacket{fixedHeader: fixedHeader{messageType: UNSUBSCRIBE}}
	case UNSUBACK:
		return &unsubackPacket{fixedHeader: fixedHeader{messageType: UNSUBACK}}
	case PINGREQ:
		return &pingreqPacket{fixedHeader: fixedHeader{messageType: PINGREQ}}
	case PINGRESP:
		return &pingrespPacket{fixedHeader: fixedHeader{messageType: PINGRESP}}
	default:
		break
	}
	return nil
}

type fixedHeader struct {
	messageType     byte
	dup             byte
	qos             byte
	retain          byte
	remainingLength uint32
	length          int
}

func (fh fixedHeader) String() string {
	return fmt.Sprintf("%s: dup: %d qos: %d retain: %d rLength: %d", packetNames[fh.messageType], fh.dup, fh.qos, fh.retain, fh.remainingLength)
}

func (fh *fixedHeader) pack(size uint32) []byte {
	var header bytes.Buffer
	header.WriteByte(fh.messageType<<4 | fh.dup<<3 | fh.qos<<1 | fh.retain)
	header.Write(encode(size))
	return header.Bytes()
}

func (fh *fixedHeader) unpack(header byte) {
	fh.messageType = header >> 4
	fh.dup = (header >> 3) & 0x01
	fh.qos = (header >> 1) & 0x03
	fh.retain = header & 0x01
}

func encodeField(field string) []byte {
	fieldLength := make([]byte, 2)
	binary.BigEndian.PutUint16(fieldLength, uint16(len(field)))
	return append(fieldLength, []byte(field)...)
}

func encodeByteField(field []byte) []byte {
	fieldLength := make([]byte, 2)
	binary.BigEndian.PutUint16(fieldLength, uint16(len(field)))
	return append(fieldLength, field...)
}

func decodeField(packet []byte) ([]byte, string, int) {
	if len(packet) == 0 {
		return packet, "", 0
	}
	fieldLength := binary.BigEndian.Uint16(packet[:2]) + 2
	return packet[fieldLength:], string(packet[2:fieldLength]), int(fieldLength)
}

func decodeByteField(packet []byte) ([]byte, []byte, int) {
	if len(packet) == 0 {
		return packet, nil, 0
	}
	fieldLength := binary.BigEndian.Uint16(packet[:2]) + 2
	return packet[fieldLength:], packet[2:fieldLength], int(fieldLength)
}

func encode(length uint32) []byte {
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

func decodeLength(src *bufio.ReadWriter) uint32 {
	var rLength uint32
	var count int
	var multiplier uint32 = 1
	var digit byte
	count = 1
	for {
		digit, _ = src.ReadByte()
		rLength += uint32(digit&127) * multiplier
		if (digit & 128) == 0 {
			break
		}
		multiplier *= 128
		count++
	}
	return rLength
}

func messagepacketType(mType byte) byte {
	return mType >> 4
}

//CONNECT packet

type connectPacket struct {
	fixedHeader
	protocolName    string
	protocolVersion byte
	cleanSession    byte
	willFlag        byte
	willQos         byte
	willRetain      byte
	usernameFlag    byte
	passwordFlag    byte
	reservedBit     byte
	keepaliveTimer  uint16

	clientIdentifier string
	willTopic        string
	willMessage      []byte
	username         string
	password         []byte
}

func (c *connectPacket) String() string {
	str := fmt.Sprintf("%s\n", c.fixedHeader)
	str += fmt.Sprintf("protocolversion: %d protocolname: %s cleansession: %d willflag: %d willQos: %d willRetain: %d usernameflag: %d passwordflag: %d keepalivetimer: %d\nclientId: %s\nwilltopic: %s\nwillmessage: %s\nusername: %s\npassword: %s\n", c.protocolVersion, c.protocolName, c.cleanSession, c.willFlag, c.willQos, c.willRetain, c.usernameFlag, c.passwordFlag, c.keepaliveTimer, c.clientIdentifier, c.willTopic, c.willMessage, c.username, c.password)
	return str
}

func (c *connectPacket) pack() []byte {
	var body []byte
	keepalive := make([]byte, 2)
	binary.BigEndian.PutUint16(keepalive, c.keepaliveTimer)
	body = append(body, encodeField(c.protocolName)...)
	body = append(body, c.protocolVersion)
	body = append(body, (c.cleanSession<<1 | c.willFlag<<2 | c.willQos<<3 | c.willRetain<<5 | c.passwordFlag<<6 | c.usernameFlag<<7))
	body = append(body, keepalive...)
	body = append(body, encodeField(c.clientIdentifier)...)
	if c.willFlag == 1 {
		body = append(body, encodeField(c.willTopic)...)
		body = append(body, encodeByteField(c.willMessage)...)
	}
	if c.usernameFlag == 1 {
		body = append(body, encodeField(c.username)...)
	}
	if c.passwordFlag == 1 {
		body = append(body, encodeByteField(c.password)...)
	}
	return append(c.fixedHeader.pack(uint32(len(body))), body...)
}

func (c *connectPacket) unpack(packet []byte) {
	packet, c.protocolName, _ = decodeField(packet[c.fixedHeader.length:])
	c.protocolVersion = packet[0]
	options := packet[1]
	c.reservedBit = 1 & options
	c.cleanSession = 1 & (options >> 1)
	c.willFlag = 1 & (options >> 2)
	c.willQos = 3 & (options >> 3)
	c.willRetain = 1 & (options >> 5)
	c.passwordFlag = 1 & (options >> 6)
	c.usernameFlag = 1 & (options >> 7)
	c.keepaliveTimer = binary.BigEndian.Uint16(packet[2:4])
	packet, c.clientIdentifier, _ = decodeField(packet[4:])
	if c.willFlag == 1 {
		packet, c.willTopic, _ = decodeField(packet[:])
		packet, c.willMessage, _ = decodeByteField(packet[:])
	}
	if c.usernameFlag == 1 {
		packet, c.username, _ = decodeField(packet[:])
	}
	if c.passwordFlag == 1 {
		packet, c.password, _ = decodeByteField(packet[:])
	}
}

func (c *connectPacket) Validate() byte {
	if c.passwordFlag == 1 && c.usernameFlag != 1 {
		return CONN_REF_BAD_USER_PASS
	}
	if c.reservedBit != 0 {
		return CONN_PROTOCOL_VIOLATION
	}
	if c.protocolName != "MQIsdp" && c.protocolName != "MQTT" {
		ERROR.Println("Bad protocol name", c.protocolName)
		return CONN_PROTOCOL_VIOLATION
	}
	if len(c.clientIdentifier) > 65535 || len(c.username) > 65535 || len(c.password) > 65535 {
		return CONN_PROTOCOL_VIOLATION
	}
	return CONN_ACCEPTED
}

func (c *connectPacket) msgID() msgID {
	return 0
}

func (c *connectPacket) setMsgID(id msgID) {
}

func (c *connectPacket) packetType() uint8 {
	return CONNECT
}

//CONNACK packet

type connackPacket struct {
	fixedHeader
	topicNameCompression byte
	returnCode           byte
}

func (ca *connackPacket) String() string {
	str := fmt.Sprintf("%s\n", ca.fixedHeader)
	str += fmt.Sprintf("returncode: %d", ca.returnCode)
	return str
}

func (ca *connackPacket) pack() []byte {
	var body []byte
	body = append(body, ca.topicNameCompression)
	body = append(body, ca.returnCode)
	return append(ca.fixedHeader.pack(uint32(2)), body...)
}

func (ca *connackPacket) unpack(packet []byte) {
	packet = packet[ca.fixedHeader.length:]
	ca.topicNameCompression = packet[0]
	ca.returnCode = packet[1]
}

func (ca *connackPacket) msgID() msgID {
	return 0
}

func (ca *connackPacket) setMsgID(id msgID) {
}

func (ca *connackPacket) packetType() uint8 {
	return CONNACK
}

//DISCONNECT packet

type disconnectPacket struct {
	fixedHeader
}

func (d *disconnectPacket) String() string {
	str := fmt.Sprintf("%s\n", d.fixedHeader)
	return str
}

func (d *disconnectPacket) pack() []byte {
	return d.fixedHeader.pack(uint32(0))
}

func (d *disconnectPacket) unpack(packet []byte) {
}

func (d *disconnectPacket) msgID() msgID {
	return 0
}

func (d *disconnectPacket) setMsgID(id msgID) {
}

func (d *disconnectPacket) packetType() uint8 {
	return DISCONNECT
}

//PUBLISH packet

type publishPacket struct {
	fixedHeader
	topicName string
	messageID msgID
	payload   []byte
}

func (p *publishPacket) String() string {
	str := fmt.Sprintf("%s\n", p.fixedHeader)
	str += fmt.Sprintf("topicName: %s messageID: %d\n", p.topicName, p.messageID)
	str += fmt.Sprintf("payload: %s\n", string(p.payload))
	return str
}

func (p *publishPacket) pack() []byte {
	var body []byte
	body = append(body, encodeField(p.topicName)...)
	if p.qos > 0 {
		body = append(body, msgIDToBytes(p.messageID)...)
	}
	body = append(body, p.payload...)
	return append(p.fixedHeader.pack(uint32(len(body))), body...)
}

func (p *publishPacket) unpack(packet []byte) {
	packet, p.topicName, _ = decodeField(packet[p.fixedHeader.length:])
	if p.qos > 0 {
		p.messageID = bytesToMsgID(packet[:2])
		p.payload = packet[2:]
	} else {
		p.payload = packet[:]
	}
}

func (p *publishPacket) Copy() *publishPacket {
	newP := newControlPacket(PUBLISH).(*publishPacket)
	newP.topicName = p.topicName
	newP.payload = p.payload

	return newP
}

func (p *publishPacket) msgID() msgID {
	return p.messageID
}

func (p *publishPacket) setMsgID(id msgID) {
	p.messageID = id
}

func (p *publishPacket) packetType() uint8 {
	return PUBLISH
}

//PUBACK packet

type pubackPacket struct {
	fixedHeader
	messageID msgID
}

func (pa *pubackPacket) String() string {
	str := fmt.Sprintf("%s\n", pa.fixedHeader)
	str += fmt.Sprintf("messageID: %d", pa.messageID)
	return str
}

func (pa *pubackPacket) pack() []byte {
	return append(pa.fixedHeader.pack(uint32(2)), msgIDToBytes(pa.messageID)...)
}

func (pa *pubackPacket) unpack(packet []byte) {
	pa.messageID = bytesToMsgID(packet[:2])
}

func (pa *pubackPacket) msgID() msgID {
	return pa.messageID
}

func (pa *pubackPacket) setMsgID(id msgID) {
	pa.messageID = id
}

func (pa *pubackPacket) packetType() uint8 {
	return PUBACK
}

//PUBREC packet

type pubrecPacket struct {
	fixedHeader
	messageID msgID
}

func (pr *pubrecPacket) String() string {
	str := fmt.Sprintf("%s\n", pr.fixedHeader)
	str += fmt.Sprintf("messageID: %d", pr.messageID)
	return str
}

func (pr *pubrecPacket) pack() []byte {
	return append(pr.fixedHeader.pack(uint32(2)), msgIDToBytes(pr.messageID)...)
}

func (pr *pubrecPacket) unpack(packet []byte) {
	pr.messageID = bytesToMsgID(packet[:2])
}

func (pr *pubrecPacket) msgID() msgID {
	return pr.messageID
}

func (pr *pubrecPacket) setMsgID(id msgID) {
	pr.messageID = id
}

func (pr *pubrecPacket) packetType() uint8 {
	return PUBREC
}

//PUBREL packet

type pubrelPacket struct {
	fixedHeader
	messageID msgID
}

func (pr *pubrelPacket) String() string {
	str := fmt.Sprintf("%s\n", pr.fixedHeader)
	str += fmt.Sprintf("messageID: %d", pr.messageID)
	return str
}

func (pr *pubrelPacket) pack() []byte {
	return append(pr.fixedHeader.pack(uint32(2)), msgIDToBytes(pr.messageID)...)
}

func (pr *pubrelPacket) unpack(packet []byte) {
	pr.messageID = bytesToMsgID(packet[:2])
}

func (pr *pubrelPacket) msgID() msgID {
	return pr.messageID
}

func (pr *pubrelPacket) setMsgID(id msgID) {
	pr.messageID = id
}

func (pr *pubrelPacket) packetType() uint8 {
	return PUBREL
}

//PUBCOMP packet

type pubcompPacket struct {
	fixedHeader
	messageID msgID
}

func (pc *pubcompPacket) String() string {
	str := fmt.Sprintf("%s\n", pc.fixedHeader)
	str += fmt.Sprintf("messageID: %d", pc.messageID)
	return str
}

func (pc *pubcompPacket) pack() []byte {
	return append(pc.fixedHeader.pack(uint32(2)), msgIDToBytes(pc.messageID)...)
}

func (pc *pubcompPacket) unpack(packet []byte) {
	pc.messageID = bytesToMsgID(packet[:2])
}

func (pc *pubcompPacket) msgID() msgID {
	return pc.messageID
}

func (pc *pubcompPacket) setMsgID(id msgID) {
	pc.messageID = id
}

func (pc *pubcompPacket) packetType() uint8 {
	return PUBCOMP
}

//SUBSCRIBE packet

type subscribePacket struct {
	fixedHeader
	messageID msgID
	payload   []byte
	topics    []string
	qoss      []byte
}

func (s *subscribePacket) String() string {
	str := fmt.Sprintf("%s\n", s.fixedHeader)
	//str += fmt.Sprintf("messageID: %d topics: %s", s.messageID, string(s.payload[:bytes.Index(s.payload, []byte{0})]))
	str += fmt.Sprintf("messageID: %d topics: %s", s.messageID, string(s.payload[:bytes.Index(s.payload, []byte{0})]))
	return str
}

func (s *subscribePacket) pack() []byte {
	var body []byte
	body = append(body, msgIDToBytes(s.messageID)...)
	body = append(body, s.payload...)
	return append(s.fixedHeader.pack(uint32(len(body))), body...)
}

func (s *subscribePacket) unpack(packet []byte) {
	s.messageID = bytesToMsgID(packet[0:2])
	s.payload = packet[2:]
	payload := packet[2:]
	var topic string
	for payload, topic, _ = decodeField(payload); topic != ""; payload, topic, _ = decodeField(payload) {
		s.topics = append(s.topics, topic)
		s.qoss = append(s.qoss, payload[0])
		payload = payload[1:]
	}
}

func (s *subscribePacket) msgID() msgID {
	return s.messageID
}

func (s *subscribePacket) setMsgID(id msgID) {
	s.messageID = id
}

func (s *subscribePacket) packetType() uint8 {
	return SUBSCRIBE
}

//SUBACK packet

type subackPacket struct {
	fixedHeader
	messageID   msgID
	grantedQoss []byte
}

func (sa *subackPacket) String() string {
	str := fmt.Sprintf("%s\n", sa.fixedHeader)
	str += fmt.Sprintf("messageID: %d", sa.messageID)
	return str
}

func (sa *subackPacket) pack() []byte {
	var body []byte
	body = append(body, msgIDToBytes(sa.messageID)...)
	body = append(body, sa.grantedQoss...)
	return append(sa.fixedHeader.pack(uint32(len(body))), body...)
}

func (sa *subackPacket) unpack(packet []byte) {
	sa.messageID = bytesToMsgID(packet[:2])
}

func (sa *subackPacket) msgID() msgID {
	return sa.messageID
}

func (sa *subackPacket) setMsgID(id msgID) {
	sa.messageID = id
}

func (sa *subackPacket) packetType() uint8 {
	return SUBACK
}

//UNSUBSCRIBE packet

type unsubscribePacket struct {
	fixedHeader
	messageID msgID
	payload   []byte
	topics    []string
}

func (u *unsubscribePacket) String() string {
	str := fmt.Sprintf("%s\n", u.fixedHeader)
	str += fmt.Sprintf("messageID: %d", u.messageID)
	return str
}

func (u *unsubscribePacket) pack() []byte {
	var body []byte
	body = append(body, msgIDToBytes(u.messageID)...)
	body = append(body, u.payload...)
	return append(u.fixedHeader.pack(uint32(len(body))), body...)
}

func (u *unsubscribePacket) unpack(packet []byte) {
	u.messageID = bytesToMsgID(packet[:2])
	u.payload = packet[2:]
	payload := packet[2:]
	var topic string
	for payload, topic, _ = decodeField(payload); topic != ""; payload, topic, _ = decodeField(payload) {
		u.topics = append(u.topics, topic)
	}
}

func (u *unsubscribePacket) msgID() msgID {
	return u.messageID
}

func (u *unsubscribePacket) setMsgID(id msgID) {
	u.messageID = id
}

func (u *unsubscribePacket) packetType() uint8 {
	return UNSUBSCRIBE
}

//UNSUBACK packet

type unsubackPacket struct {
	fixedHeader
	messageID msgID
}

func (ua *unsubackPacket) String() string {
	str := fmt.Sprintf("%s\n", ua.fixedHeader)
	str += fmt.Sprintf("messageID: %d", ua.messageID)
	return str
}

func (ua *unsubackPacket) pack() []byte {
	return append(ua.fixedHeader.pack(uint32(2)), msgIDToBytes(ua.messageID)...)
}

func (ua *unsubackPacket) unpack(packet []byte) {
	ua.messageID = bytesToMsgID(packet[:2])
}

func (ua *unsubackPacket) msgID() msgID {
	return ua.messageID
}

func (ua *unsubackPacket) setMsgID(id msgID) {
	ua.messageID = id
}

func (ua *unsubackPacket) packetType() uint8 {
	return UNSUBACK
}

//PINGREQ packet

type pingreqPacket struct {
	fixedHeader
}

func (pr *pingreqPacket) String() string {
	str := fmt.Sprintf("%s", pr.fixedHeader)
	return str
}

func (pr *pingreqPacket) pack() []byte {
	return pr.fixedHeader.pack(uint32(0))
}

func (pr *pingreqPacket) unpack(packet []byte) {
}

func (pr *pingreqPacket) msgID() msgID {
	return 0
}

func (pr *pingreqPacket) setMsgID(id msgID) {
}

func (pr *pingreqPacket) packetType() uint8 {
	return PINGREQ
}

//PINGRESP packet

type pingrespPacket struct {
	fixedHeader
}

func (pr *pingrespPacket) String() string {
	str := fmt.Sprintf("%s", pr.fixedHeader)
	return str
}

func (pr *pingrespPacket) pack() []byte {
	return pr.fixedHeader.pack(uint32(0))
}

func (pr *pingrespPacket) unpack(packet []byte) {
}

func (pr *pingrespPacket) msgID() msgID {
	return 0
}

func (pr *pingrespPacket) setMsgID(id msgID) {
}

func (pr *pingrespPacket) packetType() uint8 {
	return PINGRESP
}

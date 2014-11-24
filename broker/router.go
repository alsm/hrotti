package hrotti

import (
	. "github.com/alsm/hrotti/packets"
	"strings"
	"sync"
)

type subscriptionMap struct {
	subElements map[string][]string
	subMap      map[string]map[string]byte
	subBitmap   []map[string]map[string]bool
	retained    map[string]*PublishPacket
	sync.RWMutex
}

var subs subscriptionMap

func newSubMap() *subscriptionMap {
	s := &subscriptionMap{}
	s.subElements = make(map[string][]string)
	s.subMap = make(map[string]map[string]byte)
	s.subBitmap = make([]map[string]map[string]bool, 10)
	for i, _ := range s.subBitmap {
		s.subBitmap[i] = make(map[string]map[string]bool)
	}
	s.retained = make(map[string]*PublishPacket)

	return s
}

func (s *subscriptionMap) SetRetained(topic string, message *PublishPacket) {
	DEBUG.Println("Setting retained message for", topic)
	s.RLock()
	defer s.RUnlock()
	if len(message.Payload) == 0 {
		delete(s.retained, topic)
	} else {
		s.retained[topic] = message
	}
}

func match(route []string, topic []string) bool {
	if len(route) == 0 {
		if len(topic) == 0 {
			return true
		}
		return false
	}

	if len(topic) == 0 {
		if route[0] == "#" {
			return true
		}
		return false
	}

	if route[0] == "#" {
		return true
	}

	if (route[0] == "+") || (route[0] == topic[0]) {
		return match(route[1:], topic[1:])
	}

	return false
}

func (h *Hrotti) FindRetained(id string, topic string, qos byte) {
	var deliverList []*PublishPacket
	client := h.getClient(id)
	if strings.ContainsAny(topic, "#+") {
		for rTopic, msg := range h.subs.retained {
			if match(strings.Split(topic, "/"), strings.Split(rTopic, "/")) {
				deliveryMsg := msg.Copy()
				deliveryMsg.Qos = calcMinQos(msg.Qos, qos)
				deliverList = append(deliverList, deliveryMsg)
			}
		}
	} else {
		if msg, ok := h.subs.retained[topic]; ok {
			deliveryMsg := msg.Copy()
			deliveryMsg.Qos = calcMinQos(msg.Qos, qos)
			deliverList = append(deliverList, deliveryMsg)
		}
	}
	if len(deliverList) > 0 {
		for _, msg := range deliverList {
			if msg.Qos > 0 {
				if client.Connected() {
					h.PersistStore.Add(id, OUTBOUND, msg)
					select {
					case client.outboundMessages <- msg:
					default:
					}
				} else {
					h.PersistStore.Add(id, OUTBOUND, msg)
				}
			} else if client.Connected() {
				select {
				case client.outboundMessages <- msg:
				default:
				}
			}
		}
	}
}

func (h *Hrotti) AddSub(client string, subscription string, qos byte) {
	h.subs.Lock()
	defer h.subs.Unlock()
	if _, ok := h.subs.subElements[subscription]; !ok {
		h.subs.subElements[subscription] = strings.Split(subscription, "/")
	}
	if _, ok := h.subs.subMap[subscription]; !ok {
		h.subs.subMap[subscription] = make(map[string]byte)
	}
	h.subs.subMap[subscription][client] = qos
	for i, element := range append(h.subs.subElements[subscription], "\u0000") {
		if _, ok := h.subs.subBitmap[i][element]; !ok {
			h.subs.subBitmap[i][element] = make(map[string]bool)
		}
		h.subs.subBitmap[i][element][subscription] = true
	}
	go h.FindRetained(client, subscription, qos)
}

func (h *Hrotti) DeleteSub(client string, subscription string) {
	h.subs.Lock()
	defer h.subs.Unlock()
	if _, ok := h.subs.subMap[subscription]; ok {
		delete(h.subs.subMap[subscription], client)
	}
}

func (h *Hrotti) DeleteSubAll(client string) {
	h.subs.Lock()
	defer h.subs.Unlock()
	for _, topic := range h.subs.subMap {
		if _, ok := topic[client]; ok {
			delete(topic, client)
		}
	}
}

func (h *Hrotti) DeliverMessage(topic string, message *PublishPacket) {
	h.subs.RLock()
	topicElements := strings.Split(topic, "/")
	var matches []string
	var hashMatches []string
	deliverList := make(map[string]byte)
	for i, element := range append(topicElements, "\u0000") {
		DEBUG.Println("Searching bitmap level", i, element)
		switch i {
		case 0:
			for sub, _ := range h.subs.subBitmap[i][element] {
				matches = append(matches, sub)
			}
			for sub, _ := range h.subs.subBitmap[i]["+"] {
				matches = append(matches, sub)
			}
			for sub, _ := range h.subs.subBitmap[i]["#"] {
				hashMatches = append(hashMatches, sub)
			}
		default:
			var tmpMatches []string
			for _, sub := range matches {
				switch element {
				case "\u0000":
					if h.subs.subBitmap[i][element][sub] {
						tmpMatches = append(tmpMatches, sub)
					}
				default:
					if h.subs.subBitmap[i][element][sub] || h.subs.subBitmap[i]["+"][sub] {
						tmpMatches = append(tmpMatches, sub)
					} else if h.subs.subBitmap[i]["#"][sub] {
						hashMatches = append(hashMatches, sub)
					}
				}
			}
			matches = tmpMatches
		}
		if len(matches) == 0 {
			break
		}
	}
	h.subs.RUnlock()

	zeroCopy := message.Copy()
	zeroCopy.Qos = 0

	for _, sub := range append(hashMatches, matches...) {
		for c, qos := range h.subs.subMap[sub] {
			if currQos, ok := deliverList[c]; ok {
				deliverList[c] = calcMinQos(calcMaxQos(currQos, qos), message.Qos)
			} else {
				deliverList[c] = calcMinQos(qos, message.Qos)
			}
		}
	}

	DEBUG.Println(deliverList)
	for cid, subQos := range deliverList {
		client := h.getClient(cid)
		if subQos > 0 {
			go func(c *Client, subQos byte) {
				deliveryMessage := message.Copy()
				deliveryMessage.Qos = subQos
				if c.Connected() {
					//deliveryMessage.MessageID = c.getMsgID(deliveryMessage.UUID())
					h.PersistStore.Add(c.clientID, OUTBOUND, deliveryMessage)
					select {
					case c.outboundMessages <- deliveryMessage:
					default:
					}
				} else {
					h.PersistStore.Add(c.clientID, OUTBOUND, deliveryMessage)
				}
			}(client, subQos)
		} else if client.Connected() {
			select {
			case client.outboundMessages <- zeroCopy:
			default:
			}
		}
	}
}

func calcMinQos(a, b byte) byte {
	if a < b {
		return a
	}
	return b
}

func calcMaxQos(a, b byte) byte {
	if a > b {
		return a
	}
	return b
}

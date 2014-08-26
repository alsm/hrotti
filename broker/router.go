package hrotti

import (
	. "github.com/alsm/hrotti/packets"
	"strings"
	"sync"
)

type retainedMap struct {
	sync.RWMutex
	retainedMessages map[string]*PublishPacket
}

type subscriptionMap struct {
	subElements map[string][]string
	subMap      map[string]map[*Client]byte
	subBitmap   []map[string]map[string]bool
	retained    retainedMap
	sync.RWMutex
}

var subs subscriptionMap

func init() {
	subs.subElements = make(map[string][]string)
	subs.subMap = make(map[string]map[*Client]byte)
	subs.subBitmap = make([]map[string]map[string]bool, 10)
	for i, _ := range subs.subBitmap {
		subs.subBitmap[i] = make(map[string]map[string]bool)
	}
	subs.retained.retainedMessages = make(map[string]*PublishPacket)
}

func SetRetained(topic string, message *PublishPacket) {
	subs.retained.Lock()
	defer subs.retained.RUnlock()
	if len(message.Payload) == 0 {
		delete(subs.retained.retainedMessages, topic)
	} else {
		subs.retained.retainedMessages[topic] = message
	}
}

func AddSub(client *Client, subscription string, qos byte) {
	subs.Lock()
	defer subs.Unlock()
	if _, ok := subs.subElements[subscription]; !ok {
		subs.subElements[subscription] = strings.Split(subscription, "/")
	}
	if _, ok := subs.subMap[subscription]; !ok {
		subs.subMap[subscription] = make(map[*Client]byte)
	}
	subs.subMap[subscription][client] = qos
	for i, element := range append(subs.subElements[subscription], "\u0000") {
		if _, ok := subs.subBitmap[i][element]; !ok {
			subs.subBitmap[i][element] = make(map[string]bool)
		}
		subs.subBitmap[i][element][subscription] = true
	}
}

func DeleteSub(client *Client, subscription string) {
	subs.Lock()
	defer subs.Unlock()
	if _, ok := subs.subMap[subscription]; ok {
		delete(subs.subMap[subscription], client)
	}
}

func DeliverMessage(topic string, message *PublishPacket, hrotti *Hrotti) {
	subs.RLock()
	topicElements := strings.Split(topic, "/")
	var matches []string
	var hashMatches []string
	deliverList := make(map[*Client]byte)
	for i, element := range append(topicElements, "\u0000") {
		DEBUG.Println("Searching bitmap level", i, element)
		switch i {
		case 0:
			for sub, _ := range subs.subBitmap[i][element] {
				matches = append(matches, sub)
			}
			for sub, _ := range subs.subBitmap[i]["+"] {
				matches = append(matches, sub)
			}
			for sub, _ := range subs.subBitmap[i]["#"] {
				hashMatches = append(hashMatches, sub)
			}
		default:
			var tmpMatches []string
			for _, sub := range matches {
				switch element {
				case "\u0000":
					if subs.subBitmap[i][element][sub] {
						tmpMatches = append(tmpMatches, sub)
					}
				default:
					if subs.subBitmap[i][element][sub] || subs.subBitmap[i]["+"][sub] {
						tmpMatches = append(tmpMatches, sub)
					} else if subs.subBitmap[i]["#"][sub] {
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
	subs.RUnlock()

	zeroCopy := message.Copy()
	zeroCopy.Qos = 0

	for _, sub := range append(hashMatches, matches...) {
		for c, qos := range subs.subMap[sub] {
			if currQos, ok := deliverList[c]; ok {
				deliverList[c] = calcMinQos(calcMaxQos(currQos, qos), message.Qos)
			} else {
				deliverList[c] = calcMinQos(qos, message.Qos)
			}
		}
	}

	for client, subQos := range deliverList {
		if subQos > 0 {
			go func(client *Client, subQos byte) {
				deliveryMessage := message.Copy()
				deliveryMessage.Qos = subQos
				if client.Connected() {
					deliveryMessage.MessageID = client.getMsgID(deliveryMessage.UUID)
					hrotti.PersistStore.Add(client, OUTBOUND, deliveryMessage)
					select {
					case client.outboundMessages <- deliveryMessage:
					default:
					}
				} else {
					hrotti.PersistStore.Add(client, OUTBOUND, deliveryMessage)
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

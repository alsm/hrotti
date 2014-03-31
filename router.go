package main

import (
	"fmt"
	"sync"
)

var rootNode *Node = NewNode("")

func NewNode(name string) *Node {
	return &Node{Name: name,
		HashSub: make(map[*Client]byte),
		Sub:     make(map[*Client]byte),
		Nodes:   make(map[string]*Node),
	}
}

type Entry struct {
	Client *Client
	Qos    byte
}

type Node struct {
	sync.RWMutex
	Name     string
	HashSub  map[*Client]byte
	Sub      map[*Client]byte
	Nodes    map[string]*Node
	Retained *publishPacket
}

func (n *Node) Print(prefix string) string {
	for _, v := range n.Nodes {
		fmt.Printf("%s ", v.Print(prefix+"--"))
		if len(v.HashSub) > 0 || len(v.Sub) > 0 {
			for c, _ := range v.Sub {
				fmt.Printf("%s ", c.clientId)
			}
			for c, _ := range v.HashSub {
				fmt.Printf("%s ", c.clientId)
			}
		}
		fmt.Printf("\n")
	}
	return prefix + n.Name
}

func (n *Node) AddSub(client *Client, subscription []string, qos byte, complete chan bool) {
	n.Lock()
	defer n.Unlock()
	switch x := len(subscription); {
	case x > 0:
		if subscription[0] == "#" {
			n.HashSub[client] = qos
			complete <- true
			go n.SendRetainedRecursive(client)
		} else {
			subTopic := subscription[0]
			if _, ok := n.Nodes[subTopic]; !ok {
				n.Nodes[subTopic] = NewNode(subTopic)
			}
			go n.Nodes[subTopic].AddSub(client, subscription[1:], qos, complete)
		}
	case x == 0:
		n.Sub[client] = qos
		complete <- true
		if n.Retained != nil {
			client.outboundMessages.Push(n.Retained)
		}
	}
}

func (n *Node) FindRetainedForPlus(client *Client, subscription []string) {
	n.RLock()
	defer n.RUnlock()
	switch x := len(subscription); {
	case x > 0:
		if subscription[0] == "+" {
			for _, n := range n.Nodes {
				go n.FindRetainedForPlus(client, subscription[1:])
			}
		} else {
			if node, ok := n.Nodes[subscription[0]]; ok {
				go node.FindRetainedForPlus(client, subscription[1:])
			}
		}
	case x == 0:
		if n.Retained != nil {
			client.outboundMessages.Push(n.Retained)
		}
	}
}

func (n *Node) SendRetainedRecursive(client *Client) {
	n.RLock()
	defer n.RUnlock()
	for _, node := range n.Nodes {
		go node.SendRetainedRecursive(client)
	}
	if n.Retained != nil {
		client.outboundMessages.Push(n.Retained)
	}
}

func (n *Node) DeleteSub(client *Client, subscription []string, complete chan bool) {
	n.Lock()
	defer n.Unlock()
	switch x := len(subscription); {
	case x > 0:
		if subscription[0] == "#" {
			delete(n.HashSub, client)
			complete <- true
		} else {
			go n.Nodes[subscription[0]].DeleteSub(client, subscription[1:], complete)
		}
	case x == 0:
		delete(n.Sub, client)
		complete <- true
	}
}

func (n *Node) FindRecipients(topic []string, recipients chan *Entry, wg *sync.WaitGroup) {
	n.RLock()
	defer n.RUnlock()
	defer wg.Done()
	for client, subQos := range n.HashSub {
		recipients <- &Entry{client, subQos}
	}
	switch x := len(topic); {
	case x > 0:
		if node, ok := n.Nodes[topic[0]]; ok {
			wg.Add(1)
			go node.FindRecipients(topic[1:], recipients, wg)
			return
		}
		if node, ok := n.Nodes["+"]; ok {
			wg.Add(1)
			go node.FindRecipients(topic[1:], recipients, wg)
			return
		}
	case x == 0:
		for client, subQos := range n.Sub {
			recipients <- &Entry{client, subQos}
		}
		return
	}
}

func (n *Node) DeliverMessage(topic []string, message *publishPacket) {
	var treeWorkers sync.WaitGroup
	finished := make(chan bool)
	recipients := make(chan *Entry, 10)
	deliverList := make(map[*Client]byte)
	persistList := make(map[*Client]*publishPacket)

	treeWorkers.Add(1)
	go func() {
		treeWorkers.Wait()
		close(finished)
	}()
	go n.FindRecipients(topic, recipients, &treeWorkers)

FindRecipientsLoop:
	for {
		select {
		case entry := <-recipients:
			if currQos, ok := deliverList[entry.Client]; ok {
				deliverList[entry.Client] = calcMinQos(calcMaxQos(currQos, entry.Qos), message.Qos)
			} else {
				deliverList[entry.Client] = calcMinQos(entry.Qos, message.Qos)
			}
		case <-finished:
			break FindRecipientsLoop
		}
	}

	zeroCopy := message.Copy()
	zeroCopy.Qos = 0
	for client, subQos := range deliverList {
		switch subQos {
		case 0:
			client.outboundMessages.Push(zeroCopy)
		}
		if subQos > 0 {
			deliveryMessage := message.Copy()
			deliveryMessage.Qos = subQos
			deliveryMessage.messageId = client.getId()
			persistList[client] = deliveryMessage
		}
	}

	outboundPersist.AddBatch(persistList)

	for client, deliveryMessage := range persistList {
		client.outboundMessages.Push(deliveryMessage)
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

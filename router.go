package main

import (
	//"container/heap"
	"fmt"
	"sync"
)

var rootNode *Node = NewNode("")

func NewNode(name string) *Node {
	return &Node{Name: name,
		HashSub: make(map[*Client]uint),
		Sub:     make(map[*Client]uint),
		Nodes:   make(map[string]*Node),
	}
}

type Node struct {
	sync.RWMutex
	Name     string
	HashSub  map[*Client]uint
	Sub      map[*Client]uint
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

func (n *Node) AddSub(client *Client, subscription []string, qos uint, complete chan bool) {
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

func (n *Node) DeliverMessage(topic []string, message ControlPacket) {
	n.RLock()
	defer n.RUnlock()
	for client, _ := range n.HashSub {
		if client.connected {
			client.outboundMessages.Push(message)
		}
	}
	switch x := len(topic); {
	case x > 0:
		if node, ok := n.Nodes[topic[0]]; ok {
			go node.DeliverMessage(topic[1:], message)
			return
		}
		if node, ok := n.Nodes["+"]; ok {
			go node.DeliverMessage(topic[1:], message)
			return
		}
	case x == 0:
		for client, _ := range n.Sub {
			if client.connected {
				client.outboundMessages.Push(message)
			}
		}
		return
	}
}

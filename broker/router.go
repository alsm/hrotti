package hrotti

import (
	"fmt"
	"sync"
)

//The Topic tree is setup so that it branches at every "/" in a topic string, for example;
//the topic A/B is represented with an unnamed root node, off which is a node called "A",
//off which is a node called B.
//Each Node has 2 subscriptions maps of Clients to QoS, HashSub is a record of all clients
//who have made a # wildcard subscription at the level past this node. for example a client
//who subscribes to A/# will have an entry in the HashSub for the node "A"
//The map called Sub is a similar map of those clients who have subscribed precisely to that
//node, eg subscribing to A/B your client will be recorded in Sub for node "B".
//The last map in a Node is of strings to pointers to other Nodes that are further along the
//tree.
//Each Node can also have a PUBLISH set as its retained message, a retained message is
//delivered to any client that makes a subscription that would make that topic visible to it.
//If there is a retained message on A/B and you susbscribe to A/B you will be sent the message.
//If you subscribe to A/# you would also receive the retained message from A/B.

var rootNode *Node = NewNode("")

//function to create a new Node with the name; name.
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

func (n *Node) AddSub(client *Client, subscription []string, qos byte, complete chan byte) {
	//Lock this Node while we are working in it, we only lock individual Nodes rather than the
	//whole path through the Tree otherwise we'd effectively be locking the whole tree everytime
	//we wanted to make a change.
	n.Lock()
	defer n.Unlock()
	//subscription is a slice of strings representing the topic being subscribed to split on "/"s
	//switch on the length of the slice. this function calls itself removing one entry from this
	//slice every time.
	switch x := len(subscription); {
	//if there are still strings left in the slice...
	case x > 0:
		//if the head entry is a # then we have effectively reached the end of the list and
		//need to add the client to the hashsub at this level.
		//Once down initiate a recursive search from this level on for any retained messages.
		if subscription[0] == "#" {
			n.HashSub[client] = qos
			complete <- qos
			go n.SendRetainedRecursive(client)
		} else {
			//if the head entry is not a "#" then read off the head, check if we already have
			//a subNode for this subTopic, if not create it.
			//Call a goroutine on the next Node reducing the length of the subscription slice
			subTopic := subscription[0]
			if _, ok := n.Nodes[subTopic]; !ok {
				n.Nodes[subTopic] = NewNode(subTopic)
			}
			go n.Nodes[subTopic].AddSub(client, subscription[1:], qos, complete)
		}
	//if the length of subscription is 0 then we have reached the end of the topic list and
	//should add this Client to our Sub map. Once done check if this node has a retained
	//message, if so send it (or drop if the channel is full)
	case x == 0:
		n.Sub[client] = qos
		complete <- qos
		if n.Retained != nil {
			select {
			case client.outboundMessages <- n.Retained:
			default:
			}
		}
	}
}

func (n *Node) FindRetainedForPlus(client *Client, subscription []string) {
	//A subscription with a "+" in it has to find retained messages across a broader range
	//of topics
	n.RLock()
	defer n.RUnlock()
	switch x := len(subscription); {
	//We call ourselves repeatedly reducing the subscription slice by one every time.
	//if the string at the head is a "+" then we need to call this function on every
	//subNode of this Node. If it's not a + call this function on the appropriate
	//subNode. Once the string is 0 we're at the end and send any retained message held
	//by the Node.
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
			select {
			case client.outboundMessages <- n.Retained:
			default:
			}
		}
	}
}

func (n *Node) SendRetainedRecursive(client *Client) {
	n.RLock()
	defer n.RUnlock()
	//for a # subscription we simply start at the node where the client was added to
	//the HashSub map and for every known Node underneath it we check for and send
	//any retained messages.
	for _, node := range n.Nodes {
		go node.SendRetainedRecursive(client)
	}
	if n.Retained != nil {
		select {
		case client.outboundMessages <- n.Retained:
		default:
		}
	}
}

func (n *Node) DeleteSub(client *Client, subscription []string, complete chan bool) {
	n.Lock()
	defer n.Unlock()
	switch x := len(subscription); {
	//Deleting a subscription works in effectively the same way as adding a subscription
	//but rather than adding to the HashSub/Sub maps we remove the entry for this client
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

func (n *Node) DeleteSubAll(client *Client) {
	n.Lock()
	defer n.Unlock()
	//When we want to remove all subscriptions for a client simply iterate through every
	//known Node removing the client from the HashSub/Sub, we don't have to test whether
	//the client is in the map before we delete it, the delete function always suceeds.
	delete(n.HashSub, client)
	delete(n.Sub, client)
	for _, node := range n.Nodes {
		go node.DeleteSubAll(client)
	}
}

func (n *Node) SetRetained(topic []string, message *publishPacket) {
	n.Lock()
	defer n.Unlock()
	switch x := len(topic); {
	//Setting a retained message, we may have to create the Nodes for the topic as there is
	//no guarantee that there are already any subscribers to it.
	case x > 0:
		subTopic := topic[0]
		if _, ok := n.Nodes[subTopic]; !ok {
			n.Nodes[subTopic] = NewNode(subTopic)
		}
		go n.Nodes[subTopic].SetRetained(topic[1:], message)
	//once the topic slice is empty check the value of the payload of the retained message
	//a nil value payload has a special meaning that says if there is currently a retained
	//message set for this Node it should be removed.
	case x == 0:
		if len(message.payload) == 0 {
			n.Retained = nil
		} else {
			n.Retained = message
		}
	}
}

func (n *Node) FindRecipients(topic []string, recipients chan *Entry, wg *sync.WaitGroup) {
	n.RLock()
	defer n.RUnlock()
	defer wg.Done()
	//We're trying to find all matching clients for the topic specified by topic. Firstly
	//we send back down the recipients channel all entries in the HashSub, as no matter the
	//Node we're in, by definition they match.
	for client, subQos := range n.HashSub {
		recipients <- &Entry{client, subQos}
	}
	switch x := len(topic); {
	//if there are still entries in the slice, check if there is a subNode for the head value
	//if so add 1 to the waitgroup for the call to this node and call FindRecipients on the Node
	//Also check is there is a "+" entry in the subNodes, if so we need to start a separate
	//goroutine to check if there are any matching recipients down that path, so again add
	//to the waitgroup and call.
	case x > 0:
		if node, ok := n.Nodes[topic[0]]; ok {
			wg.Add(1)
			go node.FindRecipients(topic[1:], recipients, wg)
		}
		if node, ok := n.Nodes["+"]; ok {
			wg.Add(1)
			go node.FindRecipients(topic[1:], recipients, wg)
		}
		return
	//we've got all the way to the end of the topic so for any Sub entries send them down
	//the recipients channel and return.
	case x == 0:
		for client, subQos := range n.Sub {
			recipients <- &Entry{client, subQos}
		}
		return
	}
}

func (n *Node) DeliverMessage(topic []string, message *publishPacket, hrotti *Hrotti) {
	//to workout who we have to deliver a message to we need to trawl the topic space finding
	//all matching subscribers on the way, due to wild card this wont necessarily be a single
	//path from the root node to the end of the topic, and as each Node is looked at by an
	//independent goroutine we need a way to know when all the goroutines have finished their
	//work and we have all the recipients. That is what the treeWorkers waitgroup is for.
	var treeWorkers sync.WaitGroup
	recipients := make(chan *Entry)
	deliverList := make(map[*Client]byte)
	//persistList := make(map[*Client]*publishPacket)

	//start the process to go and find all the appropriate recipients for this message
	treeWorkers.Add(1)
	go n.FindRecipients(topic, recipients, &treeWorkers)
	//start a small goroutine that waits for all the treeWorkers to finish (call Done())
	//then close the recipients channel to let the next bit of code know that we have
	//identified all the clients to send the message to
	go func() {
		treeWorkers.Wait()
		close(recipients)
	}()

	//loop and receive the entries (struct of Client pointer and QoS) from the recipients channel
	//using the two value form of reading from a channel so we know when the little goroutine
	//previously has closed the channel and we have all the recipients
	for {
		entry, ok := <-recipients
		if ok {
			//it's possible that a client has overlapping matching subscriptions (ie when using a
			//wildcard sub and a specific one), in that case we need to determine the maximum Qos
			//of the current entry in the delivery list and the Qos from this new entry, then we need
			//the minimum Qos value of this calculation and the Qos the message was originally sent
			//with. If this is the first time we've seen this client we just calculate the minimum
			//of the subscription Qos and the message Qos
			if currQos, ok := deliverList[entry.Client]; ok {
				deliverList[entry.Client] = calcMinQos(calcMaxQos(currQos, entry.Qos), message.Qos)
			} else {
				deliverList[entry.Client] = calcMinQos(entry.Qos, message.Qos)
			}
		} else {
			break
		}
	}

	//being as QoS 0 messages have no message id we can just create a single copy of the message for
	//QoS 0 subscribers and sent a pointer to this single message to all delivery routines for those
	//clients, here we create that one QoS0 message
	zeroCopy := message.Copy()
	zeroCopy.Qos = 0
	//now we range through the delivery list with the client pointer and the QoS to send the message
	//to them at.
	for client, subQos := range deliverList {
		//If the QoS is > 0 create a copy of the message as it will need a unique message id assigning
		//before it is sent to the client, and set the appropriate QoS value on this copy.
		//If the client is connected then we should get a messageid from client.idChan, assign
		//message id and persist the message, attempt to send to the outbound message channel for the
		//client. We actually account with both selects here for the case where the client has gone
		//offline after calling Connected().
		//If we know the client is offline then assign an internal message id which will be replaced
		//when the client comes back online and the message is delivered.
		if subQos > 0 {
			go func(client *Client, subQos byte) {
				deliveryMessage := message.Copy()
				deliveryMessage.Qos = subQos
				if client.Connected() {
					select {
					case deliveryMessage.messageId = <-client.idChan:
					case deliveryMessage.messageId = <-hrotti.internalMsgIds.idChan:
					}
					hrotti.outboundPersist.Add(client, deliveryMessage)
					select {
					case client.outboundMessages <- deliveryMessage:
					default:
					}
				} else {
					deliveryMessage.messageId = <-hrotti.internalMsgIds.idChan
					hrotti.outboundPersist.Add(client, deliveryMessage)
				}
			}(client, subQos)
		} else if client.Connected() {
			//if the Qos we're sending at is 0 however we don't store the message for offline delivery
			//as the spec does not require it, we just try and add the message to the outbound channel
			//for the client, if the channel is full we just drop the message for that client, spec
			//allows this, QoS0 has no guarantees.
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

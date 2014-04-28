package main

import (
	"strings"
	"sync"
)

//Add a subscription for a client, taking an array of topics to subscribe to and an associated
//slice of QoS values for the topics, return a slice of byte values indicating the granted
//QoS values in topics order.
func (c *Client) AddSubscription(topics []string, qoss []byte) []byte {
	//we're going to use a waitgroup to handle the subscription requests, with 1 goroutine per
	//topic being subscribed to
	var subWorkers sync.WaitGroup
	//this is the slice we'll return and needs to be the same length as the input QoS' slice
	rQos := make([]byte, len(qoss))

	//for every topic in the topics slice, also get the index number of the topic...
	for i, topic := range topics {
		//add one to the waitgroup
		subWorkers.Add(1)
		//run a goroutine that takes the topic, the requested QoS, a pointer to the appropriate
		//index in rQos to put the granted qos value and a pointer to the waitgroup
		go func(topic string, qos byte, entry *byte, wg *sync.WaitGroup) {
			//when we return call Done on the waitgroup
			defer wg.Done()
			//the subscribe routines spawn multiple goroutines themselves so we need a way to
			//get the granted QoS back once they've finished, this channel (complete) is for
			//that purpose, it will be passed through the goroutines until the subscription is
			//made and that routine will pass back the granted QoS.
			complete := make(chan byte, 1)
			//close the channel once this function ends.
			defer close(complete)
			//the subscription routines expect the topic as a slice of strings.
			topicArr := strings.Split(topic, "/")
			//If the first element of the topic is a registered plugin topic then pass this
			//subscription request to the plugin's AddSub function...
			if plugin, ok := pluginNodes[topicArr[0]]; ok {
				go plugin.AddSub(c, topicArr, qos, complete)
			} else {
				//...otherwise subscribe starting at the rootNode of the topic tree for this
				//client
				c.rootNode.AddSub(c, topicArr, qos, complete)
			}
			//put the returned QoS value into the appropriate element in the rQoS slice.
			*entry = <-complete
			//If the topic contained any +'s in it then run the function that looks for any
			//matching retained messages and send them to the client.
			if strings.ContainsAny(topic, "+") {
				c.rootNode.FindRetainedForPlus(c, topicArr)
			}
			INFO.Println("Subscription made for", c.clientId, topic)
		}(topic, qoss[i], &rQos[i], &subWorkers)
	}
	//wait for all (even if only 1) of the workers in the waitgroup to finish.
	subWorkers.Wait()
	//return the slice of granted QoS values.
	return rQos
}

func (c *Client) RemoveSubscription(topic string) bool {
	//As with AddSubscription this process uses multiple goroutines so we use the complete
	//channel to signify when the process has finished, simply a bool value this time though.
	complete := make(chan bool)
	defer close(complete)
	topicArr := strings.Split(topic, "/")
	if plugin, ok := pluginNodes[topicArr[0]]; ok {
		go plugin.DeleteSub(c, topicArr, complete)
	} else {
		c.rootNode.DeleteSub(c, topicArr, complete)
	}
	<-complete
	return true
}

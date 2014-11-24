package hrotti

//Add a subscription for a client, taking an array of topics to subscribe to and an associated
//slice of QoS values for the topics, return a slice of byte values indicating the granted
//QoS values in topics order.
func (h *Hrotti) AddSubscription(c *Client, topics []string, qoss []byte) []byte {
	//this is the slice we'll return and needs to be the same length as the input QoS' slice
	rQos := make([]byte, len(qoss))

	//for every topic in the topics slice, also get the index number of the topic...
	for i, topic := range topics {
		h.AddSub(c.clientID, topic, qoss[i])
		rQos[i] = qoss[i]
	}
	//return the slice of granted QoS values.
	return rQos
}

func (h *Hrotti) RemoveSubscription(c *Client, topic string) bool {
	h.DeleteSub(c.clientID, topic)
	return true
}

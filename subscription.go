package main

type Subscription struct {
	topicFilter string
	qos         byte
}

func NewSubscription(topicFilter string, qos byte) *Subscription {
	return &Subscription{topicFilter, qos}
}

func (s *Subscription) match(topicFilter string) bool {
	return s.topicFilter == topicFilter
}

package main

import (
	"testing"
)

func Test_NewSubscription_simple(t *testing.T) {
	s := NewSubscription("/test", 1)

	if s.topicFilter != "/test" {
		t.Fatalf("bad subscription topic filter")
	}

	if s.qos != 1 {
		t.Fatalf("bad QoS value")
	}
}

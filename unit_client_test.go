package main

import (
	"testing"
)

func Test_NewClient_simple(t *testing.T) {
	c := NewClient(nil, "testClientId")

	if c.clientId != "testClientId" {
		t.Fatalf("bad client id")
	}

	if c.conn != nil {
		t.Fatalf("bad listen connection")
	}

}

func Test_validateClientId(t *testing.T) {
	if validateClientId("TEST") != true {
		t.Fatalf("validateClientId failed")
	}
}

// func Test_AddSubscription(t *testing.T) {
// 	c := NewClient(nil, "testClientId")

// 	s := NewSubscription("/test", 1)

// 	c.AddSubscription(s)

// 	if c.subscriptions.Len() != 1 {
// 		t.Fatalf("bad length for list of subscriptions")
// 	}

// 	if c.subscriptions.Front().Value != s {
// 		t.Fatalf("bad usbscription value")
// 	}
// }

// func Test_RemoveSubsription(t *testing.T) {
// 	c := NewClient(nil, "testClientId")

// 	s1 := NewSubscription("/test", 1)
// 	s2 := NewSubscription("/test2", 2)

// 	c.AddSubscription(s1)

// 	if ok, _ := c.RemoveSubscription(s2); ok != false {
// 		t.Fatalf("Bad Remove subscription, s2 should not be in list")
// 	}

// 	if ok, _ := c.RemoveSubscription(s1); ok != true {
// 		t.Fatalf("Bad Remove Subscriptin, removing s1 failed")
// 	}

// 	if ok, _ := c.RemoveSubscription(s1); ok != false {
// 		t.Fatalf("Bad Remove subscription, s1 should not still be in list")
// 	}
// }

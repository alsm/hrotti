package main

import (
	"fmt"
	"strings"
	"testing"
)

func Test_NewNode(t *testing.T) {
	rootNode := NewNode("test")

	if rootNode == nil {
		t.Fatalf("rootNode is nil")
	}

	if rootNode.Name != "test" {
		t.Fatalf("rootNode name is %s, not test", rootNode.Name)
	}
}

func Test_AddSub(t *testing.T) {
	rootNode := NewNode("")
	c := NewClient(nil, "testClientId")
	sub1 := strings.Split("test/test1/test2/test3", "/")
	sub2 := strings.Split("test/test1/test4/test5", "/")
	complete1 := make(chan bool)
	complete2 := make(chan bool)

	rootNode.AddSub(c, sub1, 1, complete1)
	<-complete1
	rootNode.AddSub(c, sub2, 2, complete2)
	<-complete2

	rootNode.Print("")
	close(complete1)
	close(complete2)
}

func Test_DeleteSub(t *testing.T) {
	rootNode := NewNode("")
	c := NewClient(nil, "testClientId")
	sub1 := strings.Split("test/test1/test2/test3", "/")
	sub2 := strings.Split("test/test1/test4/test5", "/")
	complete1 := make(chan bool)
	complete2 := make(chan bool)
	complete3 := make(chan bool)

	rootNode.AddSub(c, sub1, 1, complete1)
	<-complete1
	rootNode.AddSub(c, sub2, 2, complete2)
	<-complete2

	rootNode.Print("")

	rootNode.DeleteSub(c, sub2, complete3)
	<-complete3

	rootNode.Print("")
	close(complete1)
	close(complete2)
	close(complete3)
}

func Test_DeliverMessage(t *testing.T) {
	rootNode := NewNode("")
	c := NewClient(nil, "testClientId")
	sub := strings.Split("test/test1/test2/test3", "/")
	complete := make(chan bool)

	rootNode.AddSub(c, sub, 1, complete)
	<-complete

	msg := New(PUBLISH).(*publishPacket)
	msg.payload = []byte("Test Message")
	fmt.Println(msg.String())
	rootNode.DeliverMessage(strings.Split("test/test1/test2/test3", "/"), msg)

	recvmsg := <-c.outboundMessages
	if recvmsg == nil {
		t.Fatalf("failed to receive message")
	}

	switch recvmsg.(type) {
	case *publishPacket:
		pmsg := recvmsg.(*publishPacket)
		if string(pmsg.payload) != "Test Message" {
			t.Fatalf("Message payload incorrect: %s", pmsg.payload)
		}
		fmt.Println(pmsg.String())
	}
}

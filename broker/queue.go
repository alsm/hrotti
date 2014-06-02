package hrotti

import (
	"sync"
	//"fmt"
)

type qNode struct {
	msg  ControlPacket
	next *qNode
}

type msgQueue struct {
	sync.RWMutex
	head  *qNode
	tail  *qNode
	size  int
	limit int
	ready chan bool
}

func NewMsgQueue(limit int) *msgQueue {
	return &msgQueue{ready: make(chan bool, limit*2), limit: limit}
}

func (q *msgQueue) Size() int {
	q.RLock()
	defer q.RUnlock()
	return q.size
}

func (q *msgQueue) Push(msg ControlPacket) {
	q.Lock()
	defer q.Unlock()

	if q.size >= q.limit {
		return
	}

	qn := &qNode{msg: msg}

	if q.tail == nil {
		q.tail = qn
		q.head = qn
	} else {
		q.tail.next = qn
		q.tail = qn
	}
	q.size++
	q.ready <- true
}

func (q *msgQueue) PushHead(msg ControlPacket) {
	q.Lock()
	defer q.Unlock()

	qn := &qNode{msg: msg}

	if q.tail == nil {
		q.tail = qn
		q.head = qn
	} else {
		qn.next = q.head
		q.head = qn
	}
	q.size++
	q.ready <- true
}

func (q *msgQueue) Pop() ControlPacket {
	q.Lock()
	defer q.Unlock()

	qn := q.head

	if q.size == 1 {
		q.tail = nil
	} else {
		q.head = qn.next
	}
	q.size--

	return qn.msg
}

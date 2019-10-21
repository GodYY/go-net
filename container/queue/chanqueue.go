package queue

import (
	"errors"
)

var (
	ErrQueueDestroyed = errors.New("queue had been destroyed")
)

type ChanQueue struct {
	ch chan interface{}
}

func (q *ChanQueue) Size() int {
	if q.ch == nil {
		panic(ErrQueueDestroyed)
	}

	return cap(q.ch)
}

func (q *ChanQueue) Push(o interface{}) {
	if q.ch == nil {
		panic(ErrQueueDestroyed)
	}

	q.ch <- o
}

func (q *ChanQueue) Pop(wait bool) (o interface{}) {
	if q.ch == nil {
		panic(ErrQueueDestroyed)
	}

	if !wait {
		select {
		case o = <-q.ch:
		default:
		}
	} else {
		o = <-q.ch
	}
	return
}

// Destroy destroy the queue, i.e., close internal channel.
func (q *ChanQueue) Destroy() {
	if q.ch != nil {
		close(q.ch)
		q.ch = nil
	}
}

func NewChanQueue(size int) *ChanQueue {
	if size <= 0 {
		panic("invalid size")
	}
	return &ChanQueue{ch: make(chan interface{}, size)}
}

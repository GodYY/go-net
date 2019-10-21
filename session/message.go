package session

import (
	"errors"
)

// Message is a capsulation for every single message coded sent by user.
type Message interface {
	// The data that user message be coded to.
	Data() []byte

	// The length of message data.
	Length() int

	// Release message data. It may be allocated from memory management.
	Release()
}

type message struct {
	data []byte
}

func NewMessage(data []byte) *message {
	if data == nil {
		panic(errors.New("nil data"))
	}
	return &message{data: data}
}

func (p *message) Data() []byte {
	return p.data
}

func (p *message) Length() int {
	return len(p.data)
}

func (p *message) Release() {
}

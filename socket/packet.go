package socket

import (
	"errors"
)

type Packet interface {
	// 获取数据
	Data() []byte

	// 数据长度
	Length() int

	// 释放数据包
	Release()
}

type packet struct {
	data []byte
}

func NewPacket(data []byte) *packet {
	if data == nil {
		panic(errors.New("nil data"))
	}
	return &packet{data: data}
}

func (p *packet) Data() []byte {
	return p.data
}

func (p *packet) Length() int {
	return len(p.data)
}

func (p *packet) Release() {
}

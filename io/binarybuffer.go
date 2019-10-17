package io

import (
	"encoding/binary"
	"errors"
	"io"
)

var (
	ErrSizeLEZero         = errors.New("size less then or equal to zero")
	ErrBufferedNotEnough  = errors.New("buffered not enough")
	ErrAvailableNotEnough = errors.New("available not enough")
	ErrNegativeCount      = errors.New("negative count")
)

type Buffer struct {
	buf  []byte
	r, w int
}

func NewBinaryBuffer(size int) *Buffer {
	if size <= 0 {
		panic(ErrSizeLEZero)
	}

	return &Buffer{
		buf: make([]byte, size),
	}
}

func (b *Buffer) Size() int {
	return len(b.buf)
}

func (b *Buffer) Buffered() int {
	return b.w - b.r
}

func (b *Buffer) Available() int {
	return len(b.buf) - b.w
}

func (b *Buffer) Trim() {
	if b.r == 0 {
		return
	}

	n := b.w - b.r
	if n > 0 {
		copy(b.buf[0:n], b.buf[b.r:b.w])
	}
	b.r = 0
	b.w = n
}

func (b *Buffer) Peek(n int) ([]byte, error) {
	if n < 0 {
		return nil, ErrNegativeCount
	}

	var err error
	if b.Buffered() < n {
		err = ErrBufferedNotEnough
	}

	return b.buf[b.r : b.r+n], err
}

func (b *Buffer) Discard(n int) (discarded int, err error) {
	if n < 0 {
		return 0, ErrNegativeCount
	}

	if n == 0 {
		return
	}

	buffered := b.Buffered()
	if buffered < n {
		discarded = buffered
	} else {
		discarded = n
	}

	b.r += discarded
	return
}

func (b *Buffer) ReadUint16() (uint16, error) {
	if b.Buffered() < 2 {
		return 0, ErrBufferedNotEnough
	}
	value := binary.BigEndian.Uint16(b.buf[b.r:])
	b.r += 2
	return value, nil
}

func (b *Buffer) ReadUint32() (uint32, error) {
	if b.Buffered() < 4 {
		return 0, ErrBufferedNotEnough
	}
	value := binary.BigEndian.Uint32(b.buf[b.r:])
	b.r += 4
	return value, nil
}

func (b *Buffer) ReadUint64() (uint64, error) {
	if b.Buffered() < 8 {
		return 0, ErrBufferedNotEnough
	}
	value := binary.BigEndian.Uint64(b.buf[b.r:])
	b.r += 4
	return value, nil
}

func (b *Buffer) Read(bytes []byte) (n int, err error) {
	n = len(bytes)
	if n == 0 {
		return 0, nil
	}

	read := copy(bytes, b.buf[b.r:b.w])
	if read < n {
		n = read
		err = ErrBufferedNotEnough
	}
	b.r += read
	return
}

func (b *Buffer) ReadFrom(reader io.Reader) (n int, err error) {
	if b.Available() == 0 {
		return 0, ErrAvailableNotEnough
	}

	n, err = reader.Read(b.buf[b.w:])
	b.w += n
	return
}

func (b *Buffer) WriteUint16(value uint16) error {
	if b.Available() < 2 {
		return ErrAvailableNotEnough
	}
	binary.BigEndian.PutUint16(b.buf[b.w:], value)
	b.w += 2
	return nil
}

func (b *Buffer) WriteUint32(value uint32) error {
	if b.Available() < 4 {
		return ErrAvailableNotEnough
	}
	binary.BigEndian.PutUint32(b.buf[b.w:], value)
	b.w += 4
	return nil
}

func (b *Buffer) WriteUint64(value uint64) error {
	if b.Available() < 8 {
		return ErrAvailableNotEnough
	}
	binary.BigEndian.PutUint64(b.buf[b.w:], value)
	b.w += 8
	return nil
}

func (b *Buffer) Write(bytes []byte) (n int, err error) {
	n = len(bytes)
	write := copy(b.buf[b.w:], bytes)
	b.w += write
	if write < n {
		n = write
		err = ErrAvailableNotEnough
	}
	return
}

func (b *Buffer) WriteTo(writer io.Writer) (n int, err error) {
	n, err = writer.Write(b.buf[b.r:b.w])
	b.r += n
	return
}

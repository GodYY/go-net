package socket

import (
	"errors"
	"net"
	"sync/atomic"
)

var (
	ErrNilSessionCallback = errors.New("nil session callback")
	ErrListenStarted      = errors.New("listen already started")
	ErrListenStopped      = errors.New("listen already stopped")
)

type Listener struct {
	network string
	addr    string
	l       net.Listener
}

func (l *Listener) Accept() (s Session, e error) {
	switch l.network {
	case "tcp", "tcp4", "tcp6":
		if conn, err := l.l.Accept(); err == nil {
			s, e = newTcpSession(conn), nil
		} else {
			e = err
		}
	}

	return
}

func (l *Listener) Close() error {
	switch l.network {
	case "tcp", "tcp4", "tcp6":
		return l.l.Close()
	}

	return nil
}

func (l *Listener) Network() string {
	return l.network
}

func (l *Listener) Addr() string {
	return l.addr
}

func Listen(network, addr string) (*Listener, error) {
	switch network {
	case "tcp", "tcp4", "tcp6":
		if l, err := net.Listen(network, addr); err == nil {
			return &Listener{network: network, addr: addr, l: l}, nil
		} else {
			return nil, err
		}

	default:
		return nil, ErrUnknownNetwork
	}
}

type NewSessionCallback func(Session, error)

const (
	listenStarted = 1
	listenStopped = 2
)

type ListenThread struct {
	state int32
	l     *Listener
}

func (l *ListenThread) Start(cb NewSessionCallback) error {
	if cb == nil {
		return ErrNilSessionCallback
	}

	if !atomic.CompareAndSwapInt32(&l.state, 0, listenStarted) {
		return ErrListenStarted
	}

	go l.listen(cb)
	return nil
}

func (l *ListenThread) Stop() error {
	if !atomic.CompareAndSwapInt32(&l.state, listenStarted, listenStopped) {
		return ErrListenStopped
	}

	l.l.Close()
	return nil
}

func (l *ListenThread) listen(cb NewSessionCallback) {
	for true {
		s, err := l.l.Accept()

		if atomic.LoadInt32(&l.state) == listenStopped {
			break
		}

		cb(s, err)
	}
}

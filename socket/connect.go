package socket

import (
	"errors"
	"net"
	"time"
)

var (
	ErrUnknownNetwork = errors.New("unknown network")
)

// 连接服务器
func Connect(network, addr string) (Session, error) {
	return ConnectTimeout(network, addr, 0)
}

// 连接服务器，并提供超时机制
func ConnectTimeout(network, addr string, timeout time.Duration) (s Session, e error) {
	switch network {
	case "tcp", "tcp4", "tcp6":
	default:
		return nil, ErrUnknownNetwork
	}

	if conn, err := net.DialTimeout(network, addr, timeout); err == nil {
		switch network {
		case "tcp", "tcp4", "tcp6":
			s, e = newTcpSession(conn), nil
		}
	} else {
		e = err
	}

	return
}

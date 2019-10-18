package socket

import (
	"github.com/Godyy/go-net/io"
	"math"
	"net"
	"time"
)

const (
	TCPMsgSizeLen        = 4
	TCPMaxMsgSize        = math.MaxUint32 - TCPMsgSizeLen
	TCPDefaultMaxMsgSize = 65536
)

type TCPSession struct {
	session
}

func newTcpSession(conn *net.TCPConn) *TCPSession {
	s := &TCPSession{}
	s.session = newSession(s, conn)
	s.maxMsgSize = TCPDefaultMaxMsgSize
	return s
}

func (tcp *TCPSession) SetMaxMessage(size int) error {
	if size <= 0 || size > TCPMaxMsgSize {
		return ErrMaxMsgSize
	}
	return tcp.session.SetMaxMessage(size)
}

func (tcp *TCPSession) sendThread() {
	var (
		sendBuffer = io.NewBinaryBuffer(tcp.sendBuffSize)
		writeSize  = false
		msg        Message
		length     int
		wrote      int
	)

	for !tcp.isClosed(true) {
		first := true
		for sendBuffer.Available() > 0 {
			if msg == nil {
				if first {
					msg = <-tcp.sendChan
					if msg == nil {
						return
					}
				} else {
					select {
					case msg = <-tcp.sendChan:
					default:
					}
				}

				if msg == nil {
					break
				}

				length = msg.Length()
			}

			first = false

			/* 写消息大小 */
			if !writeSize {
				if sendBuffer.Available() < TCPMsgSizeLen {
					break
				}
				sendBuffer.WriteUint32(uint32(length))
				writeSize = true
			}

			/* 写消息 */
			n, _ := sendBuffer.Write(msg.Data()[wrote:length])
			wrote += n
			if wrote == length {
				writeSize = false
				msg.Release()
				msg = nil
				length = 0
				wrote = 0
			}
		}

		for sendBuffer.Buffered() > 0 {
			/* 发送字节流数据 */
			if tcp.sendTimeout > 0 {
				tcp.conn.SetWriteDeadline(time.Now().Add(tcp.sendTimeout))
			}
			_, err := sendBuffer.WriteTo(tcp.conn)
			if err != nil {
				/* 发送错误 */
				tcp.tryClose(err)
				return
			}
		}
		sendBuffer.Trim()
	}
}

func (tcp *TCPSession) receiveThread() {
	var (
		receiveBuffer = io.NewBinaryBuffer(tcp.receiveBuffSize)
		msgBytes      []byte
		msgSize       = int(-1)
		msgRead       int
	)

	for !tcp.isClosed(true) {
		/* 接收字节流数据 */
		receiveBuffer.Trim()
		if tcp.receiveTimeout > 0 {
			tcp.conn.SetReadDeadline(time.Now().Add(tcp.receiveTimeout))
		}
		_, err := receiveBuffer.ReadFrom(tcp.conn)
		if err != nil {
			/* 接收错误 */
			tcp.tryClose(err)
			return
		}

		for receiveBuffer.Buffered() > 0 {
			if msgSize < 0 {
				if receiveBuffer.Buffered() < TCPMsgSizeLen {
					break
				}
				size, _ := receiveBuffer.ReadUint32()
				msgSize = int(size)
				if msgSize > tcp.maxMsgSize {
					// 消息大小超过限制
					tcp.tryClose(ErrMsgTooLarge)
					return
				}
			}

			if msgBytes == nil {
				if receiveBuffer.Buffered() < msgSize {
					if receiveBuffer.Buffered() < receiveBuffer.Size() {
						break
					}

					msgBytes = make([]byte, msgSize)
				} else {
					msgBytes, _ = receiveBuffer.Peek(msgSize)
					receiveBuffer.Discard(msgSize)
					msgRead = msgSize
				}
			}

			if msgRead < msgSize {
				n, _ := receiveBuffer.Read(msgBytes[msgRead:])
				msgRead += n
				if msgRead < msgSize {
					break
				}
			}

			msg, err := tcp.codecs.Decode(msgBytes)
			tcp.notifyEvent(newSessionReceiveEvt(msg, err))
			msgBytes = nil
			msgSize = -1
			msgRead = 0
		}
	}
}

type TCPListener struct {
	l *net.TCPListener
}

func (l *TCPListener) Accept() (s Session, e error) {
	return l.AcceptTCP()
}

func (l *TCPListener) AcceptTCP() (s *TCPSession, e error) {
	if conn, err := l.l.AcceptTCP(); err == nil {
		s, e = newTcpSession(conn), nil
	} else {
		e = err
	}

	return
}

func (l *TCPListener) Close() error {
	return l.Close()
}

func (l *TCPListener) Network() string {
	return l.l.Addr().Network()
}

func (l *TCPListener) Addr() string {
	return l.l.Addr().String()
}

func ListenTCP(network, addr string) (*TCPListener, error) {
	switch network {
	case "tcp", "tcp4", "tcp6":
		if addr, err := net.ResolveTCPAddr(network, addr); err != nil {
			return nil, err
		} else if l, err := net.ListenTCP(network, addr); err != nil {
			return nil, err
		} else {
			return &TCPListener{l: l}, nil
		}

	default:
		return nil, ErrUnknownNetwork
	}
}

func ConnectTCP(network, addr string) (*TCPSession, error) {
	return ConnectTCPTimeout(network, addr, 0)
}

func ConnectTCPTimeout(network, addr string, timeout time.Duration) (s *TCPSession, e error) {
	switch network {
	case "tcp", "tcp4", "tcp6":
		if conn, err := net.DialTimeout(network, addr, timeout); err == nil {
			s = newTcpSession(conn.(*net.TCPConn))
		} else {
			e = err
		}

	default:
		e = ErrUnknownNetwork
	}

	return
}
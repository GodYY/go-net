package session

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

func (tcp *TCPSession) tcpConn() *net.TCPConn { return tcp.session.conn.(*net.TCPConn) }

func (tcp *TCPSession) sendThread() {
	var (
		sendBuffer = io.NewBinaryBuffer(tcp.sendBuffSize)
		writeSize  = false
		msg        Message
		length     int
		wrote      int
	)

	for !tcp.isClosed(true) {
		waitPop := true
		for sendBuffer.Available() > 0 {
			if msg == nil {
				var o = tcp.sendQueue.Pop(waitPop)
				if o == nil {
					break
				}

				msg = o.(Message)
				length = msg.Length()
			}

			waitPop = false

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

			if _, err := sendBuffer.WriteTo(tcp.conn); err != nil {
				// if session had benn closed, directly return.
				if tcp.isClosed(true) {
					return
				}

				if isConnRST(err) {
					// close session.
					tcp.Close()

					evt := newEventClose(close_ConnReset)
					tcp.notifyEvent(evt)
					return
				} else {
					evt := newEventError(newError(ErrorType_SendMessage, err))
					tcp.notifyEvent(evt)

					if isTimeout(err) {
						continue
					} else {
						time.Sleep(100 * time.Millisecond)
					}
				}
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
		discard       bool
	)

	for !tcp.isClosed(true) {
		receiveBuffer.Trim()

		/* 接收字节流数据 */
		if tcp.receiveTimeout > 0 {
			tcp.conn.SetReadDeadline(time.Now().Add(tcp.receiveTimeout))
		}

		// receive network data.
		if n, err := receiveBuffer.ReadFrom(tcp.conn); n == 0 || err != nil {
			// if session had benn closed, directly return.
			if tcp.isClosed(true) {
				return
			}

			switch {
			case n == 0 || isEOF(err):
				// remote close session, local close too.
				tcp.Close()
				evt := newEventClose(close_RemoteClose)
				tcp.notifyEvent(evt)
				return

			case isConnRST(err):
				// connection reset by remote.
				tcp.Close()
				evt := newEventClose(close_ConnReset)
				tcp.notifyEvent(evt)
				return

			default:
				evt := newEventError(newError(ErrorType_ReceiveMessage, err))
				tcp.notifyEvent(evt)

				if isTimeout(err) {
					// read timeout, directly retry.
					continue
				} else {
					// other error, sleep a few time.
					time.Sleep(100 * time.Millisecond)
				}
			}
		} else if discard {
			// receive a size-exceed message before, discard it's data.
			discarded, _ := receiveBuffer.Discard(msgSize)
			msgSize -= discarded
			if msgSize > 0 {
				continue
			}
			msgSize = -1
			discard = false
		}

		for receiveBuffer.Buffered() > 0 {
			if msgSize < 0 {
				if receiveBuffer.Buffered() < TCPMsgSizeLen {
					break
				}

				size, _ := receiveBuffer.ReadUint32()
				msgSize = int(size)
				if msgSize > tcp.maxMsgSize {
					// receive a size-exceed message, notify event and discard it's data.

					evt := newEventError(newError(ErrorType_ReceiveMessage, ErrMsgTooLarge))
					tcp.notifyEvent(evt)

					discarded, _ := receiveBuffer.Discard(msgSize)
					msgSize -= discarded
					if msgSize <= 0 {
						msgSize = -1
						continue
					} else {
						discard = true
						break
					}
				}

				if msgSize > receiveBuffer.Size() {
					// message size exceed receive buffer size, so manual alloc a data buffer.
					msgBytes = make([]byte, msgSize)
				}
			}

			// extract message data.
			if msgBytes != nil {
				// read message data from receive buffer.
				n, _ := receiveBuffer.Read(msgBytes[msgRead:])
				msgRead += n
			} else if receiveBuffer.Buffered() >= msgSize {
				// directly reference bytes of the message of receive buffer.
				msgBytes, _ = receiveBuffer.Peek(msgSize)
				receiveBuffer.Discard(msgSize)
				msgRead = msgSize
			}
			if msgRead < msgSize {
				// partial extract, continue receive data.
				break
			}

			// decode message.
			if msg, err := tcp.codecs.Decode(msgBytes); err != nil {
				// error occur while decoding message.
				tcp.notifyEvent(newEventError(newError(ErrorType_ReceiveMessage, err)))
			} else {
				// message decoded successfully, notify message up.
				tcp.notifyEvent(newEventMessage(msg))
			}

			if msgBytes != nil {
				// release manual-alloc message data buffer.
				msgBytes = nil
			}
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

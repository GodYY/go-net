package socket

import (
	"go/net/io"
	"math"
	"net"
	"time"
)

const (
	TcpMsgSizeLen        = 4
	TcpMaxMsgSize        = math.MaxUint32 - TcpMsgSizeLen
	TcpDefaultMaxMsgSize = 65536
)

type TcpSession struct {
	session
}

func newTcpSession(conn net.Conn) *TcpSession {
	if _, ok := conn.(*net.TCPConn); !ok {
		panic(errConnType)
	}
	s := &TcpSession{}
	s.session = newSession(s, conn)
	s.maxMsgSize = TcpDefaultMaxMsgSize
	return s
}

func (tcp *TcpSession) SetMaxMsgSize(size int) error {
	if size <= 0 || size > TcpMaxMsgSize {
		return ErrMaxMsgSize
	}
	return tcp.session.SetMaxMsgSize(size)
}

func (tcp *TcpSession) sendThread() {
	var (
		sendBuffer = io.NewBinaryBuffer(tcp.sendBuffSize)
		writeSize  = false
		packet     Packet
		length     int
		wrote      int
	)

	for !tcp.isClosed(true) {
		first := true
		for sendBuffer.Available() > 0 {
			if packet == nil {
				if first {
					packet = <-tcp.sendChan
					if packet == nil {
						return
					}
				} else {
					select {
					case packet = <-tcp.sendChan:
					default:
					}
				}

				if packet == nil {
					break
				}

				length = packet.Length()
			}

			first = false

			/* 写消息大小 */
			if !writeSize {
				if sendBuffer.Available() < TcpMsgSizeLen {
					break
				}
				sendBuffer.WriteUint32(uint32(length))
				writeSize = true
			}

			/* 写消息 */
			n, _ := sendBuffer.Write(packet.Data()[wrote:length])
			wrote += n
			if wrote == length {
				writeSize = false
				packet.Release()
				packet = nil
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

func (tcp *TcpSession) receiveThread() {
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
				if receiveBuffer.Buffered() < TcpMsgSizeLen {
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

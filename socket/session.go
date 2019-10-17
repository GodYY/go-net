package socket

import (
	"errors"
	"net"
	"sync"
	"time"
)

var (
	ErrNilEventCallback  = errors.New("nil event callback")
	ErrSessionNotStarted = errors.New("session not started")
	ErrSessionStarted    = errors.New("session started")
	ErrSessionClosed     = errors.New("session closed")
	ErrNilCodecs         = errors.New("nil codecs")
	ErrBuffSize          = errors.New("buffer size error")
	ErrMaxMsgSize        = errors.New("max message size error")
	ErrSendChanSize      = errors.New("send channel size error")
	ErrNilMsg            = errors.New("nil msg")
	ErrMsgTooLarge       = errors.New("msg too large")
)

const (
	defaultSendChanSize    = 10
	defaultSendBuffSize    = 8192
	defaultReceiveBuffSize = 8192
)

// 会话事件回调
type SessionEventCallback func(*session, SessionEvt)

// 套接字会话接口
// 定义套接字网络会话的基本功能
type Session interface {
	// 启动会话
	Start(SessionEventCallback) error

	// 关闭会话
	Close() error

	// 设置编解码器, 会话启动前
	SetCodecs(Codecs) error

	// 设置发送超时，会话启动前
	SetSendTimeout(time.Duration) error

	// 设置接收超时，会话启动前
	SetReceiveTimeout(time.Duration) error

	// 设置发送缓冲区大小
	SetSendBuffer(size int) error

	// 设置接收缓冲区大小
	SetReceiveBuffer(size int) error

	// 设置最大消息大小
	SetMaxMessage(size int) error

	// 设置发送channel大小
	SetSendChan(size int) error

	// 发送消息
	Send(msg interface{}) error
}

type sessionImpl interface {
	Session
	sendThread()
	receiveThread()
}

const (
	sessionStarted = 1 << 0
	sessionClosed  = 1 << 1
)

type session struct {
	impl            sessionImpl
	mtx             sync.Mutex
	state           int32
	conn            net.Conn
	codecs          Codecs               // 编解码器
	sendTimeout     time.Duration        // 发送超时
	receiveTimeout  time.Duration        // 接收超时
	sendBuffSize    int                  // 发送缓冲区大小
	receiveBuffSize int                  // 接收缓冲区大小
	maxMsgSize      int                  // 最大消息大小
	sendChan        chan Packet          // 发送channel
	evtCB           SessionEventCallback // 事件回调
}

func newSession(impl sessionImpl, conn net.Conn) session {
	return session{impl: impl, conn: conn, sendBuffSize: defaultSendBuffSize, receiveBuffSize: defaultReceiveBuffSize}
}

func (s *session) Start(evtCB SessionEventCallback) error {
	if evtCB == nil {
		return ErrNilEventCallback
	}

	s.mtx.Lock()
	defer s.mtx.Unlock()

	if s.codecs == nil {
		return ErrNilCodecs
	}

	if s.isStarted(false) {
		return ErrSessionStarted
	}

	if s.isClosed(false) {
		return ErrSessionClosed
	}

	if s.sendChan == nil {
		s.sendChan = make(chan Packet, defaultSendChanSize)
	}

	s.evtCB = evtCB
	s.state |= sessionStarted

	go s.impl.sendThread()
	go s.impl.receiveThread()

	return nil
}

func (s *session) Close() error {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	if !s.isStarted(false) {
		return ErrSessionNotStarted
	}

	if s.isClosed(false) {
		return ErrSessionClosed
	}

	s.close()

	return nil
}

func (s *session) SetCodecs(c Codecs) error {
	if c == nil {
		return ErrNilCodecs
	}

	s.mtx.Lock()
	defer s.mtx.Unlock()

	if s.isStarted(false) {
		return ErrSessionStarted
	}

	s.codecs = c
	return nil
}

func (s *session) SetSendTimeout(t time.Duration) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	if s.isStarted(false) {
		return ErrSessionStarted
	}
	if s.isClosed(false) {
		return ErrSessionClosed
	}

	s.sendTimeout = t
	return nil
}

func (s *session) SetReceiveTimeout(t time.Duration) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	if s.isStarted(false) {
		return ErrSessionStarted
	}
	if s.isClosed(false) {
		return ErrSessionClosed
	}

	s.receiveTimeout = t
	return nil
}

func (s *session) SetSendBuffer(size int) error {
	if size <= 0 {
		return ErrBuffSize
	}

	s.mtx.Lock()
	defer s.mtx.Unlock()

	if s.isStarted(false) {
		return ErrSessionStarted
	}
	if s.isClosed(false) {
		return ErrSessionClosed
	}

	s.sendBuffSize = size
	return nil
}

func (s *session) SetReceiveBuffer(size int) error {
	if size <= 0 {
		return ErrBuffSize
	}

	s.mtx.Lock()
	defer s.mtx.Unlock()

	if s.isStarted(false) {
		return ErrSessionStarted
	}
	if s.isClosed(false) {
		return ErrSessionClosed
	}

	s.receiveBuffSize = size
	return nil
}

func (s *session) SetMaxMessage(size int) error {
	if size <= 0 {
		return ErrMaxMsgSize
	}

	s.mtx.Lock()
	defer s.mtx.Unlock()

	if s.isStarted(false) {
		return ErrSessionStarted
	}
	if s.isClosed(false) {
		return ErrSessionClosed
	}

	s.maxMsgSize = size
	return nil
}

func (s *session) SetSendChan(size int) error {
	if size <= 0 {
		return ErrSendChanSize
	}

	s.mtx.Lock()
	defer s.mtx.Unlock()

	if s.isStarted(false) {
		return ErrSessionStarted
	}
	if s.isClosed(false) {
		return ErrSessionClosed
	}

	s.sendChan = make(chan Packet, size)
	return nil
}

func (s *session) Send(msg interface{}) error {
	if msg == nil {
		return ErrNilMsg
	}

	s.mtx.Lock()
	if !s.isStarted(false) {
		s.mtx.Unlock()
		return ErrSessionNotStarted
	}
	if s.isClosed(false) {
		s.mtx.Unlock()
		return ErrSessionClosed
	}
	s.mtx.Unlock()

	if packet, err := s.codecs.Encode(msg); err != nil {
		return err
	} else {
		if packet.Length() > s.maxMsgSize {
			return ErrMsgTooLarge
		}
		s.sendChan <- packet
		return nil
	}
}

func (s *session) isStarted(lock bool) bool {
	if lock {
		s.mtx.Lock()
		defer s.mtx.Unlock()
	}
	return s.state&sessionStarted > 0
}

func (s *session) isClosed(lock bool) bool {
	if lock {
		s.mtx.Lock()
		defer s.mtx.Unlock()
	}
	return s.state&sessionClosed > 0
}

func (s *session) close() {
	s.state |= sessionClosed
	s.conn.Close()
	s.conn = nil
	close(s.sendChan)
	s.sendChan = nil
}

func (s *session) notifyEvent(evt SessionEvt) {
	if evt == nil {
		panic(errors.New("evt nil"))
	}

	s.evtCB(s, evt)
}

func (s *session) tryClose(err error) {
	s.mtx.Lock()
	if s.isClosed(false) {
		s.mtx.Unlock()
		return
	}
	s.close()
	s.mtx.Unlock()

	if err != nil {
		s.notifyEvent(newSessionCloseEvt(err))
	}
}

package session

import (
	"github.com/Godyy/go-net/container/queue"
	"net"
	"sync"
	"time"
)

const (
	DefaultSendQueueSize   = 10
	DefaultSendBuffSize    = 8192
	DefaultReceiveBuffSize = 8192
)

// 会话事件回调
type EventCallback func(*session, Event)

// 套接字会话接口
// 定义套接字网络会话的基本功能
type Session interface {
	// 启动会话
	Start(EventCallback) error

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

	// 设置发送队列大小
	SetSendQueue(size int) error

	// 设置选项
	//SetOptions(options ...interface{})

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
	codecs          Codecs           // 编解码器
	sendTimeout     time.Duration    // 发送超时
	receiveTimeout  time.Duration    // 接收超时
	sendBuffSize    int              // 发送缓冲区大小
	receiveBuffSize int              // 接收缓冲区大小
	maxMsgSize      int              // 最大消息大小
	sendQueueSize   int              // 发送队列大小
	sendQueue       *queue.ChanQueue // 发送队列
	evtCB           EventCallback    // 事件回调
}

func newSession(impl sessionImpl, conn net.Conn) session {
	return session{
		impl:            impl,
		conn:            conn,
		sendBuffSize:    DefaultSendBuffSize,
		receiveBuffSize: DefaultReceiveBuffSize,
		sendQueueSize:   DefaultSendQueueSize,
	}
}

func (s *session) Start(evtCB EventCallback) error {
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

	s.sendQueue = queue.NewChanQueue(s.sendQueueSize)

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

	s.state |= sessionClosed
	s.conn.Close()
	s.conn = nil
	s.sendQueue.Destroy()
	s.sendQueue = nil

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

func (s *session) SetSendQueue(size int) error {
	if size <= 0 {
		return ErrSendQueueSize
	}

	s.mtx.Lock()
	defer s.mtx.Unlock()

	if s.isStarted(false) {
		return ErrSessionStarted
	}
	if s.isClosed(false) {
		return ErrSessionClosed
	}

	s.sendQueueSize = size
	return nil
}

func (s *session) Send(msg interface{}) error {
	if msg == nil {
		return ErrNilMessage
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

	if msgCoded, err := s.codecs.Encode(msg); err != nil {
		return err
	} else {
		if msgCoded.Length() > s.maxMsgSize {
			return ErrMsgTooLarge
		}
		s.sendQueue.Push(msgCoded)
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

func (s *session) notifyEvent(evt Event) {
	s.evtCB(s, evt)
}

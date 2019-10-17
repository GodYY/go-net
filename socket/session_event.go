package socket

import (
	"errors"
)

type SessionEvtType uint8

const (
	SessionEvtSendErr = SessionEvtType(1) // 发送错误
	SessionEvtReceive = SessionEvtType(2) // 接收
	SessionEvtClose   = SessionEvtType(3) // 会话关闭
)

// 会话事件接口
type SessionEvt interface {
	Type() SessionEvtType
}

// 发送错误事件
type SessionSendErrEvt struct {
	Msg interface{}
	Err error
}

func newSessionSendErrEvt(msg interface{}, err error) *SessionSendErrEvt {
	return &SessionSendErrEvt{Msg: msg, Err: err}
}

func (e *SessionSendErrEvt) Type() SessionEvtType {
	return SessionEvtSendErr
}

// 接收事件
type SessionReceiveEvt struct {
	Msg interface{}
	Err error
}

func newSessionReceiveEvt(msg interface{}, err error) *SessionReceiveEvt {
	return &SessionReceiveEvt{Msg: msg, Err: err}
}

func (e *SessionReceiveEvt) Type() SessionEvtType {
	return SessionEvtReceive
}

// 会话关闭事件
type SessionCloseEvt struct {
	Err error
}

func newSessionCloseEvt(err error) *SessionCloseEvt {
	if err == nil {
		panic(errors.New("err nil"))
	}
	return &SessionCloseEvt{Err: err}
}

func (e *SessionCloseEvt) Type() SessionEvtType {
	return SessionEvtClose
}

package session

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"syscall"
)

var (
	ErrUnknownNetwork = errors.New("unknown network")

	// Define errors that occur while calling session method.
	ErrNilEventCallback  = errors.New("nil event callback")
	ErrSessionNotStarted = errors.New("session not started")
	ErrSessionStarted    = errors.New("session started")
	ErrSessionClosed     = errors.New("session closed")
	ErrNilCodecs         = errors.New("nil codecs")
	ErrBuffSize          = errors.New("buffer size error")
	ErrMaxMsgSize        = errors.New("max message size error")
	ErrSendQueueSize     = errors.New("send queue size error")
	ErrNilMessage        = errors.New("nil message")
	ErrMsgTooLarge       = errors.New("message too large")
)

type ErrorType int8

const (
	ErrorType_SendMessage    = ErrorType(1)
	ErrorType_ReceiveMessage = ErrorType(2)
)

var (
	errorTypeStrings = [...]string{
		ErrorType_SendMessage:    "SendMessageError",
		ErrorType_ReceiveMessage: "ReceiveMessageError",
	}
)

func (e ErrorType) String() string { return errorTypeStrings[e] }

// Error represent errors that occur while sending and receiving messages during session communication.
type Error struct {
	errType ErrorType
	err     error
}

func (e *Error) Type() ErrorType { return e.errType }

func (e *Error) Error() string {
	return fmt.Sprintf("%s: %s", e.errType.String(), e.err.Error())
}

func newError(t ErrorType, e error) *Error {
	if e == nil {
		panic("nil internal error")
	}
	return &Error{errType: t, err: e}
}

type timeout interface {
	Timeout() bool
}

// if error represent certain operation is timeout.
func isTimeout(e error) bool {
	if e, ok := e.(timeout); ok {
		return e.Timeout()
	}
	return false
}

// if error represent connection reset.
func isConnRST(e error) bool {
	if e, ok := e.(*net.OpError); ok {
		if e, ok := e.Unwrap().(*os.SyscallError); ok {
			if e, ok := e.Unwrap().(syscall.Errno); ok && e == syscall.ECONNRESET {
				return true
			}
		}
	}
	return false
}

// if error represent end of file.
func isEOF(e error) bool {
	return e == io.EOF
}

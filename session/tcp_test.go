package session

import (
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

type stringMsg struct {
	msg []byte
}

type tcpCodecs struct {
}

func (c *tcpCodecs) Encode(o interface{}) (Message, error) {
	switch msg := o.(type) {
	case *stringMsg:
		return NewMessage(msg.msg), nil

	default:
		return nil, errors.New("msg error")
	}
}

func (c *tcpCodecs) Decode(bytes []byte) (interface{}, error) {
	return &stringMsg{msg: bytes}, nil
}

func Test(t *testing.T) {
	var (
		network    = "tcp4"
		addr       = "127.0.0.1:10000"
		listener   Listener
		cliSession Session
		srvSession Session
		err        error

		sendMsgCount  = int32(10)
		clientReceive int32
		serverReceive int32
		closed        int32
	)

	// 启动listener
	fmt.Println("start tcp listener")
	if listener, err = ListenTCP(network, addr); err != nil {
		fmt.Printf("create listener failed, %s\n", err)
		return
	}
	go func() {
		if srvSession, err = listener.Accept(); err == nil {
			srvSession.SetCodecs(&tcpCodecs{})

			// 启动session
			srvSession.Start(func(s *session, event Event) {
				switch event.Type() {
				case EventType_Message:
					msg := event.Message().(*stringMsg)
					atomic.AddInt32(&serverReceive, 1)
					fmt.Printf("server receive msg len:%d\n", len(msg.msg))

				case EventType_Error:
					fmt.Printf("server encounter error, %s\n", event.Error().Error())

				case EventType_Close:
					fmt.Printf("server close, reason: %s\n", event.Reason())
				}
			})

			go func() {
				sendMsgCount := sendMsgCount + 5
				for i := int32(0); i < sendMsgCount; i++ {
					srvSession.Send(&stringMsg{msg: make([]byte, 100)})
				}

				for atomic.LoadInt32(&serverReceive) < sendMsgCount {
					time.Sleep(1 * time.Millisecond)
				}

				fmt.Println("server session close")
				srvSession.Close()

				atomic.AddInt32(&closed, 1)
			}()
		} else {
			fmt.Printf("accept session failed, %s\n", err)
		}
	}()

	// 连接server
	fmt.Println("connect tcp server")
	if cliSession, err = ConnectTCP(network, addr); err != nil {
		fmt.Printf("connect server failed, %s\n", err)
		return
	}
	// 启动client
	cliSession.SetCodecs(&tcpCodecs{})
	cliSession.Start(func(s *session, e Event) {
		switch e.Type() {
		case EventType_Message:
			msg := e.Message().(*stringMsg)
			atomic.AddInt32(&serverReceive, 1)
			fmt.Printf("client receive msg len:%d\n", len(msg.msg))

		case EventType_Error:
			fmt.Printf("client encounter error, %s\n", e.Error().Error())

		case EventType_Close:
			fmt.Printf("client close, reason: %s\n", e.Reason())
		}
	})

	go func() {
		for i := int32(0); i < sendMsgCount; i++ {
			cliSession.Send(&stringMsg{msg: make([]byte, 100)})
		}

		for atomic.LoadInt32(&clientReceive) < sendMsgCount {
			time.Sleep(1 * time.Millisecond)
		}

		fmt.Println("client session close")
		cliSession.Close()

		atomic.AddInt32(&closed, 1)
	}()

	for atomic.LoadInt32(&closed) < 2 {
		time.Sleep(1 * time.Millisecond)
	}
}

func TestMaxMsg(t *testing.T) {
	var (
		network    = "tcp4"
		addr       = "127.0.0.1:10000"
		listener   Listener
		cliSession Session
		srvSession Session
		err        error

		maxMsgSize    = 65536
		sendMsgCount  = int32(10)
		clientReceive int32
		serverReceive int32
		cliClosed     int32
		srvClosed     int32
	)

	// 启动listener
	fmt.Println("start tcp listener")
	if listener, err = ListenTCP(network, addr); err != nil {
		fmt.Printf("create listener failed, %s\n", err)
		return
	}
	go func() {
		if srvSession, err = listener.Accept(); err == nil {
			srvSession.SetCodecs(&tcpCodecs{})
			srvSession.SetSendBuffer(8192)
			srvSession.SetReceiveBuffer(8192)
			srvSession.SetMaxMessage(maxMsgSize)

			// 启动session
			srvSession.Start(func(s *session, e Event) {
				switch e.Type() {
				case EventType_Error:
					fmt.Printf("server encounter error, %s\n", e.Error().Error())

				case EventType_Message:
					msg := e.Message().(*stringMsg)
					atomic.AddInt32(&serverReceive, 1)
					fmt.Printf("server receive msg len:%d\n", len(msg.msg))

				case EventType_Close:
					fmt.Printf("server close, reason: %s\n", e.Reason())
					atomic.StoreInt32(&srvClosed, 1)
				}
			})

			go func() {
				for i := int32(0); i < sendMsgCount; i++ {
					srvSession.Send(&stringMsg{msg: make([]byte, 65536)})
				}

				for atomic.LoadInt32(&serverReceive) < sendMsgCount && atomic.LoadInt32(&srvClosed) == 0 {
					time.Sleep(1 * time.Millisecond)
				}

				fmt.Println("server session close")
				srvSession.Close()
			}()
		} else {
			fmt.Printf("accept session failed, %s\n", err)
		}
	}()

	// 连接server
	fmt.Println("connect tcp server")
	if cliSession, err = ConnectTCP(network, addr); err != nil {
		fmt.Printf("connect server failed, %s\n", err)
		return
	}
	// 启动client
	cliSession.SetCodecs(&tcpCodecs{})
	cliSession.SetSendBuffer(8192)
	cliSession.SetReceiveBuffer(8192)
	cliSession.SetMaxMessage(maxMsgSize)
	cliSession.Start(func(s *session, evt Event) {
		switch evt.Type() {
		case EventType_Error:
			fmt.Printf("client encounter error, %s\n", evt.Error().Error())

		case EventType_Message:
			msg := evt.Message().(*stringMsg)
			atomic.AddInt32(&clientReceive, 1)
			fmt.Printf("client receive msg len:%d\n", len(msg.msg))

		case EventType_Close:
			fmt.Printf("client close, reason: %s\n", evt.Reason())
			atomic.StoreInt32(&cliClosed, 1)
		}
	})
	go func() {
		sendMsgCount := sendMsgCount + 5
		for i := int32(0); i < sendMsgCount; i++ {
			cliSession.Send(&stringMsg{msg: make([]byte, 65536)})
		}

		for atomic.LoadInt32(&clientReceive) < sendMsgCount && atomic.LoadInt32(&cliClosed) == 0 {
			time.Sleep(1 * time.Millisecond)
		}

		fmt.Println("client session close")
		cliSession.Close()
	}()

	for atomic.LoadInt32(&cliClosed) == 0 && atomic.LoadInt32(&srvClosed) == 0 {
		time.Sleep(1 * time.Millisecond)
	}
}

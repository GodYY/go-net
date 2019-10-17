package socket

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

func (c *tcpCodecs) Encode(o interface{}) (Packet, error) {
	switch msg := o.(type) {
	case *stringMsg:
		return NewPacket(msg.msg), nil

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
			srvSession.Start(func(s *session, evt SessionEvt) {
				switch evt.Type() {
				case SessionEvtSendErr:
					e := evt.(*SessionSendErrEvt)
					fmt.Printf("server send error, %s\n", e.Err)

				case SessionEvtReceive:
					e := evt.(*SessionReceiveEvt)
					atomic.AddInt32(&serverReceive, 1)
					fmt.Printf("server receive msg len:%d\n", len(e.Msg.(*stringMsg).msg))

				case SessionEvtClose:
					e := evt.(*SessionCloseEvt)
					fmt.Printf("server close, %s\n", e.Err)
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
	cliSession.Start(func(s *session, evt SessionEvt) {
		switch evt.Type() {
		case SessionEvtSendErr:
			e := evt.(*SessionSendErrEvt)
			fmt.Printf("client send error, %s\n", e.Err)

		case SessionEvtReceive:
			e := evt.(*SessionReceiveEvt)
			atomic.AddInt32(&clientReceive, 1)
			fmt.Printf("client receive msg len:%d\n", len(e.Msg.(*stringMsg).msg))

		case SessionEvtClose:
			e := evt.(*SessionCloseEvt)
			fmt.Printf("client close, %s\n", e.Err)
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
			srvSession.Start(func(s *session, evt SessionEvt) {
				switch evt.Type() {
				case SessionEvtSendErr:
					e := evt.(*SessionSendErrEvt)
					fmt.Printf("server send error, %s\n", e.Err)

				case SessionEvtReceive:
					e := evt.(*SessionReceiveEvt)
					atomic.AddInt32(&serverReceive, 1)
					fmt.Printf("server receive msg len:%d\n", len(e.Msg.(*stringMsg).msg))

				case SessionEvtClose:
					e := evt.(*SessionCloseEvt)
					fmt.Printf("server close, %s\n", e.Err)
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
	cliSession.Start(func(s *session, evt SessionEvt) {
		switch evt.Type() {
		case SessionEvtSendErr:
			e := evt.(*SessionSendErrEvt)
			fmt.Printf("client send error, %s\n", e.Err)

		case SessionEvtReceive:
			e := evt.(*SessionReceiveEvt)
			atomic.AddInt32(&clientReceive, 1)
			fmt.Printf("client receive msg len:%d\n", len(e.Msg.(*stringMsg).msg))

		case SessionEvtClose:
			e := evt.(*SessionCloseEvt)
			fmt.Printf("client close, %s\n", e.Err)
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

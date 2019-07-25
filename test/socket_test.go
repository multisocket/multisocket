// +build all socket

package test

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"bytes"

	"github.com/multisocket/multisocket"
	"github.com/multisocket/multisocket/address"
	"github.com/multisocket/multisocket/connector"
	"github.com/multisocket/multisocket/errs"
	"github.com/multisocket/multisocket/message"
	"github.com/multisocket/multisocket/options"
	_ "github.com/multisocket/multisocket/transport/all"
)

func TestSocketSendRecv(t *testing.T) {
	for idx := range sizes {
		if idx%3 != 0 {
			continue
		}
		size := sizes[idx]
		t.Run(size.name, func(t *testing.T) {
			sz := size.sz
			for idx := range transports {
				tp := transports[idx]
				t.Run(tp.name, func(t *testing.T) {
					addr := tp.addr
					testSocketSendRecv(t, addr, sz)
				})
			}
		})
	}
}

func TestSocketSwitch(t *testing.T) {
	hopsCases := []uint8{1, 2, 4, 8, 16, 17, 18, 32}

	for idx := range hopsCases {
		hops := hopsCases[idx]
		t.Run(fmt.Sprintf("Hops(%d)", hops), func(t *testing.T) {
			for idx := range transports {
				tp := transports[idx]
				t.Run(tp.name, func(t *testing.T) {
					addr := tp.addr
					testSocketSwitch(t, addr, hops)
				})
			}
		})
	}
}

func TestSocketMaxRecvContentLength(t *testing.T) {
	for idx := range sizes {
		if idx%3 != 0 {
			continue
		}

		size := sizes[idx]
		if size.sz < 10 {
			continue
		}
		t.Run(size.name, func(t *testing.T) {
			sz := size.sz
			for idx := range transports {
				tp := transports[idx]
				t.Run(tp.name, func(t *testing.T) {
					addr := tp.addr
					testSocketMaxRecvContentLength(t, addr, sz)
				})
			}
		})
	}
}

func TestSocketCloseSender(t *testing.T) {
	for idx := range sizes {
		if idx%4 != 0 {
			continue
		}

		size := sizes[idx]
		t.Run(size.name, func(t *testing.T) {
			sz := size.sz
			for idx := range transports {
				tp := transports[idx]
				t.Run(tp.name, func(t *testing.T) {
					addr := tp.addr
					testSocketCloseSender(t, addr, sz)
				})
			}
		})
	}
}

func testSocketSendRecv(t *testing.T, addr string, sz int) {
	var (
		err     error
		srvsock multisocket.Socket
		clisock multisocket.Socket
	)
	if srvsock, clisock, err = prepareSocks(addr); err != nil {
		t.Errorf("connect error: %s", err)
	}
	defer srvsock.Close()
	defer clisock.Close()

	go func() {
		var (
			err error
			msg *message.Message
		)
		for {
			if msg, err = srvsock.RecvMsg(); err != nil {
				if err != errs.ErrClosed {
					t.Errorf("RecvMsg error: %s", err)
				}
				break
			}
			if err = srvsock.SendTo(msg.Source, msg.Content); err != nil {
				t.Errorf("SendTo error: %s", err)
			}
			msg.FreeAll()
		}
	}()

	var (
		randSeed = initRandSeed(0)
		content  []byte
		msg      *message.Message
	)
	szMin := sz / 2
	for i := 0; i < 1000; i++ {
		content = genRandomContent(szMin + rand.Intn(szMin+1))
		if err = clisock.Send(content); err != nil {
			t.Errorf("Send error: %s", err)
		}
		if msg, err = clisock.RecvMsg(); err != nil {
			t.Errorf("Recv error: %s", err)
		}
		if !bytes.Equal(content, msg.Content) {
			t.Errorf("send/recv not equal: sz=%d, randSeed=%d, i=%d, len=%d/%d", sz, randSeed, i, len(content), len(msg.Content))
		}
		msg.FreeAll()
	}
}

func testSocketSwitch(t *testing.T, addr string, hops uint8) {
	var (
		err     error
		srvsock multisocket.Socket
		swBack  multisocket.Socket
		clisock multisocket.Socket
	)
	sendTTL := uint8(16)

	if srvsock, swBack, err = prepareSocks(addr); err != nil {
		t.Errorf("connect error: %s", err)
	}
	srvsock.SetOption(multisocket.Options.SendTTL, sendTTL)
	defer srvsock.Close()
	defer swBack.Close()

	// switches
	// srvsock<-|swBack,swFront|<-...-|swBack,swFront|<-clisock
	for i := 1; i < int(hops); i++ {
		swFront, _swBack, err := prepareSocks(fmt.Sprintf("inproc://switch_%d", i))
		if err != nil {
			t.Errorf("connect error: %s", err)
		}
		defer swFront.Close()
		defer _swBack.Close()
		multisocket.StartSwitch(swBack, swFront, nil)
		swBack = _swBack
	}
	clisock = swBack

	msgCount := 0
	done := make(chan struct{})
	go func() {
		var (
			err error
			msg *message.Message
		)
		for {
			if msg, err = srvsock.RecvMsg(); err != nil {
				if err != errs.ErrClosed {
					t.Errorf("RecvMsg error: %s", err)
				}
				break
			}
			if msg.Hops != hops {
				t.Errorf("Server RecvMsg Hops(%d) != %d", msg.Hops, hops)
			}

			if string(msg.Content) == "done" {
				// done
				break
			}

			if msg.TTL+msg.Hops != sendTTL {
				t.Errorf("Server RecvMsg TTL(%d)+Hops(%d) != SendTTL(%d)", msg.TTL, msg.Hops, sendTTL)
			}

			if err = srvsock.SendTo(msg.Source, msg.Content); err != nil {
				t.Errorf("SendTo error: %s", err)
			}
			msg.FreeAll()
			msgCount++
		}
		done <- struct{}{}
	}()

	var (
		content = []byte("Switch Test")
	)
	for i := 0; i < 100; i++ {
		if err = clisock.Send(content); err != nil {
			t.Errorf("Send error: %s", err)
		}
	}
	if err = clisock.SendMsg(message.NewSendMessage(0, message.SendTypeToOne, 128, nil, nil, []byte("done"))); err != nil {
		t.Errorf("Send error: %s", err)
	}
	<-done
	if hops > sendTTL {
		if msgCount != 0 {
			t.Errorf("%d messages ingore TTL(%d/%d)", msgCount, hops, sendTTL)
		}
	} else {
		if msgCount != 100 {
			t.Errorf("%d messages ingore TTL(%d/%d)", 100-msgCount, hops, sendTTL)
		}
	}
}

func testSocketMaxRecvContentLength(t *testing.T, addr string, sz int) {
	var (
		err     error
		srvsock multisocket.Socket
		clisock multisocket.Socket
	)
	maxRecvContentLength := sz * 3 / 4
	if srvsock, clisock, err = prepareSocks(addr, options.OptionValues{connector.Options.Pipe.MaxRecvContentLength: maxRecvContentLength}); err != nil {
		t.Errorf("connect error: %s", err)
	}
	defer srvsock.Close()
	defer clisock.Close()

	count := 10
	done := make(chan struct{})
	go func() {
		var (
			err error
			msg *message.Message
		)
		for i := 1; i <= count; i++ {
			if msg, err = srvsock.RecvMsg(); err != nil {
				if err != errs.ErrClosed {
					t.Errorf("RecvMsg error: %s", err)
				}
				break
			}
			if len(msg.Content) > maxRecvContentLength {
				t.Errorf("maxRecvContentLength=%d, content=%d", maxRecvContentLength, len(msg.Content))
			}
			msg.FreeAll()
		}
		done <- struct{}{}
	}()

	var (
		content []byte
	)
	for i := 1; i <= count+10; i++ {
		content = genRandomContent(maxRecvContentLength - count + i)
		if err = clisock.Send(content); err != nil {
			t.Errorf("Send error: %s", err)
		}
	}
	<-done
}

func testSocketCloseSender(t *testing.T, addr string, sz int) {
	var (
		err     error
		sa      address.MultiSocketAddress
		srvsock multisocket.Socket
		clisock multisocket.Socket
	)

	if sa, err = address.ParseMultiSocketAddress(addr); err != nil {
		t.Errorf("parse address error: %s", err)
	}

	srvsock = multisocket.New(nil)
	clisock = multisocket.New(options.OptionValues{multisocket.Options.SendQueueSize: 640})
	if err = sa.Listen(srvsock); err != nil {
		t.Errorf("server listen error: %s", err)
	}
	if err = sa.Dial(clisock); err != nil {
		t.Errorf("client dial error: %s", err)
	}

	N := 2000
	go func() {
		szMin := sz / 2
		for i := 0; i < N; i++ {
			content := genRandomContent(szMin + rand.Intn(szMin+1))
			if err = clisock.Send(content); err != nil {
				t.Errorf("Send error: %s", err)
			}
		}
		clisock.Close()
		time.AfterFunc(500*time.Millisecond, func() {
			srvsock.Close()
		})
	}()

	var (
		msg *message.Message
	)
	count := 0
	for {
		if msg, err = srvsock.RecvMsg(); err != nil {
			if err != errs.ErrClosed {
				t.Errorf("RecvMsg error: %s", err)
			}
			break
		}
		msg.FreeAll()
		count++
	}
	if count != N {
		t.Errorf("%d messages dropped after sender closed!!", N-count)
	}
}

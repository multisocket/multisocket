package test

import (
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
	"github.com/multisocket/multisocket/sender"
	_ "github.com/multisocket/multisocket/transport/all"
)

func TestSocketSendRecv(t *testing.T) {
	for idx := range sizes {
		if idx%2 != 0 {
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
				break
			}
			msg.FreeAll()
		}
	}()

	var (
		randSeed     = initRandSeed(0)
		content      []byte
		replyContent []byte
	)
	szMin := sz / 2
	for i := 0; i < 2000; i++ {
		content = genRandomContent(szMin + rand.Intn(szMin+1))
		if err = clisock.Send(content); err != nil {
			t.Errorf("Send error: %s", err)
			break
		}
		if replyContent, err = clisock.Recv(); err != nil {
			t.Errorf("Recv error: %s", err)
			break
		}
		if !bytes.Equal(content, replyContent) {
			t.Errorf("send/recv not equal: randSeed=%d, i=%d, len=%d/%d", randSeed, i, len(content), len(replyContent))
			break
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
			break
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

	srvsock = multisocket.NewDefault()
	clisock = multisocket.NewDefault()
	clisock.GetSender().SetOption(sender.Options.SendQueueSize, 6400)
	if err = sa.Listen(srvsock); err != nil {
		t.Errorf("server listen error: %s", err)
	}
	if err = sa.Dial(clisock); err != nil {
		t.Errorf("client dial error: %s", err)
	}

	N := 10000
	go func() {
		szMin := sz / 2
		for i := 0; i < N; i++ {
			content := genRandomContent(szMin + rand.Intn(szMin+1))
			if err = clisock.Send(content); err != nil {
				t.Errorf("Send error: %s", err)
				break
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

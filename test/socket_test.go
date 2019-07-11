package test

import (
	"math/rand"
	"testing"

	"bytes"

	"github.com/webee/multisocket"
	"github.com/webee/multisocket/connector"
	"github.com/webee/multisocket/errs"
	"github.com/webee/multisocket/message"
	"github.com/webee/multisocket/options"
	_ "github.com/webee/multisocket/transport/all"
)

func TestSocketSendRecv(t *testing.T) {
	for idx := range sizes {
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
			if len(msg.Content) > maxRecvContentLength {
				t.Errorf("maxRecvContentLength=%d, content=%d", maxRecvContentLength, len(msg.Content))
			}
			msg.FreeAll()
		}
	}()

	var (
		content []byte
	)
	for i := 0; i < 20; i++ {
		content = genRandomContent(maxRecvContentLength - 10 + i)
		if err = clisock.Send(content); err != nil {
			t.Errorf("Send error: %s", err)
			break
		}
	}
}

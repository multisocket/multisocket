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

func TestSocketMaxRecvBodyLength(t *testing.T) {
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
					testSocketMaxRecvBodyLength(t, addr, sz)
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
			if err = srvsock.SendTo(msg.Source, msg.Body); err != nil {
				t.Errorf("SendTo error: %s", err)
			}
			msg.FreeAll()
		}
	}()

	var (
		randSeed = initRandSeed(0)
		body  []byte
		msg      *message.Message
	)
	szMin := sz / 2
	for i := 0; i < 2000; i++ {
		body = genRandomBytes(szMin + rand.Intn(szMin+1))
		if err = clisock.Send(body); err != nil {
			t.Errorf("Send error: %s", err)
		}
		if msg, err = clisock.RecvMsg(); err != nil {
			t.Errorf("Recv error: %s", err)
		}
		if !bytes.Equal(body, msg.Body) {
			t.Errorf("send/recv not equal: randSeed=%d, i=%d, len=%d/%d", randSeed, i, len(body), len(msg.Body))
		}
		msg.FreeAll()
	}
}

func testSocketMaxRecvBodyLength(t *testing.T, addr string, sz int) {
	var (
		err     error
		srvsock multisocket.Socket
		clisock multisocket.Socket
	)
	maxRecvBodyLength := sz * 3 / 4
	if srvsock, clisock, err = prepareSocks(addr, options.OptionValues{connector.Options.Pipe.MaxRecvBodyLength: maxRecvBodyLength}); err != nil {
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
			if len(msg.Body) > maxRecvBodyLength {
				t.Errorf("maxRecvBodyLength=%d, body=%d", maxRecvBodyLength, len(msg.Body))
			}
			msg.FreeAll()
		}
		done <- struct{}{}
	}()

	var (
		body []byte
	)
	for i := 1; i <= count+10; i++ {
		body = genRandomBytes(maxRecvBodyLength - count + i)
		if err = clisock.Send(body); err != nil {
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
	clisock = multisocket.New(options.OptionValues{multisocket.Options.SendQueueSize: 6400})
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
			body := genRandomBytes(szMin + rand.Intn(szMin+1))
			if err = clisock.Send(body); err != nil {
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

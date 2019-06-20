package connector

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/webee/multisocket/options"

	"github.com/webee/multisocket/errs"
	"github.com/webee/multisocket/transport"
	"github.com/webee/multisocket/utils"
)

// pipe wraps the transport.Connection data structure with the stuff we need to keep.
// It implements the Pipe interface.
type pipe struct {
	sendTimeout time.Duration
	recvTimeout time.Duration
	closeOnEOF  bool

	sync.Mutex
	closed bool
	id     uint32
	parent *connector
	c      transport.Connection
	l      *listener
	d      *dialer
}

var pipeID = utils.NewRecyclableIDGenerator()

func newPipe(parent *connector, tc transport.Connection, d *dialer, l *listener, opts options.Options) *pipe {
	defaultSendTimeout := PipeOptionSendTimeout.Value(parent.GetOptionDefault(PipeOptionSendTimeout, time.Duration(0)))
	defaultRecvTimeout := PipeOptionRecvTimeout.Value(parent.GetOptionDefault(PipeOptionRecvTimeout, time.Duration(0)))
	defaultCloseOnEOF := PipeOptionCloseOnEOF.Value(parent.GetOptionDefault(PipeOptionCloseOnEOF, true))
	return &pipe{
		sendTimeout: PipeOptionSendTimeout.Value(opts.GetOptionDefault(PipeOptionSendTimeout, defaultSendTimeout)),
		recvTimeout: PipeOptionRecvTimeout.Value(opts.GetOptionDefault(PipeOptionRecvTimeout, defaultRecvTimeout)),
		closeOnEOF:  PipeOptionCloseOnEOF.Value(opts.GetOptionDefault(PipeOptionCloseOnEOF, defaultCloseOnEOF)),

		id:     pipeID.NextID(),
		parent: parent,
		c:      tc,
		d:      d,
		l:      l,
	}
}

func (p *pipe) ID() uint32 {
	return p.id
}

func (p *pipe) LocalAddress() string {
	return p.c.LocalAddress()
}

func (p *pipe) RemoteAddress() string {
	return p.c.RemoteAddress()
}

func (p *pipe) IsRaw() bool {
	return p.c.IsRaw()
}

func (p *pipe) Close() {
	p.Lock()
	if p.closed {
		p.Unlock()
		return
	}
	p.closed = true
	p.Unlock()

	p.c.Close()
	p.parent.remPipe(p)

	pipeID.Recycle(p.id)

	return
}

func (p *pipe) Send(msg []byte, extras ...[]byte) (err error) {
	return p.SendTimeout(p.sendTimeout, msg, extras...)
}

func (p *pipe) SendTimeout(timeout time.Duration, msg []byte, extras ...[]byte) (err error) {
	if timeout <= 0 {
		if err = p.c.Send(msg, extras...); err != nil {
			// NOTE: close on any error
			go p.Close()
		}
		return
	}

	tm := time.NewTimer(timeout)
	done := make(chan struct{})

	go func() {
		if err = p.c.Send(msg, extras...); err != nil {
			// NOTE: close on any error
			go p.Close()
		}
		done <- struct{}{}
	}()
	select {
	case <-tm.C:
		go p.Close()
		err = errs.ErrTimeout
	case <-done:
		tm.Stop()
	}
	return
}

func (p *pipe) Recv() (msg []byte, err error) {
	return p.RecvTimeout(p.recvTimeout)
}

func (p *pipe) RecvTimeout(timeout time.Duration) (msg []byte, err error) {
	if timeout <= 0 {
		if msg, err = p.c.Recv(); err != nil {
			// NOTE: close on any error except EOF
			fmt.Fprintf(os.Stderr, "err: %s, %d\n", err, len(msg))
			if err == io.EOF {
				if len(msg) > 0 {
					err = nil
				} else if p.closeOnEOF {
					go p.Close()
				}
			} else {
				go p.Close()
			}
		}
		return
	}

	tm := time.NewTimer(timeout)
	done := make(chan struct{})
	go func() {
		if msg, err = p.c.Recv(); err != nil {
			// NOTE: close on any error
			if err == io.EOF {
				if len(msg) > 0 {
					err = nil
				} else if p.closeOnEOF {
					go p.Close()
				}
			} else {
				go p.Close()
			}
		}
		done <- struct{}{}
	}()

	select {
	case <-tm.C:
		go p.Close()
		err = errs.ErrTimeout
	case <-done:
		tm.Stop()
	}
	return
}

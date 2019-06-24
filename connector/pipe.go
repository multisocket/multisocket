package connector

import (
	"io"
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
	return &pipe{
		sendTimeout: Options.Pipe.SendTimeout.ValueFrom(opts, parent.Options),
		recvTimeout: Options.Pipe.RecvTimeout.ValueFrom(opts, parent.Options),
		closeOnEOF:  Options.Pipe.CloseOnEOF.ValueFrom(opts, parent.Options),

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

func (p *pipe) Close() error {
	p.Lock()
	if p.closed {
		p.Unlock()
		return errs.ErrClosed
	}
	p.closed = true
	p.Unlock()

	p.c.Close()
	p.parent.remPipe(p)

	pipeID.Recycle(p.id)

	return nil
}

func (p *pipe) Send(msg []byte, extras ...[]byte) (err error) {
	return p.SendTimeout(p.sendTimeout, msg, extras...)
}

func (p *pipe) SendTimeout(timeout time.Duration, msg []byte, extras ...[]byte) (err error) {
	if timeout <= 0 {
		if err = p.c.Send(msg, extras...); err != nil {
			// NOTE: close on any error
			if errx := p.Close(); errx != nil {
				err = errx
			}
		}
		return
	}

	tm := time.NewTimer(timeout)
	done := make(chan struct{})

	go func() {
		err = p.SendTimeout(0, msg, extras...)
		done <- struct{}{}
	}()
	select {
	case <-tm.C:
		p.Close()
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
			if err == io.EOF {
				if len(msg) > 0 {
					err = nil
				} else if p.closeOnEOF {
					p.Close()
					err = errs.ErrClosed
				}
			} else {
				if errx := p.Close(); errx != nil {
					err = errx
				}
			}
		}
		return
	}

	tm := time.NewTimer(timeout)
	done := make(chan struct{})
	go func() {
		msg, err = p.RecvTimeout(0)
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

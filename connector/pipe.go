package connector

import (
	"time"

	"github.com/webee/multisocket/errs"
	"github.com/webee/multisocket/transport"
	"github.com/webee/multisocket/utils"
)

// pipe wraps the transport.Connection data structure with the stuff we need to keep.
// It implements the Pipe interface.
type pipe struct {
	sendTimeout time.Duration
	recvTimeout time.Duration

	closedq chan struct{}
	id      uint32
	parent  *connector
	c       transport.Connection
	l       *listener
	d       *dialer
}

var pipeID = utils.NewRecyclableIDGenerator()

func newPipe(parent *connector, tc transport.Connection, d *dialer, l *listener) *pipe {
	return &pipe{
		sendTimeout: PipeOptionSendTimeout.Value(parent.GetOptionDefault(PipeOptionSendTimeout, time.Duration(0))),
		recvTimeout: PipeOptionRecvTimeout.Value(parent.GetOptionDefault(PipeOptionRecvTimeout, time.Duration(0))),
		closedq:     make(chan struct{}),

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
	select {
	case <-p.closedq:
		return
	default:
		close(p.closedq)
	}

	p.c.Close()
	p.parent.remPipe(p)

	// This is last, as we keep the ID reserved until everything is
	// done with it.
	pipeID.Recycle(p.id)

	return
}

func (p *pipe) Send(msg []byte, extras ...[]byte) (err error) {
	return p.SendTimeout(p.sendTimeout, msg, extras...)
}

func (p *pipe) SendTimeout(timeout time.Duration, msg []byte, extras ...[]byte) (err error) {
	select {
	case <-p.closedq:
		err = errs.ErrClosed
		return
	default:
	}

	if timeout <= 0 {
		if err = p.c.Send(msg, extras...); err != nil {
			// NOTE: close on any error
			go p.Close()
			err = errs.ErrClosed
		}
		return
	}

	tm := time.NewTimer(timeout)
	done := make(chan struct{})

	go func() {
		if err = p.c.Send(msg, extras...); err != nil {
			// NOTE: close on any error
			go p.Close()
			err = errs.ErrClosed
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
	select {
	case <-p.closedq:
		err = errs.ErrClosed
		return
	default:
	}

	if timeout <= 0 {
		if msg, err = p.c.Recv(); err != nil {
			// NOTE: close on any error
			go p.Close()
			// err = ErrClosed
		}
		return
	}

	tm := time.NewTimer(timeout)
	done := make(chan struct{})
	go func() {
		if msg, err = p.c.Recv(); err != nil {
			// NOTE: close on any error
			go p.Close()
			// err = ErrClosed
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

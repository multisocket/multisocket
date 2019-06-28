package connector

import (
	"io"
	"sync"

	"github.com/webee/multisocket/options"

	"github.com/webee/multisocket/errs"
	"github.com/webee/multisocket/transport"
	"github.com/webee/multisocket/utils"
)

// pipe wraps the transport.Connection data structure with the stuff we need to keep.
// It implements the Pipe interface.
type pipe struct {
	options.Options
	transport.Connection
	closeOnEOF bool
	raw        bool
	id         uint32
	parent     *connector
	d          *dialer
	l          *listener

	sync.Mutex
	closed bool
}

var pipeID = utils.NewRecyclableIDGenerator()

func newPipe(parent *connector, tc transport.Connection, d *dialer, l *listener, opts options.Options) *pipe {
	pipeOpts := options.NewOptionsWithValues(options.OptionValues{
		Options.Pipe.CloseOnEOF:     Options.Pipe.CloseOnEOF.ValueFrom(opts, parent.Options),
		Options.Pipe.RawMode:        Options.Pipe.RawMode.ValueFrom(opts, parent.Options),
		Options.Pipe.RawRecvBufSize: Options.Pipe.RawRecvBufSize.ValueFrom(opts, parent.Options),
	})
	return &pipe{
		Options:    pipeOpts,
		Connection: tc,
		closeOnEOF: Options.Pipe.CloseOnEOF.ValueFrom(pipeOpts),
		raw:        Options.Pipe.RawMode.ValueFrom(pipeOpts),

		id:     pipeID.NextID(),
		parent: parent,
		d:      d,
		l:      l,
	}
}

func (p *pipe) ID() uint32 {
	return p.id
}

func (p *pipe) IsRaw() bool {
	return p.raw
}

func (p *pipe) Close() error {
	p.Lock()
	if p.closed {
		p.Unlock()
		return errs.ErrClosed
	}
	p.closed = true
	p.Unlock()

	p.Connection.Close()
	p.parent.remPipe(p)

	pipeID.Recycle(p.id)

	return nil
}

func (p *pipe) Read(b []byte) (n int, err error) {
	if n, err = p.Connection.Read(b); err != nil {
		if err == io.EOF {
			if n > 0 {
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

func (p *pipe) Write(b []byte) (n int, err error) {
	if n, err = p.Connection.Write(b); err != nil {
		if errx := p.Close(); errx != nil {
			err = errx
		}
	}
	return
}

func (p *pipe) Writev(v *[][]byte) (n int64, err error) {
	if n, err = p.Connection.Writev(v); err != nil {
		if errx := p.Close(); errx != nil {
			err = errx
		}
	}
	return
}

package iopipe

import (
	"io"
	"net"
	"time"

	"bufio"

	"github.com/webee/multisocket/errs"
	"github.com/webee/multisocket/options"
	"github.com/webee/multisocket/transport"
	"github.com/webee/multisocket/transport/inproc"
)

type (
	pipe struct {
		r *io.PipeReader
		io.Reader
		*io.PipeWriter
	}
)

var (
	// Transport is inproc transport based on io.Pipe
	Transport = inproc.NewTransport("inproc.iopipe", newPipe)
)

func init() {
	transport.RegisterTransport(Transport)
}

func newPipe(opts options.Options) (net.Conn, net.Conn) {
	lpr, rpw := io.Pipe()
	rpr, lpw := io.Pipe()
	readBufSize := opts.GetOptionDefault(inproc.Options.ReadBuffer).(int)
	return &pipe{
			r:          lpr,
			Reader:     bufio.NewReaderSize(lpr, readBufSize),
			PipeWriter: lpw,
		}, &pipe{
			r:          rpr,
			Reader:     bufio.NewReaderSize(rpr, readBufSize),
			PipeWriter: rpw,
		}
}

// pipe

func (p *pipe) Close() error {
	p.r.Close()
	p.PipeWriter.Close()
	return nil
}

func (p *pipe) LocalAddr() net.Addr {
	return nil
}

func (p *pipe) RemoteAddr() net.Addr {
	return nil
}

func (p *pipe) SetDeadline(t time.Time) error {
	return errs.ErrOperationNotSupported
}

func (p *pipe) SetReadDeadline(t time.Time) error {
	return errs.ErrOperationNotSupported
}

func (p *pipe) SetWriteDeadline(t time.Time) error {
	return errs.ErrOperationNotSupported
}

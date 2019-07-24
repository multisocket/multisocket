package iopipe

import (
	"io"
	"net"
	"time"

	"bufio"

	"github.com/multisocket/multisocket/errs"
	"github.com/multisocket/multisocket/options"
	"github.com/multisocket/multisocket/transport"
	"github.com/multisocket/multisocket/transport/inproc"
)

type (
	pipe struct {
		r *io.PipeReader
		io.Reader
		*io.PipeWriter
		laddr net.Addr
		raddr net.Addr
	}
)

var (
	// Transport is inproc transport based on io.Pipe
	Transport = inproc.NewTransport("inproc.iopipe", newPipe)
)

func init() {
	transport.RegisterTransport(Transport)
}

func newPipe(laddr, raddr net.Addr, opts options.Options) (net.Conn, net.Conn) {
	lpr, rpw := io.Pipe()
	rpr, lpw := io.Pipe()
	readBufSize := opts.GetOptionDefault(inproc.Options.ReadBuffer).(int)
	return &pipe{
			r:          lpr,
			Reader:     bufio.NewReaderSize(lpr, readBufSize),
			PipeWriter: lpw,
			laddr:      laddr,
			raddr:      raddr,
		}, &pipe{
			r:          rpr,
			Reader:     bufio.NewReaderSize(rpr, readBufSize),
			PipeWriter: rpw,
			laddr:      raddr,
			raddr:      laddr,
		}
}

// pipe

func (p *pipe) Close() error {
	p.r.Close()
	p.PipeWriter.Close()
	return nil
}

func (p *pipe) LocalAddr() net.Addr {
	return p.laddr
}

func (p *pipe) RemoteAddr() net.Addr {
	return p.raddr
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

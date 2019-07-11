package connector

import (
	"io"
	"sync"

	"github.com/webee/multisocket/message"
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
	closeOnEOF           bool
	raw                  bool
	maxRecvContentLength uint32
	id                   uint32
	parent               *connector
	d                    *dialer
	l                    *listener

	// for read message header
	headerBuf []byte
	// for recv raw message
	rawRecvBuf []byte

	sync.Mutex
	closed bool
}

var (
	pipeID = utils.NewRecyclableIDGenerator()
)

func newPipe(parent *connector, tc transport.Connection, d *dialer, l *listener, opts options.Options) *pipe {
	pipeOpts := options.NewOptionsWithValues(options.OptionValues{
		Options.Pipe.CloseOnEOF:           Options.Pipe.CloseOnEOF.ValueFrom(opts, parent.Options),
		Options.Pipe.Raw:                  Options.Pipe.Raw.ValueFrom(opts, parent.Options),
		Options.Pipe.RawRecvBufSize:       Options.Pipe.RawRecvBufSize.ValueFrom(opts, parent.Options),
		Options.Pipe.MaxRecvContentLength: Options.Pipe.MaxRecvContentLength.ValueFrom(opts, parent.Options),
	})
	p := &pipe{
		Options:              pipeOpts,
		Connection:           tc,
		closeOnEOF:           Options.Pipe.CloseOnEOF.ValueFrom(pipeOpts),
		raw:                  Options.Pipe.Raw.ValueFrom(pipeOpts),
		maxRecvContentLength: Options.Pipe.MaxRecvContentLength.ValueFrom(pipeOpts),

		id:     pipeID.NextID(),
		parent: parent,
		d:      d,
		l:      l,

		headerBuf: make([]byte, message.HeaderSize),
	}
	if p.raw {
		// alloc
		p.rawRecvBuf = make([]byte, p.GetOptionDefault(Options.Pipe.RawRecvBufSize).(int))
	}

	return p
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

func (p *pipe) Writev(v ...[]byte) (n int64, err error) {
	if n, err = p.Connection.Writev(v...); err != nil {
		if errx := p.Close(); errx != nil {
			err = errx
		}
	}
	return
}

func (p *pipe) SendMsg(msg *message.Message) (err error) {
	if p.raw {
		return p.sendRawMsg(msg)
	}
	return p.sendMsg(msg)
}

func (p *pipe) sendMsg(msg *message.Message) (err error) {
	if msg.Header.HasFlags(message.MsgFlagRaw) {
		// ignore raw messages. raw message is only for stream, forward raw message makes no sense,
		// raw connection can not reply to message source.
		return nil
	}

	// if zero copy {
	// 	_, err = p.Writev(msg.Encode(), msg.Content)
	// } else {
	_, err = p.Write(msg.Encode())
	// }
	return
}

func (p *pipe) sendRawMsg(msg *message.Message) (err error) {
	if msg.Header.HasAnyFlags() {
		// ignore none normal messages.
		return
	}

	_, err = p.Write(msg.Content)
	return
}

func (p *pipe) RecvMsg() (msg *message.Message, err error) {
	if p.raw {
		return p.recvRawMsg()
	}
	return p.recvMsg()
}

func (p *pipe) recvMsg() (msg *message.Message, err error) {
	return message.NewMessageFromReader(p.id, p, p.headerBuf, p.maxRecvContentLength)
}

func (p *pipe) recvRawMsg() (msg *message.Message, err error) {
	var n int
	if n, err = p.Read(p.rawRecvBuf); err != nil {
		if err == io.EOF {
			// use nil represents EOF
			msg = message.NewRawRecvMessage(p.id, nil)
		}
	} else {
		msg = message.NewRawRecvMessage(p.id, p.rawRecvBuf[:n])
	}
	return
}

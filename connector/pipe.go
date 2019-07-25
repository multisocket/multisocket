package connector

import (
	"io"
	"strings"
	"sync"

	"github.com/multisocket/multisocket/message"
	"github.com/multisocket/multisocket/options"

	"bufio"

	"github.com/multisocket/multisocket/errs"
	"github.com/multisocket/multisocket/transport"
	"github.com/multisocket/multisocket/utils"
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

	// Reader
	r io.Reader

	sr  SendReceiver
	msr MsgSendReceiver
	// funcs
	recvMsgFunc func() (msg *message.Message, err error)
	sendMsgFunc func(msg *message.Message) (err error)

	msgFreeLevel message.FreeLevel

	// for read message meta data
	metaBuf []byte
	// for recv raw message
	rawRecvBuf []byte

	sync.Mutex
	closed bool
}

var (
	pipeID = utils.NewRecyclableIDGenerator()
)

func newPipe(parent *connector, tc transport.Connection, d *dialer, l *listener, opts options.Options) *pipe {
	p := &pipe{
		Options:    opts,
		Connection: tc,
		closeOnEOF: Options.Pipe.CloseOnEOF.ValueFrom(opts),
		raw:        Options.Pipe.Raw.ValueFrom(opts),

		id:     pipeID.NextID(),
		parent: parent,
		d:      d,
		l:      l,

		// Reader
		r: tc,
	}
	readBuffer := opts.GetOptionDefault(Options.Pipe.ReadBuffer).(int)
	if readBuffer > 0 {
		p.r = bufio.NewReaderSize(tc, readBuffer)
	}

	p.msgFreeLevel = message.FreeAll
	if p.raw {
		if sr, ok := tc.RawConn().(SendReceiver); ok {
			p.sr = sr
			// funcs
			p.sendMsgFunc = p.sendBlockRawMsg
			p.recvMsgFunc = p.recvBlockRawMsg
		} else {
			// funcs
			p.sendMsgFunc = p.sendRawMsg
			p.recvMsgFunc = p.recvRawMsg
			// alloc
			p.rawRecvBuf = make([]byte, Options.Pipe.RawRecvBufSize.ValueFrom(opts))
		}
		if strings.HasPrefix(tc.Transport().Scheme(), "inproc.channel") {
			p.msgFreeLevel = message.FreeMsg
		}
	} else {
		// options
		p.maxRecvContentLength = Options.Pipe.MaxRecvContentLength.ValueFrom(opts)
		if msr, ok := tc.RawConn().(MsgSendReceiver); ok {
			p.msr = msr
			// funcs
			p.sendMsgFunc = msr.SendMsg
			p.recvMsgFunc = p.recvDirectMsg

			if strings.HasPrefix(tc.Transport().Scheme(), "inproc.channel") {
				p.msgFreeLevel = message.FreeNone
			}
		} else {
			if sr, ok := tc.RawConn().(SendReceiver); ok {
				p.sr = sr
				// funcs
				p.sendMsgFunc = p.sendBlockMsg
				p.recvMsgFunc = p.recvBlockMsg
			} else {
				// funcs
				p.sendMsgFunc = p.sendMsg
				p.recvMsgFunc = p.recvMsg
				// alloc
				p.metaBuf = make([]byte, message.MetaSize)
			}
			if strings.HasPrefix(tc.Transport().Scheme(), "inproc.channel") {
				p.msgFreeLevel = message.FreeMsg
			}
		}
	}

	return p
}

func (p *pipe) ID() uint32 {
	return p.id
}

func (p *pipe) IsRaw() bool {
	return p.raw
}

func (p *pipe) MsgFreeLevel() message.FreeLevel {
	return p.msgFreeLevel
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
	// if n, err = p.Connection.Read(b); err != nil {
	if n, err = p.r.Read(b); err != nil {
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

func (p *pipe) recv() (b []byte, err error) {
	if b, err = p.sr.Recv(); err != nil {
		if err == io.EOF {
			if len(b) > 0 {
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

func (p *pipe) send(b []byte) (err error) {
	if err = p.sr.Send(b); err != nil {
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
	return p.sendMsgFunc(msg)
}

func (p *pipe) sendMsg(msg *message.Message) (err error) {
	if msg.HasFlags(message.MsgFlagRaw) {
		// TODO: remove check, guaranteed by user
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

func (p *pipe) sendBlockMsg(msg *message.Message) (err error) {
	if msg.HasFlags(message.MsgFlagRaw) {
		// TODO: remove check, guaranteed by user
		// ignore raw messages. raw message is only for stream, forward raw message makes no sense,
		// raw connection can not reply to message source.
		return nil
	}

	return p.send(msg.Encode())
}

func (p *pipe) sendRawMsg(msg *message.Message) (err error) {
	if msg.HasAnyFlags() {
		// ignore none normal messages.
		return
	}

	_, err = p.Write(msg.Content)
	return
}

func (p *pipe) sendBlockRawMsg(msg *message.Message) (err error) {
	if msg.HasAnyFlags() {
		// ignore none normal messages.
		return
	}

	return p.send(msg.Content)
}

func (p *pipe) RecvMsg() (msg *message.Message, err error) {
	return p.recvMsgFunc()
}

func (p *pipe) recvMsg() (msg *message.Message, err error) {
	return message.NewMessageFromReader(p.id, p, p.metaBuf, p.maxRecvContentLength)
}

func (p *pipe) recvBlockMsg() (msg *message.Message, err error) {
	var buf []byte
	if buf, err = p.recv(); err != nil {
		return
	}
	return message.NewMessageFromBytes(p.id, buf, p.maxRecvContentLength)
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

func (p *pipe) recvBlockRawMsg() (msg *message.Message, err error) {
	var buf []byte
	if buf, err = p.recv(); err != nil {
		if err == io.EOF {
			// use nil represents EOF
			msg = message.NewRawRecvMessage(p.id, nil)
		}
	} else {
		msg = message.NewRawRecvMessage(p.id, buf)
	}
	return
}

func (p *pipe) recvDirectMsg() (msg *message.Message, err error) {
	var srcMsg *message.Message
	if srcMsg, err = p.msr.RecvMsg(); err != nil {
		return
	}
	return message.NewMessageFromMsg(p.id, srcMsg, p.maxRecvContentLength)
}

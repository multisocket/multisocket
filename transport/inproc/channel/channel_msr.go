package channel

import (
	"net"

	"github.com/multisocket/multisocket/errs"
	"github.com/multisocket/multisocket/message"
	"github.com/multisocket/multisocket/options"
	"github.com/multisocket/multisocket/transport"
	"github.com/multisocket/multisocket/transport/inproc"
)

type (
	msrPipe struct {
		*pipe
		lastMsg *message.Message
		recvq   <-chan *message.Message
		sendq   chan<- *message.Message
	}
)

var (
	// MsrTransport is inproc transport based on bytes channel, using MsgSendReceiver+ReadWriter
	MsrTransport = inproc.NewTransport("inproc.channel.msr", newMsrPipe)
)

func init() {
	transport.RegisterTransport(MsrTransport)
	// default inproc.channel
	transport.RegisterTransportWithScheme(MsrTransport, "inproc.channel")
}

func newMsrPipe(laddr, raddr net.Addr, opts options.Options) (net.Conn, net.Conn) {
	bufferSize := opts.GetOptionDefault(Options.BufferSize).(int)
	ma, mb := make(chan *message.Message, bufferSize), make(chan *message.Message, bufferSize)
	pa, pb := &msrPipe{
		sendq: ma,
		recvq: mb,
	}, &msrPipe{
		sendq: mb,
		recvq: ma,
	}
	pa.pipe, pb.pipe = createPipe(laddr, raddr, opts)

	return pa, pb
}

// MsgSendReceiver

func (p *msrPipe) SendMsg(msg *message.Message) (err error) {
	select {
	case <-p.closedq:
		err = errs.ErrClosed
	case p.sendq <- msg:
	}
	return
}

func (p *msrPipe) RecvMsg() (msg *message.Message, err error) {
	if p.lastMsg != nil {
		p.lastMsg.FreeAll()
		p.lastMsg = nil
	}
	select {
	case <-p.closedq:
		// exhaust received messages
		select {
		case msg = <-p.recvq:
		default:
			err = errs.ErrClosed
		}
	case msg = <-p.recvq:
	}
	p.lastMsg = msg
	return
}

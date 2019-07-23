package multisocket

import (
	"sync"

	"github.com/multisocket/multisocket/connector"
	"github.com/multisocket/multisocket/errs"
	"github.com/multisocket/multisocket/message"
	"github.com/multisocket/multisocket/options"
)

type (
	pairSocket struct {
		options.Options
		ConnectorAction // always nil, connector action is forbidden

		recvq chan *message.Message

		noSend     bool
		sendq      chan *message.Message
		ttl        uint8
		bestEffort bool

		lk      *sync.Mutex
		closedq chan struct{}

		peer *pairSocket
	}
)

// NewPair create a pair of Socket actions.
func NewPair() (Socket, Socket) {
	ca, cb := make(chan *message.Message), make(chan *message.Message)
	lk := &sync.Mutex{}
	closedq := make(chan struct{})
	sa, sb := newPairSocket(ca, cb, lk, closedq), newPairSocket(cb, ca, lk, closedq)
	sa.peer = sb
	sb.peer = sa

	return sa, sb
}

func newPairSocket(sendq, recvq chan *message.Message, lk *sync.Mutex, closedq chan struct{}) *pairSocket {
	s := &pairSocket{
		Options: options.NewOptions(),

		recvq: recvq,

		sendq: sendq,

		lk:      lk,
		closedq: closedq,
	}

	// init option values
	s.onOptionChange(Options.NoRecv, nil, nil)
	s.onOptionChange(Options.NoSend, nil, nil)
	s.onOptionChange(Options.SendTTL, nil, nil)
	s.onOptionChange(Options.SendBestEffort, nil, nil)

	s.Options.AddOptionChangeHook(s.onOptionChange)
	return s
}

// options

func (s *pairSocket) onOptionChange(opt options.Option, oldVal, newVal interface{}) error {
	switch opt {
	case Options.NoRecv:
		s.peer.onOptionChange(Options.NoSend, nil, nil)
	case Options.NoRecv:
		s.noSend = s.GetOptionDefault(Options.NoSend).(bool) || s.peer.GetOptionDefault(Options.NoRecv).(bool)
	case Options.SendTTL:
		s.ttl = s.GetOptionDefault(Options.SendTTL).(uint8)
	case Options.SendBestEffort:
		s.bestEffort = s.GetOptionDefault(Options.SendBestEffort).(bool)
	}
	return nil
}

func (s *pairSocket) RecvMsg() (msg *message.Message, err error) {
	select {
	case msg = <-s.recvq:
		return
	case <-s.closedq:
		err = errs.ErrClosed
		return
	}
}

func (s *pairSocket) SendMsg(msg *message.Message) error {
	if s.noSend {
		// drop msg
		msg.FreeAll()
		return nil
	}
	select {
	case s.sendq <- msg:
		return nil
	case <-s.closedq:
		return errs.ErrClosed
	}
}

func (s *pairSocket) Send(content []byte) error {
	if s.noSend {
		return nil
	}
	return s.SendMsg(message.NewSendMessage(0, message.SendTypeToOne, s.ttl, nil, nil, content))
}

func (s *pairSocket) SendAll(content []byte) error {
	if s.noSend {
		return nil
	}
	return s.SendMsg(message.NewSendMessage(0, message.SendTypeToAll, s.ttl, nil, nil, content))
}

func (s *pairSocket) SendTo(dest message.MsgPath, content []byte) error {
	if s.noSend {
		return nil
	}
	return s.SendMsg(message.NewSendMessage(0, message.SendTypeToDest, s.ttl, nil, dest, content))
}

// connector

func (s *pairSocket) Connector() connector.Connector {
	return nil
}

func (s *pairSocket) Close() error {
	s.lk.Lock()
	defer s.lk.Unlock()
	select {
	case <-s.closedq:
		return errs.ErrClosed
	default:
		close(s.closedq)
	}

	return nil
}

package multisocket

import (
	"sync"

	"github.com/multisocket/multisocket/connector"
	"github.com/multisocket/multisocket/errs"
	"github.com/multisocket/multisocket/message"
	"github.com/multisocket/multisocket/options"
	log "github.com/sirupsen/logrus"
)

type (
	socket struct {
		options.Options
		connector connector.Connector
		ConnectorAction

		sync.RWMutex
		closedq chan struct{}

		pipes map[uint32]*pipe

		// recv
		noRecv bool
		recvq  chan *message.Message
		// send
		*sender
	}

	pipe struct {
		connector.Pipe
		*pipeSender
	}
)

var (
	emptyByteSlice = make([]byte, 0)
)

// New creates a Socket
func New(ovs options.OptionValues) Socket {
	s := &socket{
		Options: options.NewOptionsWithValues(ovs),
		closedq: make(chan struct{}),
		pipes:   make(map[uint32]*pipe),
	}
	s.connector = connector.NewWithOptions(s.Options)
	s.ConnectorAction = s.connector
	s.sender = newSender(s)
	// init option values
	s.onOptionChange(Options.NoRecv, nil, nil)
	s.onOptionChange(Options.RecvQueueSize, nil, nil)

	s.Options.AddOptionChangeHook(s.onOptionChange)

	// set pipe event handler
	s.connector.SetPipeEventHandler(s.HandlePipeEvent)

	return s
}

// options

func (s *socket) onOptionChange(opt options.Option, oldVal, newVal interface{}) error {
	switch opt {
	case Options.NoRecv:
		s.noRecv = s.GetOptionDefault(Options.NoRecv).(bool)
	case Options.RecvQueueSize:
		s.recvq = make(chan *message.Message, s.recvQueueSize())
	default:
		return s.sender.onOptionChange(opt, oldVal, newVal)
	}
	return nil
}

func (s *socket) recvQueueSize() uint16 {
	return s.GetOptionDefault(Options.RecvQueueSize).(uint16)
}

func (s *socket) HandlePipeEvent(e connector.PipeEvent, pipe connector.Pipe) {
	switch e {
	case connector.PipeEventAdd:
		s.addPipe(pipe)
	case connector.PipeEventRemove:
		s.remPipe(pipe.ID())
	}
}

func (s *socket) addPipe(cp connector.Pipe) {
	s.Lock()
	p := &pipe{Pipe: cp}
	s.pipes[p.ID()] = p
	go s.receiver(p)
	s.sender.initPipe(p)
	go s.sender.run(p)
	s.Unlock()
}

func (s *socket) remPipe(id uint32) {
	s.Lock()
	p, ok := s.pipes[id]
	if !ok {
		s.Unlock()
		return
	}
	delete(s.pipes, id)
	s.Unlock()

	if s.sender != nil {
		s.sender.stopPipe(p)
	}
}

func (s *socket) getPipeByID(id uint32) *pipe {
	s.socket.RLock()
	p := s.socket.pipes[id]
	s.socket.RUnlock()
	return p
}

// recv

func (s *socket) RecvMsg() (msg *message.Message, err error) {
	select {
	case <-s.closedq:
		// exhaust received messages
		select {
		case msg = <-s.recvq:
		default:
			err = errs.ErrClosed
		}
	case msg = <-s.recvq:
	}
	return
}

func (s *socket) receiver(p *pipe) {
	if log.IsLevelEnabled(log.DebugLevel) {
		log.WithField("domain", "receiver").
			WithFields(log.Fields{"id": p.ID(), "raw": p.IsRaw()}).
			Debug("receiver start run")
	}

	var (
		err error
		msg *message.Message
	)

	if p.IsRaw() {
		// NOTE:
		// send a empty message to make a connection
		s.recvq <- message.NewRawRecvMessage(p.ID(), emptyByteSlice)
	}
RECVING:
	for {
		if msg, err = p.RecvMsg(); msg != nil {
			if s.noRecv {
				// just drop
				msg.FreeAll()
			} else if msg.HasFlags(message.MsgFlagInternal) {
				// FIXME: handle internal messages.
				msg.FreeAll()
			} else {
				select {
				case <-s.closedq:
					msg.FreeAll()
					s.remPipe(p.ID())
					break RECVING
				case s.recvq <- msg:
				}
			}
		}
		if err != nil {
			break RECVING
		}
	}

	if log.IsLevelEnabled(log.DebugLevel) {
		log.WithField("domain", "receiver").
			WithError(err).
			WithFields(log.Fields{"id": p.ID(), "raw": p.IsRaw()}).
			Debug("receiver stopped run")
	}
}

// connector

func (s *socket) Connector() connector.Connector {
	return s.connector
}

func (s *socket) Close() error {
	s.Lock()
	select {
	case <-s.closedq:
		s.Unlock()
		return errs.ErrClosed
	default:
		close(s.closedq)
	}
	s.Unlock()

	// clear pipe even handler
	s.connector.ClearPipeEventHandler(s.HandlePipeEvent)

	if s.sender != nil {
		s.sender.close()
	}
	s.connector.Close()

	return nil
}

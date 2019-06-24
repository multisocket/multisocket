package multisocket

import (
	"sync"

	"github.com/webee/multisocket/connector"
	"github.com/webee/multisocket/errs"
	"github.com/webee/multisocket/options"
	"github.com/webee/multisocket/receiver"
	"github.com/webee/multisocket/sender"
)

type socket struct {
	Connector
	Sender
	Receiver

	sync.Mutex
	closed bool
}

// NewDefault creates a default setting Socket
func NewDefault() (sock Socket) {
	return New(connector.New(), sender.New(), receiver.New())
}

// New creates a Socket
func New(connector Connector, tx Sender, rx Receiver) (sock Socket) {
	if rx == nil {
		// use receiver to check pipe closed
		rx = receiver.NewWithOptions(options.OptionValues{receiver.Options.NoRecv: true})
	}

	sock = &socket{
		Connector: connector,
		Sender:    tx,
		Receiver:  rx,
	}

	if tx != nil {
		tx.AttachConnector(connector)
	}
	rx.AttachConnector(connector)

	return
}

func (s *socket) GetConnector() Connector {
	return s.Connector
}

func (s *socket) GetSender() Sender {
	return s.Sender
}

func (s *socket) GetReceiver() Receiver {
	return s.Receiver
}

func (s *socket) Close() error {
	s.Lock()
	if s.closed {
		s.Unlock()
		return errs.ErrClosed
	}
	s.closed = true
	s.Unlock()

	s.Connector.Close()
	s.Receiver.Close()
	if s.Sender != nil {
		s.Sender.Close()
	}

	return nil
}

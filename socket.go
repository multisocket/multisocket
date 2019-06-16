package multisocket

import (
	"sync"

	"github.com/webee/multisocket/options"
	"github.com/webee/multisocket/receiver"

	"github.com/webee/multisocket/errs"
	. "github.com/webee/multisocket/types"
)

type socket struct {
	Connector
	Sender
	Receiver

	sync.Mutex
	closed bool
}

// New creates a Socket
func New(connector Connector, tx Sender, rx Receiver) (sock Socket) {
	if rx == nil {
		// use receiver to check pipe closed
		rx = receiver.NewWithOptions(options.NewOptionValue(receiver.OptionNoRecv, true))
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

func (s *socket) Close() error {
	s.Lock()
	if s.closed {
		s.Unlock()
		return errs.ErrClosed
	}
	s.closed = true
	s.Unlock()

	s.Connector.Close()
	if s.Sender != nil {
		s.Sender.Close()
	}
	if s.Receiver != nil {
		s.Receiver.Close()
	}

	return nil
}

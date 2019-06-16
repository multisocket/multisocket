package multisocket

import (
	"sync"
)

type socket struct {
	Connector
	Sender
	Receiver

	sync.Mutex
	closed bool
}

// New creates a Socket
func New(connector Connector, sender Sender, receiver Receiver) (sock Socket) {
	sock = &socket{
		Connector: connector,
		Sender:    sender,
		Receiver:  receiver,
	}

	if sender != nil {
		sender.AttachConnector(connector)
	}
	if receiver != nil {
		receiver.AttachConnector(connector)
	}
	return
}

func (s *socket) Close() error {
	s.Lock()
	if s.closed {
		s.Unlock()
		return ErrClosed
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

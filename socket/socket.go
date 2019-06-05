package socket

import (
	"sync"

	"github.com/webee/multisocket"
)

type socket struct {
	connector Connector
	sender    Sender
	receiver  Receiver

	sync.Mutex
	closed bool
}

// New creates a Socket
func New(connector Connector, sender Sender, receiver Receiver) Socket {
	sender.AttachConnector(connector)
	receiver.AttachConnector(connector)

	return &socket{
		connector: connector,
		sender:    sender,
		receiver:  receiver,
	}
}

func (s *socket) Dial(addr string) error {
	return s.connector.Dial(addr)
}

func (s *socket) DialOptions(addr string, opts multisocket.Options) error {
	return s.connector.DialOptions(addr, opts)
}

func (s *socket) NewDialer(addr string, opts multisocket.Options) (Dialer, error) {
	return s.connector.NewDialer(addr, opts)
}

func (s *socket) Listen(addr string) error {
	return s.connector.Listen(addr)
}

func (s *socket) ListenOptions(addr string, opts multisocket.Options) error {
	return s.connector.ListenOptions(addr, opts)
}

func (s *socket) NewListener(addr string, opts multisocket.Options) (Listener, error) {
	return s.connector.NewListener(addr, opts)
}

func (s *socket) Send(msg []byte) (PipeInfo, error) {
	return s.sender.Send(msg)
}

func (s *socket) SendTo(connInfo PipeInfo, msg []byte) error {
	return s.sender.SendTo(connInfo, msg)
}

func (s *socket) Recv() (PipeInfo, []byte, error) {
	return s.receiver.Recv()
}

func (s *socket) RecvFrom(connInfo PipeInfo) ([]byte, error) {
	return s.receiver.RecvFrom(connInfo)
}

func (s *socket) Close() error {
	s.Lock()
	if s.closed {
		s.Unlock()
		return multisocket.ErrClosed
	}
	s.closed = true
	s.Unlock()

	s.connector.Close()
	s.sender.Close()
	s.receiver.Close()

	return nil
}

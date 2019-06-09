package multisocket

import (
	"sync"

	"github.com/webee/multisocket/options"
)

type socket struct {
	connector Connector
	sender    Sender
	receiver  Receiver

	sync.Mutex
	attached bool
	closed   bool
}

// New creates a Socket
func New(connector Connector, sender Sender, receiver Receiver) Socket {
	return &socket{
		connector: connector,
		sender:    sender,
		receiver:  receiver,
	}
}

func (s *socket) Connector() Connector {
	return s.connector
}

func (s *socket) Sender() Sender {
	return s.sender
}

func (s *socket) Receiver() Receiver {
	return s.receiver
}

func (s *socket) attachConnector() {
	s.Lock()
	if s.attached {
		s.Unlock()
		return
	}
	s.sender.AttachConnector(s.connector)
	s.receiver.AttachConnector(s.connector)
	s.attached = true
}

func (s *socket) Dial(addr string) error {
	s.attachConnector()
	return s.connector.Dial(addr)
}

func (s *socket) DialOptions(addr string, opts options.Options) error {
	s.attachConnector()
	return s.connector.DialOptions(addr, opts)
}

func (s *socket) NewDialer(addr string, opts options.Options) (Dialer, error) {
	s.attachConnector()
	return s.connector.NewDialer(addr, opts)
}

func (s *socket) Listen(addr string) error {
	s.attachConnector()
	return s.connector.Listen(addr)
}

func (s *socket) ListenOptions(addr string, opts options.Options) error {
	s.attachConnector()
	return s.connector.ListenOptions(addr, opts)
}

func (s *socket) NewListener(addr string, opts options.Options) (Listener, error) {
	s.attachConnector()
	return s.connector.NewListener(addr, opts)
}

func (s *socket) SendTo(dest MsgPath, content []byte) error {
	return s.sender.SendTo(dest, content)
}

func (s *socket) Send(content []byte) error {
	return s.sender.Send(content)
}

func (s *socket) ForwardMsg(msg *Message) error {
	return s.sender.ForwardMsg(msg)
}

func (s *socket) RecvMsg() (*Message, error) {
	return s.receiver.RecvMsg()
}

func (s *socket) Recv() ([]byte, error) {
	return s.receiver.Recv()
}

func (s *socket) Close() error {
	s.Lock()
	if s.closed {
		s.Unlock()
		return ErrClosed
	}
	s.closed = true
	s.Unlock()

	s.connector.Close()
	s.sender.Close()
	s.receiver.Close()

	return nil
}

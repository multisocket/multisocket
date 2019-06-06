package socket

import (
	"sync"

	"github.com/webee/multisocket"
	"github.com/webee/multisocket/options"
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

func (s *socket) Connector() Connector {
	return s.connector
}

func (s *socket) Sender() Sender {
	return s.sender
}

func (s *socket) Receiver() Receiver {
	return s.receiver
}

func (s *socket) Dial(addr string) error {
	return s.connector.Dial(addr)
}

func (s *socket) DialOptions(addr string, opts options.Options) error {
	return s.connector.DialOptions(addr, opts)
}

func (s *socket) NewDialer(addr string, opts options.Options) (Dialer, error) {
	return s.connector.NewDialer(addr, opts)
}

func (s *socket) Listen(addr string) error {
	return s.connector.Listen(addr)
}

func (s *socket) ListenOptions(addr string, opts options.Options) error {
	return s.connector.ListenOptions(addr, opts)
}

func (s *socket) NewListener(addr string, opts options.Options) (Listener, error) {
	return s.connector.NewListener(addr, opts)
}

func (s *socket) SendTo(src MsgSource, content []byte) error {
	return s.sender.SendTo(src, content)
}

func (s *socket) SendMsg(msg *Message) error {
	return s.sender.SendMsg(msg)
}

func (s *socket) Send(content []byte) error {
	return s.sender.Send(content)
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
		return multisocket.ErrClosed
	}
	s.closed = true
	s.Unlock()

	s.connector.Close()
	s.sender.Close()
	s.receiver.Close()

	return nil
}

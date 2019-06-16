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
	closed bool
}

// New creates a Socket
func New(connector Connector, sender Sender, receiver Receiver) (sock Socket) {
	sock = &socket{
		connector: connector,
		sender:    sender,
		receiver:  receiver,
	}

	if sender != nil {
		sender.AttachConnector(connector)
	}
	if receiver != nil {
		receiver.AttachConnector(connector)
	}
	return
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

func (s *socket) SetNegotiator(negotiator Negotiator) {
	s.connector.SetNegotiator(negotiator)
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

func (s *socket) GetPipe(id uint32) Pipe {
	return s.connector.GetPipe(id)
}

func (s *socket) ClosePipe(id uint32) {
	s.connector.ClosePipe(id)
}

func (s *socket) SendTo(dest MsgPath, content []byte, extras ...[]byte) error {
	if s.sender == nil {
		return ErrOperationNotSupported
	}

	return s.sender.SendTo(dest, content, extras...)
}

func (s *socket) Send(content []byte, extras ...[]byte) error {
	if s.sender == nil {
		return ErrOperationNotSupported
	}

	return s.sender.Send(content, extras...)
}

func (s *socket) SendAll(content []byte, extras ...[]byte) error {
	if s.sender == nil {
		return ErrOperationNotSupported
	}

	return s.sender.SendAll(content, extras...)
}

func (s *socket) SendMsg(msg *Message) error {
	if s.sender == nil {
		return ErrOperationNotSupported
	}

	return s.sender.SendMsg(msg)
}

func (s *socket) RecvMsg() (*Message, error) {
	if s.receiver == nil {
		return nil, ErrOperationNotSupported
	}

	return s.receiver.RecvMsg()
}

func (s *socket) Recv() ([]byte, error) {
	if s.receiver == nil {
		return nil, ErrOperationNotSupported
	}

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
	if s.sender != nil {
		s.sender.Close()
	}
	if s.receiver != nil {
		s.receiver.Close()
	}

	return nil
}

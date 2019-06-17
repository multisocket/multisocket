package pipeline

import (
	"github.com/webee/multisocket"
	"github.com/webee/multisocket/connector"
	"github.com/webee/multisocket/sender"
)

type (
	push struct {
		multisocket.Socket
	}
)

func NewPush() Push {
	return &push{
		Socket: multisocket.New(connector.New(), sender.New(), nil),
	}
}

func (p *push) GetSocket() multisocket.Socket {
	return p.Socket
}

func (p *push) Send(content []byte) error {
	return p.Socket.Send(content)
}

func (p *push) Close() error {
	return p.Socket.Close()
}

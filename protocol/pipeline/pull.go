package pipeline

import (
	"github.com/webee/multisocket"
	"github.com/webee/multisocket/connector"
	"github.com/webee/multisocket/receiver"
)

type (
	pull struct {
		multisocket.Socket
	}
)

func NewPull() Pull {
	return &pull{
		Socket: multisocket.New(connector.New(), nil, receiver.New()),
	}
}

func (p *pull) GetSocket() multisocket.Socket {
	return p.Socket
}

func (p *pull) Recv() ([]byte, error) {
	return p.Socket.Recv()
}

func (p *pull) Close() error {
	return p.Socket.Close()
}

package pipeline

import (
	"github.com/webee/multisocket"
	"github.com/webee/multisocket/connector"
)

type (
	// Push is the Pipeline producer
	Push interface {
		connector.ConnectorAction
		GetSocket() multisocket.Socket
		Close() error

		// actions
		Send([]byte) error
	}

	// Pull is the Pipeline consumer
	Pull interface {
		connector.ConnectorAction
		GetSocket() multisocket.Socket
		Close() error

		// actions
		Recv() ([]byte, error)
	}
)

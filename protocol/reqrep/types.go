package reqrep

import (
	"time"

	"github.com/webee/multisocket"

	"github.com/webee/multisocket/connector"
)

type (
	// Req is the Req protocol
	Req interface {
		connector.Action
		GetSocket() multisocket.Socket
		Close() error

		// actions
		Request(content []byte) ([]byte, error)
		ReqeustTimeout(t time.Duration, content []byte) ([]byte, error)
		ReqeustAsync(content []byte) *Request
	}

	// Rep is the Rep protocol
	Rep interface {
		connector.Action
		GetSocket() multisocket.Socket
		Close() error

		// actions
		SetRunner(runner Runner)
		Start()
	}

	// Request is a request object for Req
	Request struct {
		Cancel func()
		Err    error // request error, not business error
		Reply  []byte
		Done   chan *Request
	}

	// Handler is request handler for Rep
	Handler interface {
		Handle(req []byte) (rep []byte)
	}
	// Runner is handler runner for Rep
	Runner interface {
		Run(func())
	}
)

package reqrep

import (
	"time"

	. "github.com/webee/multisocket/types"
)

type (
	// Request is a request object
	Request struct {
		Cancel func()
		Err    error
		Reply  []byte
		Done   chan *Request
	}

	// Req is the Req protocol
	Req interface {
		ConnectorAction

		// actions
		Request(content []byte) ([]byte, error)
		ReqeustTimeout(t time.Duration, content []byte) ([]byte, error)
		ReqeustAsync(content []byte) *Request
		Close() error
	}

	// Handler is request handler
	Handler interface {
		Handle(req []byte) (rep []byte)
	}
	// Runner is handler runner
	Runner interface {
		Run(func())
	}

	// Rep is the Rep protocol
	Rep interface {
		ConnectorAction

		// actions
		SetRunner(runner Runner)
		Start()
		Close() error
	}
)

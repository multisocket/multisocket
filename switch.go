package multisocket

import (
	"github.com/multisocket/multisocket/message"
)

type (
	// SwitchMiddlewareFunc check or modidy switch messages
	SwitchMiddlewareFunc func(msg *message.Message) *message.Message
)

// StartSwitch start switch messages between back and front sockets
func StartSwitch(backSock, frontSock Socket, mid SwitchMiddlewareFunc) {
	go forward(backSock, frontSock, mid)
	go forward(frontSock, backSock, mid)
}

func forward(from Socket, to Socket, mid SwitchMiddlewareFunc) {
	var (
		err error
		msg *message.Message
	)
	if mid != nil {
		for {
			if msg, err = from.RecvMsg(); err != nil {
				break
			}

			if mid != nil {
				msg = mid(msg)
			}

			if err = to.SendMsg(msg); err != nil {
				break
			}
		}

	} else {
		for {
			if msg, err = from.RecvMsg(); err != nil {
				break
			}

			if err = to.SendMsg(msg); err != nil {
				break
			}
		}
	}
}

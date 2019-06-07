package sender

import (
	"github.com/webee/multisocket"
)

const (
	defaultMsgTTL = 16
)

// NewHeader create a message header.
func NewHeader() *multisocket.MsgHeader {
	return &multisocket.MsgHeader{TTL: defaultMsgTTL, Hops: 0}
}

// NewMessage create a message.
func NewMessage(src multisocket.MsgSource, content []byte) *multisocket.Message {
	return &multisocket.Message{
		Header:  NewHeader(),
		Source:  src,
		Content: content,
	}
}

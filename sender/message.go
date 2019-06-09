package sender

import (
	"github.com/webee/multisocket"
)

const (
	defaultMsgTTL      = 16
	defaultMsgDistance = 0xff
)

// NewHeader create a message header.
func NewHeader() *multisocket.MsgHeader {
	return &multisocket.MsgHeader{TTL: defaultMsgTTL, Hops: 0, Distance: defaultMsgDistance}
}

// NewMessage create a message.
func NewMessage(dest multisocket.MsgPath, content []byte) *multisocket.Message {
	header := NewHeader()
	if dest != nil {
		header.Distance = dest.Length()
	}
	return &multisocket.Message{
		BaseMessage: multisocket.BaseMessage{
			Header:      NewHeader(),
			Destination: dest,
		},
		Content: content,
	}
}

package sender

import (
	"github.com/webee/multisocket"
)

func newMessage(sendType uint8, ttl uint8, dest multisocket.MsgPath, content []byte, extras [][]byte) *multisocket.Message {
	header := &multisocket.MsgHeader{SendType: sendType, TTL: ttl, Hops: 0}
	if header.SendType == multisocket.SendTypeReply {
		header.Distance = dest.Length()
	}
	return &multisocket.Message{
		Header:      header,
		Destination: dest,
		Content:     content,
		Extras:      extras,
	}
}

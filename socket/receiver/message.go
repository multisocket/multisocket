package receiver

import (
	"bytes"
	"encoding/binary"

	"github.com/webee/multisocket/socket"
)

const (
	defaultMsgTTL = 16
)

// NewHeaderFromBytes create a msg header from bytes.
func NewHeaderFromBytes(payload []byte) (header *socket.MsgHeader, err error) {
	header = new(socket.MsgHeader)
	if len(payload) < header.Size() {
		err = ErrRecvInvalidData
		return
	}

	if err = binary.Read(bytes.NewReader(payload), binary.BigEndian, header); err != nil {
		return
	}
	return
}

// NewSourceFromBytes create a source from bytes.
func NewSourceFromBytes(n int, payload []byte) socket.MsgSource {
	return socket.MsgSource(append([]byte{}, payload[:4*n]...))
}

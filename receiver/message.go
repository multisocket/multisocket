package receiver

import (
	"bytes"
	"encoding/binary"

	"github.com/webee/multisocket"
)

const (
	defaultMsgTTL = 16
)

// NewHeaderFromBytes create a msg header from bytes.
func NewHeaderFromBytes(payload []byte) (header *multisocket.MsgHeader, err error) {
	header = new(multisocket.MsgHeader)
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
func NewSourceFromBytes(n int, payload []byte) multisocket.MsgSource {
	return multisocket.MsgSource(append([]byte{}, payload[:4*n]...))
}

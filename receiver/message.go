package receiver

import (
	"bytes"
	"encoding/binary"

	"github.com/webee/multisocket/message"
)

const (
	defaultMsgTTL = message.DefaultMsgTTL
)

func newHeaderFromBytes(payload []byte) (header *MsgHeader, err error) {
	header = new(MsgHeader)
	if len(payload) < header.Size() {
		err = ErrRecvInvalidData
		return
	}

	if err = binary.Read(bytes.NewReader(payload), binary.BigEndian, header); err != nil {
		return
	}
	return
}

func newPathFromBytes(n int, payload []byte) MsgPath {
	return MsgPath(append([]byte{}, payload[:4*n]...))
}

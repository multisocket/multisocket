package receiver

import (
	"bytes"
	"encoding/binary"

	"github.com/webee/multisocket"
)

func newHeaderFromBytes(payload []byte) (header *multisocket.MsgHeader, err error) {
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

func newPathFromBytes(n int, payload []byte) multisocket.MsgPath {
	return multisocket.MsgPath(append([]byte{}, payload[:4*n]...))
}

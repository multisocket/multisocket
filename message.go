package multisocket

import (
	"bytes"
	"encoding/binary"
	"unsafe"
)

type (
	// MsgHeader message meta data
	MsgHeader struct {
		TTL  uint8
		Hops uint8
	}

	// MsgSource is message's source composed of pipe ids(uint32) traceback path.
	MsgSource []byte

	// Message is message
	Message struct {
		Header  *MsgHeader
		Source  MsgSource
		Content []byte
	}
)

// Size get Header byte size.
func (h *MsgHeader) Size() int {
	return int(unsafe.Sizeof(h))
}

// Encode Header to bytes
func (h *MsgHeader) Encode() []byte {
	buf := bytes.NewBuffer(make([]byte, h.Size()))
	binary.Write(buf, binary.BigEndian, h)
	return buf.Bytes()
}

// Size get Source byte size
func (src MsgSource) Size() int {
	return len(src)
}

// Encode Source to bytes
func (src MsgSource) Encode() []byte {
	return src
}

// NewID add the new pipe id to head.
func (src MsgSource) NewID(id uint32) (source MsgSource) {
	source = append(make([]byte, 4), src...)
	binary.BigEndian.PutUint32(source, id)
	return
}

// NextID get source's next pipe id and remain source.
func (src MsgSource) NextID() (id uint32, source MsgSource, ok bool) {
	if len(src) < 4 {
		source = src
		return
	}
	id = binary.BigEndian.Uint32(src)
	source = src[4:]
	ok = true
	return
}

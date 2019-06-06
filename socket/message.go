package socket

import (
	"bytes"
	"encoding/binary"
	"errors"
	"unsafe"
)

type (
	// MsgHeader message meta data
	MsgHeader struct {
		TTL  uint8
		Hops uint8
	}

	// MsgSource is message's source composed of pipe ids(uint32).
	MsgSource []byte

	// Message is message
	Message struct {
		Header  *MsgHeader
		Source  MsgSource
		Content []byte
	}
)

const (
	defaultMsgTTL = 16
)

var (
	// DefaultHeader is default header when send a message.
	DefaultHeader = &MsgHeader{TTL: defaultMsgTTL, Hops: 0}
)

// errors
var (
	ErrInvalidData = errors.New("invalid data")
)

// NewHeaderFromBytes create a msg header from bytes.
func NewHeaderFromBytes(payload []byte) (header *MsgHeader, err error) {
	header = new(MsgHeader)
	if len(payload) < header.Size() {
		err = ErrInvalidData
		return
	}

	if err = binary.Read(bytes.NewReader(payload), binary.BigEndian, header); err != nil {
		return
	}
	return
}

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

// NewMessage create a message.
func NewMessage(src MsgSource, content []byte) *Message {
	return &Message{
		Header:  DefaultHeader,
		Source:  src,
		Content: content,
	}
}

// NewSourceFromBytes create a source from bytes.
func NewSourceFromBytes(n int, payload []byte) MsgSource {
	return MsgSource(append([]byte{}, payload[:4*n]...))
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

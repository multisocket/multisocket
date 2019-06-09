package multisocket

import (
	"bytes"
	"encoding/binary"
	"unsafe"
)

type (
	// MsgHeader message meta data
	MsgHeader struct {
		TTL      uint8 // time to live
		Hops     uint8 // node count from origin
		Distance uint8 // node count to destination, 0xff means initiative/forward send, [0,0xff) means reply send.
	}

	// MsgPath is message's path composed of pipe ids(uint32) traceback.
	MsgPath []byte

	// Message is a message
	Message struct {
		Header      *MsgHeader
		Source      MsgPath
		Destination MsgPath
		Content     []byte
	}
)

// Size get Header byte size.
func (h *MsgHeader) Size() int {
	return int(unsafe.Sizeof(*h))
}

// DestLength get distance length
func (h *MsgHeader) DestLength() int {
	if h.Distance == 0xff {
		return 0
	}
	return int(h.Distance)
}

// Encode Header to bytes
func (h *MsgHeader) Encode() []byte {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, h)
	return buf.Bytes()
}

// Size get Path byte size
func (src MsgPath) Size() int {
	return len(src)
}

// Length get Path length
func (src MsgPath) Length() uint8 {
	return uint8(len(src) / 4)
}

// Encode Source to bytes
func (src MsgPath) Encode() []byte {
	return src
}

// CurID get source's current pipe id.
func (src MsgPath) CurID() (id uint32, ok bool) {
	if len(src) < 4 {
		return
	}
	id = binary.BigEndian.Uint32(src)
	ok = true
	return
}

// NewID add the new pipe id to head.
func (src MsgPath) NewID(id uint32) (source MsgPath) {
	source = append(make([]byte, 4), src...)
	binary.BigEndian.PutUint32(source, id)
	return
}

// NextID get source's next pipe id and remain source.
func (src MsgPath) NextID() (id uint32, source MsgPath, ok bool) {
	if len(src) < 4 {
		source = src
		return
	}
	id = binary.BigEndian.Uint32(src)
	source = src[4:]
	ok = true
	return
}

// HasDestination check if msg has a destination
func (msg *Message) HasDestination() bool {
	return msg.Header.Distance != 0xff || msg.Destination.Length() > 0
}

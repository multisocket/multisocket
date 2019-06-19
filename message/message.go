package message

import (
	"bytes"
	"encoding/binary"
	"unsafe"
)

const (
	// DefaultMsgTTL is default msg ttl
	DefaultMsgTTL = uint8(16)
)

type (
	// MsgHeader message meta data
	MsgHeader struct {
		// Flags
		Flags    uint8 // 6:other flags|2:send type to/one,all,dest
		TTL      uint8 // time to live
		Hops     uint8 // node count from origin
		Distance uint8 // node count to destination
	}

	// MsgPath is message's path composed of pipe ids(uint32) traceback.
	MsgPath []byte

	// Message is a message
	Message struct {
		Header      *MsgHeader
		Source      MsgPath
		Destination MsgPath
		Content     []byte
		Extras      [][]byte
	}

	// TODO:
	// InternalMsg internal message content structure.
	InternalMsg struct {
		Type uint8
		// others
	}
)

const (
	sendTypeMask uint8 = 0x03
	flagsMask    uint8 = 0xff ^ sendTypeMask
)

// send types, low 2bits
const (
	// random select one pipe to send
	SendTypeToOne uint8 = iota & sendTypeMask
	// send to all pipes
	SendTypeToAll
	// send to a destination
	SendTypeToDest
)

// Msg Flags
const (
	// socket internal message, used by socket internal
	MsgFlagInternal uint8 = 1 << (iota + 2)
	// MsgFlagRaw is used to indicate the message is from a raw transport
	MsgFlagRaw
	// protocol control message, predefined flag, use by protocols implementations or others.
	MsgFlagControl
)

// TODO:
// Internal Messages
const (
	// close peer
	InternalMsgClosePeer uint8 = iota
)

// SendType get message's send type
func (h *MsgHeader) SendType() uint8 {
	return h.Flags & sendTypeMask
}

// HasFlags check if header has flags setted.
func (h *MsgHeader) HasFlags(flags uint8) bool {
	return h.Flags&flags == flags
}

// HasAnyFlags check if header has any flags setted.
func (h *MsgHeader) HasAnyFlags() bool {
	return h.Flags&flagsMask != 0
}

// ClearFlags clear flags
func (h *MsgHeader) ClearFlags(flags uint8) uint8 {
	return h.Flags & (flags ^ 0xff)
}

// Size get Header byte size.
func (h *MsgHeader) Size() int {
	return int(unsafe.Sizeof(*h))
}

// DestLength get distance length
func (h *MsgHeader) DestLength() int {
	return int(h.Distance)
}

// Encode Header to bytes
func (h *MsgHeader) Encode() []byte {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, h)
	return buf.Bytes()
}

// Size get Path byte size
func (path MsgPath) Size() int {
	return len(path)
}

// Length get Path length
func (path MsgPath) Length() uint8 {
	return uint8(len(path) / 4)
}

// Encode Source to bytes
func (path MsgPath) Encode() []byte {
	return path
}

// CurID get source's current pipe id.
func (path MsgPath) CurID() (id uint32, ok bool) {
	if len(path) < 4 {
		return
	}
	id = binary.BigEndian.Uint32(path)
	ok = true
	return
}

// AddID add the new pipe id to head.
func (path MsgPath) AddID(id uint32) (source MsgPath) {
	source = append(make([]byte, 4), path...)
	binary.BigEndian.PutUint32(source, id)
	return
}

// AddSource add the new pipe id to head.
func (path MsgPath) AddSource(id [4]byte) (source MsgPath) {
	source = append(id[:4], path...)
	return
}

// NextID get source's next pipe id and remain source.
func (path MsgPath) NextID() (id uint32, source MsgPath, ok bool) {
	if len(path) < 4 {
		source = path
		return
	}
	id = binary.BigEndian.Uint32(path)
	source = path[4:]
	ok = true
	return
}

// NewMessage create a message
func NewMessage(sendType uint8, dest MsgPath, flags uint8, content []byte, extras ...[]byte) *Message {
	header := &MsgHeader{Flags: flags | sendType, TTL: DefaultMsgTTL}
	if sendType == SendTypeToDest {
		header.Distance = dest.Length()
	}
	return &Message{
		Header:      header,
		Destination: dest,
		Content:     content,
		Extras:      extras,
	}
}

// Encode encode msg to bytes.
func (msg *Message) Encode() [][]byte {
	res := make([][]byte, 4+len(msg.Extras))
	res[0] = msg.Header.Encode()
	res[1] = msg.Source.Encode()
	res[2] = msg.Destination.Encode()
	res[3] = msg.Content
	for i := 4; i < len(res); i++ {
		res[i] = msg.Extras[i-4]
	}

	return res
}

// AddSource add pipe id to msg source
func (msg *Message) AddSource(id [4]byte) {
	msg.Source = msg.Source.AddSource(id)
	msg.Header.Hops++
}

// HasDestination check if msg has a destination
func (msg *Message) HasDestination() bool {
	return msg.Header.SendType() == SendTypeToDest || msg.Destination.Length() > 0
}

// PipeID get this message's source pipe id.
func (msg *Message) PipeID() (id uint32) {
	id, _ = msg.Source.CurID()
	return
}

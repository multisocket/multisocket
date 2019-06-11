package multisocket

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
		Type     uint8 // 6:control type|2:send type one, all, rely
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
)

const sendTypeMask uint8 = 0x03
const controlTypeMask uint8 = 0xff ^ sendTypeMask

// control types, high 6bits
const (
	ControlTypeNone uint8 = iota << 2
	ControlTypeClosePeer
)

// send types, low 2bits
const (
	// random select one pipe to send
	SendTypeToOne uint8 = iota & sendTypeMask
	// send to all pipes
	SendTypeToAll
	// reply to a source
	SendTypeReply
)

// SendType get message's send type
func (h *MsgHeader) SendType() uint8 {
	return h.Type & sendTypeMask
}

// ControlType get message's control type
func (h *MsgHeader) ControlType() uint8 {
	return h.Type & controlTypeMask
}

// IsControlMsg check if is control msg
func (h *MsgHeader) IsControlMsg() bool {
	return h.Type&controlTypeMask != ControlTypeNone
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

// AddID add the new pipe id to head.
func (src MsgPath) AddID(id uint32) (source MsgPath) {
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

// NewControlMessage create a control message
func NewControlMessage(controlType, sendType uint8, dest MsgPath) *Message {
	header := &MsgHeader{Type: controlType | sendType, TTL: DefaultMsgTTL}
	if sendType == SendTypeReply {
		header.Distance = dest.Length()
	}
	return &Message{
		Header:      header,
		Destination: dest,
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

// HasDestination check if msg has a destination
func (msg *Message) HasDestination() bool {
	return msg.Header.SendType() == SendTypeReply || msg.Destination.Length() > 0
}

// PipeID get this message's source pipe id.
func (msg *Message) PipeID() (id uint32) {
	id, _ = msg.Source.CurID()
	return
}

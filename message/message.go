package message

import (
	"encoding/binary"
	"io"
	"sync"
	"unsafe"

	"github.com/webee/multisocket/errs"

	"github.com/webee/multisocket/bytespool"
)

const (
	// DefaultMsgTTL is default msg ttl
	DefaultMsgTTL = uint8(16)
)

type (
	// MsgHeader message meta data
	MsgHeader struct {
		// Flags
		Flags    uint8  // 6:other flags|2:send type to/one,all,dest
		TTL      uint8  // time to live
		Hops     uint8  // node count from origin
		Distance uint8  // node count to destination
		Length   uint32 // content length
	}

	// MsgPath is message's path composed of pipe ids(uint32) traceback.
	MsgPath []byte

	// Message is a message
	Message struct {
		Header      MsgHeader
		buf         []byte
		Source      MsgPath
		Destination MsgPath
		Content     []byte
	}

	// TODO: use internal message

	// InternalMsg internal message content structure.
	InternalMsg struct {
		Type uint8
		// others
	}
)

var (
	// MsgHeaderSize is the MsgHeader's memory byte size.
	MsgHeaderSize = int(unsafe.Sizeof(MsgHeader{}))
	emptyHeader   = MsgHeader{
		TTL: DefaultMsgTTL,
	}
	msgPool = &sync.Pool{
		New: func() interface{} { return &Message{} },
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
	return MsgHeaderSize
}

// DestLength get distance length
func (h *MsgHeader) DestLength() int {
	return int(h.Distance)
}

// Encode header to bytes
func (h *MsgHeader) Encode() []byte {
	b := bytespool.Alloc(MsgHeaderSize)
	b[0] = h.Flags
	b[1] = h.TTL
	b[2] = h.Hops
	b[3] = h.Distance
	binary.BigEndian.PutUint32(b[4:], h.Length)

	return b
}

// decodeHeader from reader
func decodeHeader(r io.Reader, h *MsgHeader) (err error) {
	b := bytespool.Alloc(MsgHeaderSize)
	if _, err = io.ReadFull(r, b); err != nil {
		// free
		bytespool.Free(b)
		return
	}
	h.Flags = b[0]
	h.TTL = b[1]
	h.Hops = b[2]
	h.Distance = b[3]
	h.Length = binary.BigEndian.Uint32(b[4:])
	// free
	bytespool.Free(b)
	return nil
}

// Size get Path byte size
func (path MsgPath) Size() int {
	return len(path)
}

// Length get Path length
func (path MsgPath) Length() uint8 {
	return uint8(len(path) / 4)
}

// CurID get source's current pipe id.
func (path MsgPath) CurID() (id uint32, ok bool) {
	l := len(path)
	if l < 4 {
		return
	}
	id = binary.BigEndian.Uint32(path[l-4:])
	ok = true
	return
}

// AddSource add the new pipe id to tail.
func (path MsgPath) AddSource(id [4]byte) MsgPath {
	return append(path, id[:4]...)
}

// NextID get source's next pipe id and remain source.
func (path MsgPath) NextID() (id uint32, source MsgPath, ok bool) {
	l := len(path)
	if l < 4 {
		source = path
		return
	}
	id = binary.BigEndian.Uint32(path[l-4:])
	source = path[:l-4]
	ok = true
	return
}

// NewMessageFromReader create a message from reader
func NewMessageFromReader(pid uint32, r io.Reader, maxLength uint32) (msg *Message, err error) {
	var (
		header     *MsgHeader
		sourceSize int
		destSize   int
		length     int
	)
	msg = msgPool.Get().(*Message)
	header = &msg.Header

	if err = decodeHeader(r, header); err != nil {
		msg.FreeAll()
		msg = nil
		// err = errs.ErrBadMsg
		return
	}
	if maxLength != 0 && header.Length > maxLength {
		msg.FreeAll()
		msg = nil
		err = errs.ErrContentTooLong
		return
	}

	sendType := header.SendType()

	sourceSize = 4 * int(header.Hops)
	destSize = 4 * int(header.Distance)
	length = int(header.Length)
	msg.buf = bytespool.Alloc(sourceSize + 4 + destSize + length)
	// Source
	msg.Source = msg.buf[: sourceSize+4 : sourceSize+4]
	if _, err = io.ReadFull(r, msg.Source[:sourceSize]); err != nil {
		msg.FreeAll()
		msg = nil
		return
	}
	// update source, add current pipe id
	binary.BigEndian.PutUint32(msg.Source[sourceSize:], pid)
	header.Hops++
	header.TTL--

	if sendType == SendTypeToDest && header.Distance > 0 {
		destSize = 4 * int(header.Distance)
		if destSize > 0 {
			msg.Destination = msg.buf[sourceSize+4 : sourceSize+4+destSize : sourceSize+4+destSize]
			if _, err = io.ReadFull(r, msg.Destination); err != nil {
				msg.FreeAll()
				msg = nil
				return
			}
		}
	}

	msg.Content = msg.buf[sourceSize+4+destSize : sourceSize+4+destSize+length : sourceSize+4+destSize+length]
	if _, err = io.ReadFull(r, msg.Content); err != nil {
		msg.FreeAll()
		msg = nil
		return
	}

	return
}

// NewRecvMessage create a new recv message
func NewRecvMessage(pid uint32, sendType uint8, dest MsgPath, flags uint8, ttl uint8, content []byte) *Message {
	if ttl <= 0 {
		ttl = DefaultMsgTTL
	}
	msg := msgPool.Get().(*Message)
	msg.Header = MsgHeader{
		Flags:    flags | sendType,
		TTL:      ttl,
		Distance: dest.Length(),
		Length:   uint32(len(content)),
	}
	header := &msg.Header

	sourceSize := 4
	destSize := len(dest)
	length := len(content)

	msg.buf = bytespool.Alloc(sourceSize + destSize + length)
	// Source
	msg.Source = msg.buf[:sourceSize:sourceSize]
	// update source, add current pipe id
	binary.BigEndian.PutUint32(msg.Source, pid)
	header.Hops++
	header.TTL--

	if sendType == SendTypeToDest && header.Distance > 0 {
		msg.Destination = msg.buf[sourceSize : sourceSize+destSize : sourceSize+destSize]
		copy(msg.Destination, dest)
	}

	if content != nil {
		// nil raw message is EOF
		msg.Content = msg.buf[sourceSize+destSize : sourceSize+destSize+length : sourceSize+destSize+length]
		copy(msg.Content, content)
	}

	return msg
}

// NewMessage create a message
func NewMessage(sendType uint8, dest MsgPath, flags uint8, ttl uint8, content []byte) *Message {
	if ttl <= 0 {
		ttl = DefaultMsgTTL
	}
	msg := msgPool.Get().(*Message)
	msg.Header = MsgHeader{
		Flags:    flags | sendType,
		TTL:      ttl,
		Distance: dest.Length(),
		Length:   uint32(len(content)),
	}

	destSize := len(dest)
	length := len(content)

	msg.buf = bytespool.Alloc(destSize + length)

	if sendType == SendTypeToDest && msg.Header.Distance > 0 {
		msg.Destination = msg.buf[:destSize:destSize]
		copy(msg.Destination, dest)
	}

	msg.Content = msg.buf[destSize : destSize+length : destSize+length]
	copy(msg.Content, content)

	return msg
}

// Dup create a duplicated message
// TODO: try effective way, like reference counting.
func (msg *Message) Dup() (dup *Message) {
	dup = msgPool.Get().(*Message)
	dup.Header = msg.Header

	dup.buf = bytespool.Alloc(cap(msg.buf))
	copy(dup.buf, msg.buf)
	sourceSize := 4 * int(dup.Header.Hops)
	destSize := 4 * int(dup.Header.Distance)
	length := int(dup.Header.Length)

	dup.Source = dup.buf[:sourceSize:sourceSize]
	dup.Destination = dup.buf[sourceSize : sourceSize+destSize : destSize]
	dup.Content = dup.buf[sourceSize+destSize : sourceSize+destSize+length : length]

	return dup
}

// FreeAll put msg and buf to pools
func (msg *Message) FreeAll() {
	bytespool.Free(msg.buf)

	msg.buf = nil
	msg.Header = emptyHeader
	msg.Source = nil
	msg.Destination = nil
	msg.Content = nil
	msgPool.Put(msg)
}

// AddSource add pipe id to msg source
func (msg *Message) AddSource(id [4]byte) {
	msg.Source = msg.Source.AddSource(id)
	msg.Header.Hops++
}

// PipeID get this message's source pipe id.
func (msg *Message) PipeID() (id uint32) {
	id, _ = msg.Source.CurID()
	return
}

package message

import (
	"encoding/binary"
	"io"
	"io/ioutil"
	"sync"

	"github.com/multisocket/multisocket/errs"

	"github.com/multisocket/multisocket/bytespool"
)

const (
	// DefaultMsgTTL is default msg ttl
	DefaultMsgTTL = uint8(16)
)

type (
	// Meta message's meta data
	Meta struct {
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
		buf []byte // decode/encode buffer
		Meta
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

// MetaSize is the Meta data's memory byte size.
// TODO: update when Meta modifed
const MetaSize = 8

var (
	emptyMeta = Meta{
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
	// TODO:
	// MsgFlagNoSource, do not record message source
	MsgFlagNoSource
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
func (m *Meta) SendType() uint8 {
	return m.Flags & sendTypeMask
}

// HasFlags check if meta data has flags setted.
func (m *Meta) HasFlags(flags uint8) bool {
	return m.Flags&flags == flags
}

// HasAnyFlags check if meta data has any flags setted.
func (m *Meta) HasAnyFlags() bool {
	return m.Flags&flagsMask != 0
}

// ClearFlags clear flags
func (m *Meta) ClearFlags(flags uint8) uint8 {
	return m.Flags & (flags ^ 0xff)
}

// encodeTo encode meta data to bytes
func (m *Meta) encodeTo(b []byte) []byte {
	b[0] = m.Flags
	b[1] = m.TTL
	b[2] = m.Hops
	b[3] = m.Distance
	binary.BigEndian.PutUint32(b[4:], m.Length)

	return b
}

// decodeMetaFrom reader
func decodeMetaFrom(r io.Reader, a []byte, m *Meta) (err error) {
	if _, err = io.ReadFull(r, a); err != nil {
		return
	}
	m.Flags = a[0]
	m.TTL = a[1]
	m.Hops = a[2]
	m.Distance = a[3]
	m.Length = binary.BigEndian.Uint32(a[4:])
	return nil
}

// Length get Path length
func (path MsgPath) Length() uint8 {
	return uint8(len(path) / 4)
}

// CurID get source's current pipe id.
func (path MsgPath) CurID() uint32 {
	l := len(path)
	if l < 4 {
		return 0
	}
	return binary.BigEndian.Uint32(path[l-4:])
}

// NextID get source's next pipe id and remain source.
func (path MsgPath) NextID() (id uint32, source MsgPath) {
	l := len(path)
	id = binary.BigEndian.Uint32(path[l-4:])
	source = path[:l-4]
	return
}

// NewMessageFromReader create a message from reader
func NewMessageFromReader(pid uint32, r io.ReadCloser, metaBuf []byte, maxLength uint32) (msg *Message, err error) {
	var (
		meta       *Meta
		from, to   int
		sourceSize int
		destSize   int
		length     int
	)
	msg = msgPool.Get().(*Message)
	meta = &msg.Meta

	if err = decodeMetaFrom(r, metaBuf, meta); err != nil {
		msg.FreeAll()
		msg = nil
		// err = errs.ErrBadMsg
		return
	}
	if maxLength != 0 && meta.Length > maxLength {
		msg.FreeAll()
		msg = nil
		r.Close()
		err = errs.ErrContentTooLong
		return
	}

	sourceSize = 4 * int(meta.Hops+1)
	if meta.Distance > 0 {
		destSize = 4 * int(meta.Distance-1)
	}
	length = int(meta.Length)
	msg.buf = bytespool.Alloc(MetaSize + sourceSize + destSize + length)
	// Source
	from = MetaSize
	to = from + sourceSize
	msg.Source = msg.buf[from:to:to]
	if _, err = io.ReadFull(r, msg.Source[:sourceSize-4]); err != nil {
		msg.FreeAll()
		msg = nil
		return
	}
	// update source, add current pipe id
	binary.BigEndian.PutUint32(msg.Source[sourceSize-4:], pid)
	meta.Hops++
	meta.TTL--

	// Destination
	if meta.Distance > 0 {
		if destSize > 0 {
			from = to
			to = from + destSize
			msg.Destination = msg.buf[from:to:to]
			if _, err = io.ReadFull(r, msg.Destination); err != nil {
				msg.FreeAll()
				msg = nil
				return
			}
		}
		// previous node's sender's pipe id
		if _, err = io.CopyN(ioutil.Discard, r, 4); err != nil {
			msg.FreeAll()
			msg = nil
			return
		}
		meta.Distance--
	}

	// Content
	from = to
	to = from + length
	msg.Content = msg.buf[from:to:to]
	if _, err = io.ReadFull(r, msg.Content); err != nil {
		msg.FreeAll()
		msg = nil
		return
	}

	return
}

// NewRawRecvMessage create a new raw recv message
func NewRawRecvMessage(pid uint32, content []byte) (msg *Message) {
	var (
		meta     *Meta
		from, to   int
		sourceSize int
		length     int
	)
	// raw message is always send to one.
	msg = msgPool.Get().(*Message)
	msg.Meta = Meta{
		Flags:    MsgFlagRaw | SendTypeToOne,
		TTL:      DefaultMsgTTL,
		Distance: 0,
		Length:   uint32(len(content)),
	}
	meta = &msg.Meta

	sourceSize = 4
	length = len(content)

	msg.buf = bytespool.Alloc(MetaSize + sourceSize + length)

	// Source
	from = MetaSize
	to = from + sourceSize
	msg.Source = msg.buf[from:to:to]
	// update source, add current pipe id
	binary.BigEndian.PutUint32(msg.Source, pid)
	meta.Hops++
	meta.TTL--

	if content != nil {
		// nil raw message is EOF
		from = to
		to = from + length
		msg.Content = msg.buf[from:to:to]
		copy(msg.Content, content)
	}

	return
}

// NewSendMessage create a message to send
func NewSendMessage(sendType uint8, src, dest MsgPath, flags uint8, ttl uint8, content []byte) *Message {
	var (
		from, to   int
		sourceSize int
		destSize   int
		length     int
	)

	if ttl <= 0 {
		ttl = DefaultMsgTTL
	}
	msg := msgPool.Get().(*Message)
	msg.Meta = Meta{
		Flags:    flags | sendType,
		TTL:      ttl,
		Hops:     src.Length(),
		Distance: dest.Length(),
		Length:   uint32(len(content)),
	}

	sourceSize = len(src)
	destSize = len(dest)
	length = len(content)
	// if zero copy {
	// 	length = 0
	// }

	msg.buf = bytespool.Alloc(MetaSize + sourceSize + destSize + length)
	to = MetaSize

	// Source
	if sourceSize > 0 {
		from = to
		to = from + sourceSize
		msg.Source = msg.buf[from:to:to]
		copy(msg.Source, src)
	}

	// Destination
	if msg.Meta.Distance > 0 {
		from = to
		to = from + destSize
		msg.Destination = msg.buf[from:to:to]
		copy(msg.Destination, dest)
	}

	// Content
	// if zero copy {
	// 	msg.Content = content
	// } else {
	from = to
	to = from + length
	msg.Content = msg.buf[from:to:to]
	copy(msg.Content, content)
	// }

	return msg
}

// Encode encode msg'b body parts.
func (msg *Message) Encode() []byte {
	msg.Meta.encodeTo(msg.buf)
	return msg.buf
}

// Dup create a duplicated message
// TODO: try effective way, like reference counting.
func (msg *Message) Dup() (dup *Message) {
	dup = msgPool.Get().(*Message)
	dup.Meta = msg.Meta

	dup.buf = bytespool.Alloc(len(msg.buf))
	copy(dup.buf, msg.buf)
	sourceSize := 4 * int(dup.Meta.Hops)
	destSize := 4 * int(dup.Meta.Distance)
	length := int(dup.Meta.Length)

	dup.Source = dup.buf[:sourceSize:sourceSize]
	dup.Destination = dup.buf[sourceSize : sourceSize+destSize : sourceSize+destSize]
	dup.Content = dup.buf[sourceSize+destSize : sourceSize+destSize+length : sourceSize+destSize+length]

	return dup
}

// FreeAll put buf and msg to pools
func (msg *Message) FreeAll() {
	bytespool.Free(msg.buf)

	msg.Free()
}

// Free put msg to pool
func (msg *Message) Free() {
	msg.buf = nil
	msg.Meta = emptyMeta
	msg.Source = nil
	msg.Destination = nil
	// if zero copy {
	// 	bytespool.Free(msg.Content)
	// }
	msg.Content = nil
	msgPool.Put(msg)
}

// PipeID get this message's source pipe id.
func (msg *Message) PipeID() uint32 {
	return msg.Source.CurID()
}

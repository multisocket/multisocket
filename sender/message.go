package sender

func newMessage(sendType uint8, ttl uint8, dest MsgPath, content []byte, extras [][]byte) *Message {
	header := &MsgHeader{Flags: sendType, TTL: ttl, Hops: 0}
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

package protocol

type Packet struct {
	SequenceNumber uint32
	Type           PacketType
	Payload        []byte
}

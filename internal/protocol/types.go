package protocol

type PacketType byte

const (
	Presence PacketType = iota + 1
	TransferRequest
	TransferAccept
	TransferReject
	Msg
	FileChunk
	FileEnd
	ACK
)

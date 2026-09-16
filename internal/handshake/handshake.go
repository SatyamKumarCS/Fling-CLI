package handshake

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/SatyamKumarCS/Fling-CLI/internal/protocol"
)

type TransferRequest struct {
	Filename string
	FileSize int64
	Checksum uint32
}

func CreateTransferRequest(
	filename string,
	fileSize int64,
	checksum uint32,
	sequenceNumber uint32,
) protocol.Packet {

	payload := fmt.Sprintf(
		"%s|%d|%d",
		filename,
		fileSize,
		checksum,
	)

	return protocol.Packet{
		SequenceNumber: sequenceNumber,
		Type:           protocol.TransferRequest,
		Payload:        []byte(payload),
	}
}

func ParseTransferRequest(
	packet protocol.Packet,
) (TransferRequest, error) {

	if packet.Type != protocol.TransferRequest {
		return TransferRequest{}, fmt.Errorf(
			"expected TRANSFER_REQUEST packet",
		)
	}

	parts := strings.Split(string(packet.Payload), "|")

	if len(parts) != 3 {
		return TransferRequest{}, fmt.Errorf(
			"invalid transfer request",
		)
	}

	fileSize, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return TransferRequest{}, fmt.Errorf(
			"invalid file size: %w",
			err,
		)
	}

	checksum, err := strconv.ParseUint(parts[2], 10, 32)
	if err != nil {
		return TransferRequest{}, fmt.Errorf(
			"invalid checksum: %w",
			err,
		)
	}

	return TransferRequest{
		Filename: parts[0],
		FileSize: fileSize,
		Checksum: uint32(checksum),
	}, nil
}

func CreateTransferAccept(sequenceNumber uint32) protocol.Packet {
	return protocol.Packet{
		SequenceNumber: sequenceNumber,
		Type:           protocol.TransferAccept,
		Payload:        nil,
	}
}

func CreateTransferReject(sequenceNumber uint32) protocol.Packet {
	return protocol.Packet{
		SequenceNumber: sequenceNumber,
		Type:           protocol.TransferReject,
		Payload:        nil,
	}
}

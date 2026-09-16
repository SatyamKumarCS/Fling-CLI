package transfer

import (
	"fmt"
	"net"
	"os"

	"github.com/SatyamKumarCS/Fling-CLI/internal/cli"
	"github.com/SatyamKumarCS/Fling-CLI/internal/network"
	"github.com/SatyamKumarCS/Fling-CLI/internal/protocol"
)

// SendFile reads the specified file, splits it into chunks, reliably transmits each FileChunk packet,
// reports transfer progress, and sends a FileEnd packet upon completion.
// Returns the next available sequence number.
func SendFile(
	conn *net.UDPConn,
	addr *net.UDPAddr,
	filePath string,
	startSequence uint32,
	progress cli.ProgressFunc,
) (uint32, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return startSequence, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	return SendBytes(conn, addr, data, startSequence, progress)
}

// SendBytes sends a byte slice as chunks reliably over UDP to the specified destination address.
func SendBytes(
	conn *net.UDPConn,
	addr *net.UDPAddr,
	data []byte,
	startSequence uint32,
	progress cli.ProgressFunc,
) (uint32, error) {
	totalBytes := int64(len(data))
	chunks := ChunkBytes(data, startSequence)

	var transferred int64
	currentSeq := startSequence

	for _, chunk := range chunks {
		err := network.SendReliable(conn, chunk, addr)
		if err != nil {
			return currentSeq, fmt.Errorf("failed to send chunk #%d: %w", chunk.SequenceNumber, err)
		}

		transferred += int64(len(chunk.Payload))
		if progress != nil {
			progress(transferred, totalBytes)
		}

		currentSeq++
	}

	// Send FileEnd packet to signal transfer completion
	fileEndPacket := protocol.Packet{
		SequenceNumber: currentSeq,
		Type:           protocol.FileEnd,
		Payload:        nil,
	}

	err := network.SendReliable(conn, fileEndPacket, addr)
	if err != nil {
		return currentSeq, fmt.Errorf("failed to send FileEnd packet #%d: %w", currentSeq, err)
	}

	currentSeq++

	return currentSeq, nil
}

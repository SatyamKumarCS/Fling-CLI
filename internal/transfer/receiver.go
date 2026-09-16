package transfer

import (
	"fmt"
	"net"
	"time"

	"github.com/SatyamKumarCS/Fling-CLI/internal/cli"
	"github.com/SatyamKumarCS/Fling-CLI/internal/network"
	"github.com/SatyamKumarCS/Fling-CLI/internal/protocol"
)

const DefaultReceiveTimeout = 5 * time.Second

// ReceiveFile listens for incoming FileChunk and FileEnd packets, ACKs each received packet,
// orders and deduplicates them, verifies size and CRC32 checksum, and saves the file to outputPath.
func ReceiveFile(
	conn *net.UDPConn,
	startSequence uint32,
	expectedSize int64,
	expectedChecksum uint32,
	outputPath string,
	progress cli.ProgressFunc,
) error {
	data, err := ReceiveBytes(conn, startSequence, expectedSize, expectedChecksum, progress)
	if err != nil {
		return err
	}

	reassembler := NewReassembler(expectedSize, expectedChecksum)
	reassembler.AddChunk(data)
	return reassembler.SaveToFile(outputPath)
}

// ReceiveBytes receives chunks from conn, ACKs them, orders and validates them,
// and returns the reassembled byte slice.
func ReceiveBytes(
	conn *net.UDPConn,
	startSequence uint32,
	expectedSize int64,
	expectedChecksum uint32,
	progress cli.ProgressFunc,
) ([]byte, error) {
	orderedReceiver := network.NewOrderedReceiver(startSequence)
	reassembler := NewReassembler(expectedSize, expectedChecksum)
	buffer := make([]byte, 2048)

	for {
		_ = conn.SetReadDeadline(time.Now().Add(DefaultReceiveTimeout))
		n, senderAddr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			return nil, fmt.Errorf("transfer receive timed out: %w", err)
		}

		packet, err := protocol.Decode(buffer[:n])
		if err != nil {
			// Corrupt packet or decoding error, ignore and let sender retry
			continue
		}

		// Immediately ACK the received packet so sender knows it arrived
		_ = network.SendACK(conn, packet.SequenceNumber, senderAddr)

		// Feed into OrderedReceiver to handle duplicates and out-of-order delivery
		deliveredPackets := orderedReceiver.Receive(packet)

		for _, delivered := range deliveredPackets {
			switch delivered.Type {
			case protocol.FileChunk:
				reassembler.AddChunk(delivered.Payload)
				if progress != nil {
					progress(reassembler.ReceivedBytes(), expectedSize)
				}
			case protocol.FileEnd:
				return reassembler.Finalize()
			}
		}
	}
}

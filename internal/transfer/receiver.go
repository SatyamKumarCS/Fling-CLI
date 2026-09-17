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

// StreamReceiver reassembles chunks delivered via asynchronous packet channels.
type StreamReceiver struct {
	orderedReceiver  *network.OrderedReceiver
	reassembler      *Reassembler
	expectedSize     int64
	expectedChecksum uint32
	outputPath       string
	progress         cli.ProgressFunc
	key              []byte
	chunkChan        chan protocol.Packet
}

// NewStreamReceiver initializes a StreamReceiver for incoming transfers.
func NewStreamReceiver(
	startSequence uint32,
	expectedSize int64,
	expectedChecksum uint32,
	outputPath string,
	progress cli.ProgressFunc,
) *StreamReceiver {
	return &StreamReceiver{
		orderedReceiver:  network.NewOrderedReceiver(startSequence),
		reassembler:      NewReassembler(expectedSize, expectedChecksum),
		expectedSize:     expectedSize,
		expectedChecksum: expectedChecksum,
		outputPath:       outputPath,
		progress:         progress,
		chunkChan:        make(chan protocol.Packet, 500),
	}
}

// SetKey sets the AES-256-GCM decryption key for incoming stream payload decryption.
func (s *StreamReceiver) SetKey(key []byte) {
	s.key = key
}

// Feed inputs a received FileChunk or FileEnd packet into the stream.
func (s *StreamReceiver) Feed(packet protocol.Packet) {
	select {
	case s.chunkChan <- packet:
	default:
	}
}

// ProcessChunk processes a single chunk directly and returns true when transfer finishes.
func (s *StreamReceiver) ProcessChunk(packet protocol.Packet) (bool, error) {
	deliveredPackets := s.orderedReceiver.Receive(packet)
	for _, delivered := range deliveredPackets {
		switch delivered.Type {
		case protocol.FileChunk:
			s.reassembler.AddChunk(delivered.Payload)
			if s.progress != nil {
				s.progress(s.reassembler.ReceivedBytes(), s.expectedSize)
			}
		case protocol.FileEnd:
			err := s.reassembler.SaveToFileWithKey(s.outputPath, s.key)
			return true, err
		}
	}
	return false, nil
}


package transfer

import (
	"hash/crc32"
	"io"
	"os"

	"github.com/SatyamKumarCS/Fling-CLI/internal/protocol"
)

// ChunkSize is the maximum payload size in bytes per FileChunk packet.
const ChunkSize = 1024

// CalculateFileChecksum calculates the CRC32 IEEE checksum and size of a file.
func CalculateFileChecksum(filePath string) (uint32, int64, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()

	hash := crc32.NewIEEE()
	size, err := io.Copy(hash, file)
	if err != nil {
		return 0, 0, err
	}

	return hash.Sum32(), size, nil
}

// CalculateBytesChecksum calculates the CRC32 IEEE checksum of a byte slice.
func CalculateBytesChecksum(data []byte) uint32 {
	return protocol.CalculateChecksum(data)
}

// ChunkBytes splits a byte slice into FileChunk packets starting at startSequence.
func ChunkBytes(data []byte, startSequence uint32) []protocol.Packet {
	if len(data) == 0 {
		return nil
	}

	var packets []protocol.Packet
	currentSeq := startSequence

	for offset := 0; offset < len(data); offset += ChunkSize {
		end := offset + ChunkSize
		if end > len(data) {
			end = len(data)
		}

		chunkData := make([]byte, end-offset)
		copy(chunkData, data[offset:end])

		packet := protocol.Packet{
			SequenceNumber: currentSeq,
			Type:           protocol.FileChunk,
			Payload:        chunkData,
		}

		packets = append(packets, packet)
		currentSeq++
	}

	return packets
}

// ChunkFile reads the file at filePath and splits it into FileChunk packets starting at startSequence.
func ChunkFile(filePath string, startSequence uint32) ([]protocol.Packet, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	return ChunkBytes(data, startSequence), nil
}

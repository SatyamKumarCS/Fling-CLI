package transfer

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
)

// Reassembler collects file chunks and validates file size and checksum before saving.
type Reassembler struct {
	ExpectedSize     int64
	ExpectedChecksum uint32
	buffer           bytes.Buffer
	receivedBytes    int64
}

// NewReassembler creates a new Reassembler with expected file size and CRC32 checksum.
func NewReassembler(expectedSize int64, expectedChecksum uint32) *Reassembler {
	return &Reassembler{
		ExpectedSize:     expectedSize,
		ExpectedChecksum: expectedChecksum,
	}
}

// AddChunk appends a delivered chunk payload to the reassembler.
func (r *Reassembler) AddChunk(payload []byte) {
	r.buffer.Write(payload)
	r.receivedBytes += int64(len(payload))
}

// ReceivedBytes returns the count of bytes received so far.
func (r *Reassembler) ReceivedBytes() int64 {
	return r.receivedBytes
}

// Finalize verifies the reassembled data against ExpectedSize and ExpectedChecksum,
// returning the verified data bytes or an error if validation fails.
func (r *Reassembler) Finalize() ([]byte, error) {
	if r.receivedBytes != r.ExpectedSize {
		return nil, fmt.Errorf(
			"file size mismatch: expected %d bytes, got %d bytes",
			r.ExpectedSize,
			r.receivedBytes,
		)
	}

	data := r.buffer.Bytes()
	actualChecksum := CalculateBytesChecksum(data)

	if actualChecksum != r.ExpectedChecksum {
		return nil, fmt.Errorf(
			"checksum mismatch: expected %08x, got %08x",
			r.ExpectedChecksum,
			actualChecksum,
		)
	}

	return data, nil
}

// SaveToFile verifies the reassembled file and writes it to outputPath.
func (r *Reassembler) SaveToFile(outputPath string) error {
	data, err := r.Finalize()
	if err != nil {
		return err
	}

	dir := filepath.Dir(outputPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create target directory %s: %w", dir, err)
		}
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write output file %s: %w", outputPath, err)
	}

	return nil
}

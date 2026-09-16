package tests

import (
	"bytes"
	"crypto/rand"
	"hash/crc32"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/SatyamKumarCS/Fling-CLI/internal/cli"
	"github.com/SatyamKumarCS/Fling-CLI/internal/network"
	"github.com/SatyamKumarCS/Fling-CLI/internal/protocol"
	"github.com/SatyamKumarCS/Fling-CLI/internal/transfer"
)

func TestChunkBytesAndChunkFile(t *testing.T) {
	// Test exact 1024 multiple (2048 bytes -> 2 chunks)
	data2048 := make([]byte, 2048)
	_, _ = rand.Read(data2048)

	chunks := transfer.ChunkBytes(data2048, 1)
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks for 2048 bytes, got %d", len(chunks))
	}
	if chunks[0].SequenceNumber != 1 || chunks[1].SequenceNumber != 2 {
		t.Errorf("incorrect sequence numbers: %d, %d", chunks[0].SequenceNumber, chunks[1].SequenceNumber)
	}
	if len(chunks[0].Payload) != 1024 || len(chunks[1].Payload) != 1024 {
		t.Errorf("chunk sizes incorrect: %d, %d", len(chunks[0].Payload), len(chunks[1].Payload))
	}
	if chunks[0].Type != protocol.FileChunk || chunks[1].Type != protocol.FileChunk {
		t.Errorf("expected FileChunk type")
	}

	// Test non-multiple (2500 bytes -> 2 * 1024 + 1 * 452)
	data2500 := make([]byte, 2500)
	_, _ = rand.Read(data2500)

	chunks = transfer.ChunkBytes(data2500, 10)
	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks for 2500 bytes, got %d", len(chunks))
	}
	if chunks[0].SequenceNumber != 10 || chunks[1].SequenceNumber != 11 || chunks[2].SequenceNumber != 12 {
		t.Errorf("incorrect sequence numbers for offset: %d, %d, %d", chunks[0].SequenceNumber, chunks[1].SequenceNumber, chunks[2].SequenceNumber)
	}
	if len(chunks[0].Payload) != 1024 || len(chunks[1].Payload) != 1024 || len(chunks[2].Payload) != 452 {
		t.Errorf("chunk payload sizes incorrect: %d, %d, %d", len(chunks[0].Payload), len(chunks[1].Payload), len(chunks[2].Payload))
	}

	// Test empty bytes
	emptyChunks := transfer.ChunkBytes([]byte{}, 1)
	if len(emptyChunks) != 0 {
		t.Errorf("expected 0 chunks for empty data, got %d", len(emptyChunks))
	}

	// Test ChunkFile on filesystem
	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "test_chunk_file.bin")
	if err := os.WriteFile(tempFile, data2500, 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	fileChunks, err := transfer.ChunkFile(tempFile, 100)
	if err != nil {
		t.Fatalf("ChunkFile failed: %v", err)
	}
	if len(fileChunks) != 3 {
		t.Fatalf("expected 3 chunks from file, got %d", len(fileChunks))
	}
	if fileChunks[0].SequenceNumber != 100 {
		t.Errorf("expected start sequence 100, got %d", fileChunks[0].SequenceNumber)
	}
}

func TestCalculateFileChecksum(t *testing.T) {
	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "checksum_test.dat")
	data := []byte("Hello, Fling P2P file transfer checksum test!")

	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	checksum, size, err := transfer.CalculateFileChecksum(tempFile)
	if err != nil {
		t.Fatalf("CalculateFileChecksum failed: %v", err)
	}

	expectedChecksum := crc32.ChecksumIEEE(data)
	if checksum != expectedChecksum {
		t.Errorf("expected checksum %08x, got %08x", expectedChecksum, checksum)
	}
	if size != int64(len(data)) {
		t.Errorf("expected size %d, got %d", len(data), size)
	}

	byteChecksum := transfer.CalculateBytesChecksum(data)
	if byteChecksum != expectedChecksum {
		t.Errorf("CalculateBytesChecksum mismatch: expected %08x, got %08x", expectedChecksum, byteChecksum)
	}
}

func TestReassemblerSuccessInOrder(t *testing.T) {
	data := []byte("A long string of test data that will be split into multiple chunks and reassembled.")
	expectedChecksum := transfer.CalculateBytesChecksum(data)
	expectedSize := int64(len(data))

	reassembler := transfer.NewReassembler(expectedSize, expectedChecksum)

	chunks := transfer.ChunkBytes(data, 1)
	for _, chunk := range chunks {
		reassembler.AddChunk(chunk.Payload)
	}

	if reassembler.ReceivedBytes() != expectedSize {
		t.Errorf("expected received bytes %d, got %d", expectedSize, reassembler.ReceivedBytes())
	}

	finalBytes, err := reassembler.Finalize()
	if err != nil {
		t.Fatalf("Finalize failed: %v", err)
	}

	if !bytes.Equal(finalBytes, data) {
		t.Errorf("reassembled data does not match original data")
	}
}

func TestReassemblerChecksumMismatch(t *testing.T) {
	data := []byte("Original authentic data payload")
	expectedChecksum := transfer.CalculateBytesChecksum(data)
	expectedSize := int64(len(data))

	reassembler := transfer.NewReassembler(expectedSize, expectedChecksum)

	// Feed corrupted data
	corrupted := []byte("Corrupted tampered data payload")
	reassembler.AddChunk(corrupted)

	_, err := reassembler.Finalize()
	if err == nil {
		t.Fatal("expected checksum mismatch error, got nil")
	}

	if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Errorf("expected error containing 'checksum mismatch', got: %v", err)
	}
}

func TestReassemblerFileSizeMismatch(t *testing.T) {
	expectedSize := int64(500)
	expectedChecksum := uint32(0x12345678)

	reassembler := transfer.NewReassembler(expectedSize, expectedChecksum)
	reassembler.AddChunk([]byte("only 20 bytes here!"))

	_, err := reassembler.Finalize()
	if err == nil {
		t.Fatal("expected file size mismatch error, got nil")
	}

	if !strings.Contains(err.Error(), "file size mismatch") {
		t.Errorf("expected error containing 'file size mismatch', got: %v", err)
	}
}

func TestReassemblyWithOrderedReceiver(t *testing.T) {
	data := make([]byte, 3000)
	_, _ = rand.Read(data)
	expectedChecksum := transfer.CalculateBytesChecksum(data)
	expectedSize := int64(len(data))

	chunks := transfer.ChunkBytes(data, 1) // 3 chunks (seq 1, 2, 3)

	receiver := network.NewOrderedReceiver(1)
	reassembler := transfer.NewReassembler(expectedSize, expectedChecksum)

	// Feed out of order: chunk 3, chunk 1 (duplicate), chunk 2, chunk 1
	// 1. Send chunk 3 -> buffered
	delivered := receiver.Receive(chunks[2])
	for _, p := range delivered {
		reassembler.AddChunk(p.Payload)
	}

	// 2. Send duplicate chunk 3 -> ignored
	delivered = receiver.Receive(chunks[2])
	for _, p := range delivered {
		reassembler.AddChunk(p.Payload)
	}

	// 3. Send chunk 1 -> delivers #1
	delivered = receiver.Receive(chunks[0])
	for _, p := range delivered {
		reassembler.AddChunk(p.Payload)
	}

	// 4. Send duplicate chunk 1 -> ignored
	delivered = receiver.Receive(chunks[0])
	for _, p := range delivered {
		reassembler.AddChunk(p.Payload)
	}

	// 5. Send chunk 2 -> delivers #2 and then buffered #3
	delivered = receiver.Receive(chunks[1])
	for _, p := range delivered {
		reassembler.AddChunk(p.Payload)
	}

	finalBytes, err := reassembler.Finalize()
	if err != nil {
		t.Fatalf("Finalize failed: %v", err)
	}

	if !bytes.Equal(finalBytes, data) {
		t.Errorf("reassembled data does not match original data")
	}
}

func TestEndToEndFileTransfer(t *testing.T) {
	tempDir := t.TempDir()
	inputFile := filepath.Join(tempDir, "source.bin")
	outputFile := filepath.Join(tempDir, "received", "destination.bin")

	// Generate 3500 bytes of arbitrary test data (spans 4 chunks)
	originalData := make([]byte, 3500)
	_, _ = rand.Read(originalData)
	if err := os.WriteFile(inputFile, originalData, 0644); err != nil {
		t.Fatalf("failed to write input file: %v", err)
	}

	checksum, size, err := transfer.CalculateFileChecksum(inputFile)
	if err != nil {
		t.Fatalf("CalculateFileChecksum failed: %v", err)
	}

	// Create receiver UDP socket
	receiverConn, err := network.ListenUDP(0)
	if err != nil {
		t.Fatalf("failed to create receiver socket: %v", err)
	}
	defer receiverConn.Close()

	receiverAddr := &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: receiverConn.LocalAddr().(*net.UDPAddr).Port,
	}

	// Create sender UDP socket
	senderConn, err := network.ListenUDP(0)
	if err != nil {
		t.Fatalf("failed to create sender socket: %v", err)
	}
	defer senderConn.Close()

	var receiverProgressCalled int32
	var senderProgressCalled int32
	var receiverErr error
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		receiverErr = transfer.ReceiveFile(
			receiverConn,
			1,
			size,
			checksum,
			outputFile,
			func(transferred, total int64) {
				atomic.AddInt32(&receiverProgressCalled, 1)
			},
		)
	}()

	progressBar := cli.NewProgressBar(size, "source.bin")

	nextSeq, err := transfer.SendFile(
		senderConn,
		receiverAddr,
		inputFile,
		1,
		func(transferred, total int64) {
			atomic.AddInt32(&senderProgressCalled, 1)
			progressBar.Update(transferred)
		},
	)
	progressBar.Finish()

	if err != nil {
		t.Fatalf("SendFile failed: %v", err)
	}

	// 4 chunks (seq 1, 2, 3, 4) + 1 FileEnd (seq 5) -> nextSeq should be 6
	if nextSeq != 6 {
		t.Errorf("expected next sequence number 6, got %d", nextSeq)
	}

	wg.Wait()

	if receiverErr != nil {
		t.Fatalf("ReceiveFile failed: %v", receiverErr)
	}

	if atomic.LoadInt32(&senderProgressCalled) == 0 {
		t.Errorf("expected sender progress callback to be called")
	}

	if atomic.LoadInt32(&receiverProgressCalled) == 0 {
		t.Errorf("expected receiver progress callback to be called")
	}

	// Verify output file on disk
	receivedData, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("failed to read destination file: %v", err)
	}

	if !bytes.Equal(receivedData, originalData) {
		t.Errorf("received file content does not match original file")
	}

	receivedChecksum, receivedSize, err := transfer.CalculateFileChecksum(outputFile)
	if err != nil {
		t.Fatalf("CalculateFileChecksum on output failed: %v", err)
	}

	if receivedSize != size {
		t.Errorf("size mismatch: expected %d, got %d", size, receivedSize)
	}

	if receivedChecksum != checksum {
		t.Errorf("checksum mismatch: expected %08x, got %08x", checksum, receivedChecksum)
	}
}

func TestProgressBarFormatting(t *testing.T) {
	pb := cli.NewProgressBar(1000, "test.txt")
	pb.Update(500)

	formatted := pb.Format()
	if !strings.Contains(formatted, "test.txt") {
		t.Errorf("expected progress bar to contain filename, got: %s", formatted)
	}
	if !strings.Contains(formatted, "500/1000 bytes") {
		t.Errorf("expected progress bar to contain byte counts, got: %s", formatted)
	}
	if !strings.Contains(formatted, "50.0%") {
		t.Errorf("expected progress bar to contain 50.0%%, got: %s", formatted)
	}
}

func TestResolveFilePath(t *testing.T) {
	// 1. Direct path
	p1, info, err := transfer.ResolveFilePath("tests/test.txt")
	if err != nil || info == nil || !strings.HasSuffix(p1, "tests/test.txt") {
		t.Errorf("failed to resolve direct path tests/test.txt: %v", err)
	}

	// 2. Quoted path (from drag and drop)
	p2, info, err := transfer.ResolveFilePath("\"tests/test.txt\"")
	if err != nil || info == nil || !strings.HasSuffix(p2, "tests/test.txt") {
		t.Errorf("failed to resolve quoted path: %v", err)
	}

	// 3. Typo variation (test/test.txt -> tests/test.txt)
	p3, info, err := transfer.ResolveFilePath("test/test.txt")
	if err != nil || info == nil || !strings.HasSuffix(p3, "tests/test.txt") {
		t.Errorf("failed to resolve variation test/test.txt: %v", err)
	}

	// 4. Filename only fallback
	p4, info, err := transfer.ResolveFilePath("test.txt")
	if err != nil || info == nil || !strings.HasSuffix(p4, "test.txt") {
		t.Errorf("failed to resolve filename only test.txt: %v", err)
	}

	// 5. Non-existent file
	_, _, err = transfer.ResolveFilePath("non_existent_dir/completely_missing_file_12345.xyz")
	if err == nil {
		t.Errorf("expected error for non-existent file, got nil")
	}
}

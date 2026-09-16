package tests

import (
	"testing"

	"github.com/SatyamKumarCS/Fling-CLI/internal/handshake"
	"github.com/SatyamKumarCS/Fling-CLI/internal/protocol"
)

func TestCreateAndParseTransferRequest(t *testing.T) {
	filename := "document.pdf"
	fileSize := int64(1048576)
	checksum := uint32(0xDEADBEEF)
	seq := uint32(1)

	packet := handshake.CreateTransferRequest(filename, fileSize, checksum, seq)

	if packet.Type != protocol.TransferRequest {
		t.Errorf("expected packet type %d (TransferRequest), got %d", protocol.TransferRequest, packet.Type)
	}

	if packet.SequenceNumber != seq {
		t.Errorf("expected sequence number %d, got %d", seq, packet.SequenceNumber)
	}

	// Parse valid request
	req, err := handshake.ParseTransferRequest(packet)
	if err != nil {
		t.Fatalf("ParseTransferRequest failed: %v", err)
	}

	if req.Filename != filename {
		t.Errorf("expected filename %q, got %q", filename, req.Filename)
	}

	if req.FileSize != fileSize {
		t.Errorf("expected fileSize %d, got %d", fileSize, req.FileSize)
	}

	if req.Checksum != checksum {
		t.Errorf("expected checksum %08x, got %08x", checksum, req.Checksum)
	}
}

func TestParseTransferRequestErrors(t *testing.T) {
	// Wrong packet type
	wrongTypePacket := protocol.Packet{
		SequenceNumber: 1,
		Type:           protocol.Msg,
		Payload:        []byte("file.txt|100|1234"),
	}
	if _, err := handshake.ParseTransferRequest(wrongTypePacket); err == nil {
		t.Error("expected error for wrong packet type, got nil")
	}

	// Invalid payload format (fewer than 3 parts)
	invalidPartsPacket := protocol.Packet{
		SequenceNumber: 1,
		Type:           protocol.TransferRequest,
		Payload:        []byte("file.txt|100"),
	}
	if _, err := handshake.ParseTransferRequest(invalidPartsPacket); err == nil {
		t.Error("expected error for invalid parts count, got nil")
	}

	// Invalid file size (non-numeric)
	invalidSizePacket := protocol.Packet{
		SequenceNumber: 1,
		Type:           protocol.TransferRequest,
		Payload:        []byte("file.txt|notanumber|1234"),
	}
	if _, err := handshake.ParseTransferRequest(invalidSizePacket); err == nil {
		t.Error("expected error for invalid file size, got nil")
	}

	// Invalid checksum (non-numeric)
	invalidChecksumPacket := protocol.Packet{
		SequenceNumber: 1,
		Type:           protocol.TransferRequest,
		Payload:        []byte("file.txt|100|notachecksum"),
	}
	if _, err := handshake.ParseTransferRequest(invalidChecksumPacket); err == nil {
		t.Error("expected error for invalid checksum, got nil")
	}
}

func TestCreateTransferAccept(t *testing.T) {
	seq := uint32(42)
	packet := handshake.CreateTransferAccept(seq)

	if packet.Type != protocol.TransferAccept {
		t.Errorf("expected packet type %d (TransferAccept), got %d", protocol.TransferAccept, packet.Type)
	}

	if packet.SequenceNumber != seq {
		t.Errorf("expected sequence number %d, got %d", seq, packet.SequenceNumber)
	}

	if packet.Payload != nil {
		t.Errorf("expected nil payload, got %v", packet.Payload)
	}
}

func TestCreateTransferReject(t *testing.T) {
	seq := uint32(99)
	packet := handshake.CreateTransferReject(seq)

	if packet.Type != protocol.TransferReject {
		t.Errorf("expected packet type %d (TransferReject), got %d", protocol.TransferReject, packet.Type)
	}

	if packet.SequenceNumber != seq {
		t.Errorf("expected sequence number %d, got %d", seq, packet.SequenceNumber)
	}

	if packet.Payload != nil {
		t.Errorf("expected nil payload, got %v", packet.Payload)
	}
}

func TestHandshakePacketsEncodeDecode(t *testing.T) {
	packets := []protocol.Packet{
		handshake.CreateTransferRequest("archive.zip", 2048, 0x12345678, 1),
		handshake.CreateTransferAccept(2),
		handshake.CreateTransferReject(3),
	}

	for _, original := range packets {
		encoded, err := protocol.Encode(original)
		if err != nil {
			t.Fatalf("failed to encode packet type %d: %v", original.Type, err)
		}

		decoded, err := protocol.Decode(encoded)
		if err != nil {
			t.Fatalf("failed to decode packet type %d: %v", original.Type, err)
		}

		if decoded.Type != original.Type {
			t.Errorf("type mismatch: expected %d, got %d", original.Type, decoded.Type)
		}

		if decoded.SequenceNumber != original.SequenceNumber {
			t.Errorf("sequence mismatch: expected %d, got %d", original.SequenceNumber, decoded.SequenceNumber)
		}
	}
}

package tests

import (
	"bytes"
	"testing"

	"github.com/SatyamKumarCS/Fling-CLI/internal/protocol"
)

func TestEncodeDecode(t *testing.T) {
	original := protocol.Packet{
		SequenceNumber: 42,
		Type:           protocol.Msg,
		Payload:        []byte("Hello Fling"),
	}

	encoded, err := protocol.Encode(original)

	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	decoded, err := protocol.Decode(encoded)

	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if decoded.SequenceNumber != original.SequenceNumber {
		t.Errorf(
			"sequence mismatch: expected %d, got %d",
			original.SequenceNumber,
			decoded.SequenceNumber,
		)
	}

	if decoded.Type != original.Type {
		t.Errorf(
			"type mismatch: expected %d, got %d",
			original.Type,
			decoded.Type,
		)
	}

	if !bytes.Equal(decoded.Payload, original.Payload) {
		t.Errorf(
			"payload mismatch: expected %q, got %q",
			original.Payload,
			decoded.Payload,
		)
	}
}

func TestChecksumDetectsCorruption(t *testing.T) {
	packet := protocol.Packet{
		SequenceNumber: 1,
		Type:           protocol.FileChunk,
		Payload:        []byte("important file data"),
	}

	encoded, err := protocol.Encode(packet)

	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	// Corrupt one byte in the payload.
	encoded[len(encoded)-1] ^= 0xFF

	_, err = protocol.Decode(encoded)

	if err == nil {
		t.Fatal("expected checksum error, got nil")
	}
}

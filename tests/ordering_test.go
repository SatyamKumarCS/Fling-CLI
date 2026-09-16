package tests

import (
	"testing"

	"github.com/SatyamKumarCS/Fling-CLI/internal/network"
	"github.com/SatyamKumarCS/Fling-CLI/internal/protocol"
)

func TestOrderedReceiverInOrder(t *testing.T) {
	receiver := network.NewOrderedReceiver(1)

	for seq := uint32(1); seq <= 5; seq++ {
		packet := protocol.Packet{
			SequenceNumber: seq,
			Type:           protocol.FileChunk,
			Payload:        []byte{byte(seq)},
		}

		delivered := receiver.Receive(packet)
		if len(delivered) != 1 {
			t.Fatalf("expected 1 delivered packet for seq %d, got %d", seq, len(delivered))
		}

		if delivered[0].SequenceNumber != seq {
			t.Errorf("expected sequence number %d, got %d", seq, delivered[0].SequenceNumber)
		}
	}
}

func TestOrderedReceiverOutOfOrderAndReassembly(t *testing.T) {
	receiver := network.NewOrderedReceiver(1)

	p1 := protocol.Packet{SequenceNumber: 1, Type: protocol.FileChunk, Payload: []byte("chunk 1")}
	p2 := protocol.Packet{SequenceNumber: 2, Type: protocol.FileChunk, Payload: []byte("chunk 2")}
	p3 := protocol.Packet{SequenceNumber: 3, Type: protocol.FileChunk, Payload: []byte("chunk 3")}
	p4 := protocol.Packet{SequenceNumber: 4, Type: protocol.FileChunk, Payload: []byte("chunk 4")}
	p5 := protocol.Packet{SequenceNumber: 5, Type: protocol.FileChunk, Payload: []byte("chunk 5")}

	// Send #3 first (arrives early) -> should buffer
	if delivered := receiver.Receive(p3); len(delivered) != 0 {
		t.Errorf("expected 0 delivered packets for early packet #3, got %d", len(delivered))
	}

	// Send #2 (arrives early) -> should buffer
	if delivered := receiver.Receive(p2); len(delivered) != 0 {
		t.Errorf("expected 0 delivered packets for early packet #2, got %d", len(delivered))
	}

	// Send #1 (expected packet) -> should deliver #1, #2, #3 in order
	delivered := receiver.Receive(p1)
	if len(delivered) != 3 {
		t.Fatalf("expected 3 delivered packets after receiving #1, got %d", len(delivered))
	}
	expectedSeqs := []uint32{1, 2, 3}
	for i, p := range delivered {
		if p.SequenceNumber != expectedSeqs[i] {
			t.Errorf("at index %d: expected sequence %d, got %d", i, expectedSeqs[i], p.SequenceNumber)
		}
	}

	// Send #5 (arrives early) -> should buffer
	if delivered := receiver.Receive(p5); len(delivered) != 0 {
		t.Errorf("expected 0 delivered packets for early packet #5, got %d", len(delivered))
	}

	// Send #4 -> should deliver #4, #5
	delivered = receiver.Receive(p4)
	if len(delivered) != 2 {
		t.Fatalf("expected 2 delivered packets after receiving #4, got %d", len(delivered))
	}
	if delivered[0].SequenceNumber != 4 || delivered[1].SequenceNumber != 5 {
		t.Errorf("expected packets [4, 5], got [%d, %d]", delivered[0].SequenceNumber, delivered[1].SequenceNumber)
	}
}

func TestOrderedReceiverDuplicates(t *testing.T) {
	receiver := network.NewOrderedReceiver(1)

	p1 := protocol.Packet{SequenceNumber: 1, Type: protocol.FileChunk, Payload: []byte("chunk 1")}
	p2 := protocol.Packet{SequenceNumber: 2, Type: protocol.FileChunk, Payload: []byte("chunk 2")}
	p3 := protocol.Packet{SequenceNumber: 3, Type: protocol.FileChunk, Payload: []byte("chunk 3")}

	// Receive packet #1 -> delivered
	delivered := receiver.Receive(p1)
	if len(delivered) != 1 || delivered[0].SequenceNumber != 1 {
		t.Fatalf("expected packet #1 delivered, got %v", delivered)
	}

	// Send duplicate packet #1 (already processed) -> ignored (nil)
	delivered = receiver.Receive(p1)
	if len(delivered) != 0 {
		t.Errorf("expected duplicate packet #1 to be ignored, got %d packets", len(delivered))
	}

	// Send future packet #3 -> buffered
	delivered = receiver.Receive(p3)
	if len(delivered) != 0 {
		t.Errorf("expected packet #3 to be buffered, got %d packets", len(delivered))
	}

	// Send duplicate packet #3 (already in buffer) -> ignored (nil)
	delivered = receiver.Receive(p3)
	if len(delivered) != 0 {
		t.Errorf("expected duplicate packet #3 to be ignored, got %d packets", len(delivered))
	}

	// Send missing packet #2 -> delivers [#2, #3]
	delivered = receiver.Receive(p2)
	if len(delivered) != 2 {
		t.Fatalf("expected 2 packets delivered [#2, #3], got %d", len(delivered))
	}
	if delivered[0].SequenceNumber != 2 || delivered[1].SequenceNumber != 3 {
		t.Errorf("expected sequence numbers [2, 3], got [%d, %d]", delivered[0].SequenceNumber, delivered[1].SequenceNumber)
	}
}

func TestOrderedReceiverCustomStartSequence(t *testing.T) {
	startSeq := uint32(100)
	receiver := network.NewOrderedReceiver(startSeq)

	p100 := protocol.Packet{SequenceNumber: 100, Type: protocol.Msg, Payload: []byte("msg 100")}
	p101 := protocol.Packet{SequenceNumber: 101, Type: protocol.Msg, Payload: []byte("msg 101")}
	p102 := protocol.Packet{SequenceNumber: 102, Type: protocol.Msg, Payload: []byte("msg 102")}

	// Send #102 early -> buffered
	if delivered := receiver.Receive(p102); len(delivered) != 0 {
		t.Errorf("expected 0 delivered for #102, got %d", len(delivered))
	}

	// Send #100 -> delivered [#100]
	delivered := receiver.Receive(p100)
	if len(delivered) != 1 || delivered[0].SequenceNumber != 100 {
		t.Fatalf("expected [#100] delivered, got %v", delivered)
	}

	// Send #101 -> delivers [#101, #102]
	delivered = receiver.Receive(p101)
	if len(delivered) != 2 {
		t.Fatalf("expected 2 packets delivered [#101, #102], got %d", len(delivered))
	}
	if delivered[0].SequenceNumber != 101 || delivered[1].SequenceNumber != 102 {
		t.Errorf("expected [101, 102], got [%d, %d]", delivered[0].SequenceNumber, delivered[1].SequenceNumber)
	}
}

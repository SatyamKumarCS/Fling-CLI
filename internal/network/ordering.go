package network

import (
	"github.com/SatyamKumarCS/Fling-CLI/internal/protocol"
)

type OrderedReceiver struct {
	expectedSequence uint32
	buffer           map[uint32]protocol.Packet
}

func NewOrderedReceiver(startSequence uint32) *OrderedReceiver {
	return &OrderedReceiver{
		expectedSequence: startSequence,
		buffer:           make(map[uint32]protocol.Packet),
	}
}

func (r *OrderedReceiver) Receive(
	packet protocol.Packet,
) []protocol.Packet {
	// Already processed.
	if packet.SequenceNumber < r.expectedSequence {
		return nil
	}

	// Packet arrived too early.
	if packet.SequenceNumber > r.expectedSequence {
		if _, exists := r.buffer[packet.SequenceNumber]; exists {
			return nil
		}

		r.buffer[packet.SequenceNumber] = packet
		return nil
	}

	// Correct packet arrived.
	delivered := []protocol.Packet{packet}
	r.expectedSequence++

	// Release any consecutive buffered packets.
	for {
		nextPacket, exists := r.buffer[r.expectedSequence]
		if !exists {
			break
		}

		delivered = append(delivered, nextPacket)
		delete(r.buffer, r.expectedSequence)
		r.expectedSequence++
	}

	return delivered
}

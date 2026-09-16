package network

import (
	"fmt"

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
		fmt.Printf(
			"[DUPLICATE] packet #%d ignored\n",
			packet.SequenceNumber,
		)

		return nil
	}

	// Packet arrived too early.
	if packet.SequenceNumber > r.expectedSequence {

		if _, exists := r.buffer[packet.SequenceNumber]; exists {
			fmt.Printf(
				"[DUPLICATE] packet #%d already buffered\n",
				packet.SequenceNumber,
			)

			return nil
		}

		r.buffer[packet.SequenceNumber] = packet

		fmt.Printf(
			"[BUFFER] packet #%d waiting for #%d\n",
			packet.SequenceNumber,
			r.expectedSequence,
		)

		return nil
	}

	// Correct packet arrived.
	delivered := []protocol.Packet{packet}

	fmt.Printf(
		"[DELIVER] packet #%d\n",
		packet.SequenceNumber,
	)

	r.expectedSequence++

	// Release any consecutive buffered packets.
	for {

		nextPacket, exists := r.buffer[r.expectedSequence]

		if !exists {
			break
		}

		delivered = append(delivered, nextPacket)

		fmt.Printf(
			"[DELIVER] buffered packet #%d\n",
			nextPacket.SequenceNumber,
		)

		delete(r.buffer, r.expectedSequence)

		r.expectedSequence++
	}

	return delivered
}

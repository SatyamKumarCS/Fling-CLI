package network

import (
	"fmt"
	"net"
	"time"

	"github.com/SatyamKumarCS/Fling-CLI/internal/protocol"
)

const (
	AckTimeout = 500 * time.Millisecond
	MaxRetries = 6
)

func SendReliable(
	conn *net.UDPConn,
	packet protocol.Packet,
	addr *net.UDPAddr,
) error {

	encoded, err := protocol.Encode(packet)

	if err != nil {
		return err
	}

	for attempt := 1; attempt <= MaxRetries; attempt++ {

		_, err := conn.WriteToUDP(encoded, addr)

		if err != nil {
			return err
		}

		fmt.Printf(
			"Sent packet #%d (attempt %d/%d)\n",
			packet.SequenceNumber,
			attempt,
			MaxRetries,
		)

		err = conn.SetReadDeadline(
			time.Now().Add(AckTimeout),
		)

		if err != nil {
			return err
		}

		buffer := make([]byte, 1500)

		n, _, err := conn.ReadFromUDP(buffer)

		if err != nil {
			fmt.Println("ACK timeout, retrying...")
			continue
		}

		ack, err := protocol.Decode(buffer[:n])

		if err != nil {
			continue
		}

		if ack.Type != protocol.ACK {
			continue
		}

		if ack.SequenceNumber != packet.SequenceNumber {
			continue
		}

		fmt.Printf(
			"Received ACK for packet #%d\n",
			packet.SequenceNumber,
		)

		return nil
	}

	return fmt.Errorf(
		"packet #%d failed after %d retries",
		packet.SequenceNumber,
		MaxRetries,
	)
}

// SendACK sends an ACK for a received packet.
func SendACK(
	conn *net.UDPConn,
	sequenceNumber uint32,
	addr *net.UDPAddr,
) error {

	ack := protocol.Packet{
		SequenceNumber: sequenceNumber,
		Type:           protocol.ACK,
		Payload:        nil,
	}

	encoded, err := protocol.Encode(ack)

	if err != nil {
		return fmt.Errorf("failed to encode ACK: %w", err)
	}

	_, err = conn.WriteToUDP(encoded, addr)

	if err != nil {
		return fmt.Errorf("failed to send ACK: %w", err)
	}

	fmt.Printf(
		"[ACK-SEND] ACK #%d sent\n",
		sequenceNumber,
	)

	return nil
}

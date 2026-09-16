package tests

import (
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SatyamKumarCS/Fling-CLI/internal/network"
	"github.com/SatyamKumarCS/Fling-CLI/internal/protocol"
)

func TestSendReliableSuccess(t *testing.T) {
	receiverConn, err := network.ListenUDP(0)
	if err != nil {
		t.Fatalf("failed to create receiver: %v", err)
	}
	defer receiverConn.Close()

	receiverAddr := &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: receiverConn.LocalAddr().(*net.UDPAddr).Port,
	}

	senderConn, err := network.ListenUDP(0)
	if err != nil {
		t.Fatalf("failed to create sender: %v", err)
	}
	defer senderConn.Close()

	packet := protocol.Packet{
		SequenceNumber: 10,
		Type:           protocol.Msg,
		Payload:        []byte("reliability test payload"),
	}

	// Receiver goroutine
	go func() {
		buf := make([]byte, 1500)
		_ = receiverConn.SetReadDeadline(time.Now().Add(2 * time.Second))
		n, senderAddr, err := receiverConn.ReadFromUDP(buf)
		if err != nil {
			return
		}

		received, err := protocol.Decode(buf[:n])
		if err != nil {
			return
		}

		if received.SequenceNumber != packet.SequenceNumber {
			return
		}

		_ = network.SendACK(receiverConn, received.SequenceNumber, senderAddr)
	}()

	err = network.SendReliable(senderConn, packet, receiverAddr)
	if err != nil {
		t.Fatalf("SendReliable failed: %v", err)
	}
}

func TestSendReliableRetrySuccess(t *testing.T) {
	receiverConn, err := network.ListenUDP(0)
	if err != nil {
		t.Fatalf("failed to create receiver: %v", err)
	}
	defer receiverConn.Close()

	receiverAddr := &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: receiverConn.LocalAddr().(*net.UDPAddr).Port,
	}

	senderConn, err := network.ListenUDP(0)
	if err != nil {
		t.Fatalf("failed to create sender: %v", err)
	}
	defer senderConn.Close()

	packet := protocol.Packet{
		SequenceNumber: 25,
		Type:           protocol.FileChunk,
		Payload:        []byte("retry test chunk"),
	}

	var attempts int32

	// Receiver goroutine: drop the first attempt, ACK the second
	go func() {
		buf := make([]byte, 1500)
		for {
			_ = receiverConn.SetReadDeadline(time.Now().Add(3 * time.Second))
			n, senderAddr, err := receiverConn.ReadFromUDP(buf)
			if err != nil {
				return
			}

			received, err := protocol.Decode(buf[:n])
			if err != nil {
				continue
			}

			if received.SequenceNumber == packet.SequenceNumber {
				attempt := atomic.AddInt32(&attempts, 1)
				if attempt == 1 {
					// Simulate packet/ack loss by not sending ACK on 1st attempt
					continue
				}
				_ = network.SendACK(receiverConn, received.SequenceNumber, senderAddr)
				return
			}
		}
	}()

	err = network.SendReliable(senderConn, packet, receiverAddr)
	if err != nil {
		t.Fatalf("SendReliable failed on retry: %v", err)
	}

	if atomic.LoadInt32(&attempts) < 2 {
		t.Errorf("expected at least 2 attempts, got %d", attempts)
	}
}

func TestSendReliableMaxRetriesExceeded(t *testing.T) {
	receiverConn, err := network.ListenUDP(0)
	if err != nil {
		t.Fatalf("failed to create receiver: %v", err)
	}
	// Close receiver immediately so no ACKs are ever sent
	receiverAddr := &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: receiverConn.LocalAddr().(*net.UDPAddr).Port,
	}
	receiverConn.Close()

	senderConn, err := network.ListenUDP(0)
	if err != nil {
		t.Fatalf("failed to create sender: %v", err)
	}
	defer senderConn.Close()

	packet := protocol.Packet{
		SequenceNumber: 99,
		Type:           protocol.Msg,
		Payload:        []byte("unreachable"),
	}

	err = network.SendReliable(senderConn, packet, receiverAddr)
	if err == nil {
		t.Fatal("expected SendReliable to fail after max retries, got nil")
	}

	if !strings.Contains(err.Error(), "failed after 6 retries") {
		t.Errorf("expected error message to contain 'failed after 6 retries', got: %v", err)
	}
}

func TestSendACK(t *testing.T) {
	receiverConn, err := network.ListenUDP(0)
	if err != nil {
		t.Fatalf("failed to create receiver: %v", err)
	}
	defer receiverConn.Close()

	receiverAddr := &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: receiverConn.LocalAddr().(*net.UDPAddr).Port,
	}

	senderConn, err := network.ListenUDP(0)
	if err != nil {
		t.Fatalf("failed to create sender: %v", err)
	}
	defer senderConn.Close()

	expectedSeq := uint32(77)

	err = network.SendACK(senderConn, expectedSeq, receiverAddr)
	if err != nil {
		t.Fatalf("SendACK failed: %v", err)
	}

	buf := make([]byte, 1500)
	_ = receiverConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, _, err := receiverConn.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("failed to read ACK: %v", err)
	}

	packet, err := protocol.Decode(buf[:n])
	if err != nil {
		t.Fatalf("failed to decode ACK packet: %v", err)
	}

	if packet.Type != protocol.ACK {
		t.Errorf("expected packet type ACK (%d), got %d", protocol.ACK, packet.Type)
	}

	if packet.SequenceNumber != expectedSeq {
		t.Errorf("expected sequence number %d, got %d", expectedSeq, packet.SequenceNumber)
	}

	if len(packet.Payload) != 0 {
		t.Errorf("expected empty payload for ACK, got %d bytes", len(packet.Payload))
	}
}

func TestSendReliableIgnoresWrongACK(t *testing.T) {
	receiverConn, err := network.ListenUDP(0)
	if err != nil {
		t.Fatalf("failed to create receiver: %v", err)
	}
	defer receiverConn.Close()

	receiverAddr := &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: receiverConn.LocalAddr().(*net.UDPAddr).Port,
	}

	senderConn, err := network.ListenUDP(0)
	if err != nil {
		t.Fatalf("failed to create sender: %v", err)
	}
	defer senderConn.Close()

	packet := protocol.Packet{
		SequenceNumber: 50,
		Type:           protocol.Msg,
		Payload:        []byte("wrong ack test"),
	}

	var attempts int32

	go func() {
		buf := make([]byte, 1500)
		for {
			_ = receiverConn.SetReadDeadline(time.Now().Add(3 * time.Second))
			n, senderAddr, err := receiverConn.ReadFromUDP(buf)
			if err != nil {
				return
			}

			received, err := protocol.Decode(buf[:n])
			if err != nil {
				continue
			}

			if received.SequenceNumber == packet.SequenceNumber {
				attempt := atomic.AddInt32(&attempts, 1)
				if attempt == 1 {
					// Send ACK with wrong sequence number
					_ = network.SendACK(receiverConn, 999, senderAddr)
					continue
				}
				// Send correct ACK on subsequent attempt
				_ = network.SendACK(receiverConn, packet.SequenceNumber, senderAddr)
				return
			}
		}
	}()

	err = network.SendReliable(senderConn, packet, receiverAddr)
	if err != nil {
		t.Fatalf("SendReliable failed: %v", err)
	}

	if atomic.LoadInt32(&attempts) < 2 {
		t.Errorf("expected at least 2 attempts due to wrong ACK, got %d", attempts)
	}
}

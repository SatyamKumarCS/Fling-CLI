package tests

import (
	"bytes"
	"encoding/json"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SatyamKumarCS/Fling-CLI/internal/messaging"
	"github.com/SatyamKumarCS/Fling-CLI/internal/network"
	"github.com/SatyamKumarCS/Fling-CLI/internal/protocol"
)

func TestCreateMessageValid(t *testing.T) {
	sender := "alice-laptop"
	content := "Hello Bob, are you online?"
	seq := uint32(101)

	packet, err := messaging.CreateMessage(sender, content, seq)
	if err != nil {
		t.Fatalf("CreateMessage failed: %v", err)
	}

	if packet.Type != protocol.Msg {
		t.Errorf("expected packet type %d (protocol.Msg), got %d", protocol.Msg, packet.Type)
	}

	if packet.SequenceNumber != seq {
		t.Errorf("expected sequence number %d, got %d", seq, packet.SequenceNumber)
	}

	if len(packet.Payload) == 0 {
		t.Fatal("expected non-empty payload")
	}

	// Verify payload parses back to Message
	msg, err := messaging.ParseMessage(packet)
	if err != nil {
		t.Fatalf("ParseMessage failed on created packet: %v", err)
	}

	if msg.Sender != sender {
		t.Errorf("expected sender %q, got %q", sender, msg.Sender)
	}

	if msg.Content != content {
		t.Errorf("expected content %q, got %q", content, msg.Content)
	}

	if msg.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
}

func TestCreateMessageWithTimestamp(t *testing.T) {
	sender := "charlie"
	content := "Specific timestamp message"
	fixedTime := time.Date(2026, 9, 16, 15, 30, 0, 0, time.UTC)
	seq := uint32(42)

	packet, err := messaging.CreateMessageWithTimestamp(sender, content, fixedTime, seq)
	if err != nil {
		t.Fatalf("CreateMessageWithTimestamp failed: %v", err)
	}

	msg, err := messaging.ParseMessage(packet)
	if err != nil {
		t.Fatalf("ParseMessage failed: %v", err)
	}

	if !msg.Timestamp.Equal(fixedTime) {
		t.Errorf("expected timestamp %v, got %v", fixedTime, msg.Timestamp)
	}
}

func TestCreateMessageValidationErrors(t *testing.T) {
	// Empty content
	_, err := messaging.CreateMessage("sender", "", 1)
	if err != messaging.ErrEmptyMessage {
		t.Errorf("expected ErrEmptyMessage for empty content, got: %v", err)
	}

	// Whitespace-only content
	_, err = messaging.CreateMessage("sender", "   \t\n  ", 1)
	if err != messaging.ErrEmptyMessage {
		t.Errorf("expected ErrEmptyMessage for whitespace content, got: %v", err)
	}

	// Empty sender
	_, err = messaging.CreateMessage("", "hello", 1)
	if err != messaging.ErrEmptySender {
		t.Errorf("expected ErrEmptySender for empty sender, got: %v", err)
	}

	// Whitespace-only sender
	_, err = messaging.CreateMessage("   ", "hello", 1)
	if err != messaging.ErrEmptySender {
		t.Errorf("expected ErrEmptySender for whitespace sender, got: %v", err)
	}

	// Oversized message
	oversizedContent := strings.Repeat("a", messaging.MaxMessageSize+100)
	_, err = messaging.CreateMessage("sender", oversizedContent, 1)
	if err != messaging.ErrMessageTooLong {
		t.Errorf("expected ErrMessageTooLong, got: %v", err)
	}
}

func TestParseMessageErrors(t *testing.T) {
	// Wrong packet type
	wrongTypePacket := protocol.Packet{
		SequenceNumber: 1,
		Type:           protocol.TransferRequest,
		Payload:        []byte(`{"sender":"alice","content":"hello"}`),
	}
	if _, err := messaging.ParseMessage(wrongTypePacket); err != messaging.ErrInvalidPacket {
		t.Errorf("expected ErrInvalidPacket, got: %v", err)
	}

	// Empty payload
	emptyPayloadPacket := protocol.Packet{
		SequenceNumber: 1,
		Type:           protocol.Msg,
		Payload:        nil,
	}
	if _, err := messaging.ParseMessage(emptyPayloadPacket); err != messaging.ErrEmptyPayload {
		t.Errorf("expected ErrEmptyPayload, got: %v", err)
	}

	// Corrupted payload JSON
	badPayloadPacket := protocol.Packet{
		SequenceNumber: 1,
		Type:           protocol.Msg,
		Payload:        []byte("not a json or valid delimited string"),
	}
	if _, err := messaging.ParseMessage(badPayloadPacket); err == nil {
		t.Error("expected error for malformed payload, got nil")
	}

	// JSON with empty sender
	emptySenderJSON, _ := json.Marshal(messaging.Message{Sender: "", Content: "hello", Timestamp: time.Now()})
	if _, err := messaging.ParseMessage(protocol.Packet{Type: protocol.Msg, Payload: emptySenderJSON}); err != messaging.ErrEmptySender {
		t.Errorf("expected ErrEmptySender, got: %v", err)
	}

	// JSON with empty content
	emptyContentJSON, _ := json.Marshal(messaging.Message{Sender: "alice", Content: "", Timestamp: time.Now()})
	if _, err := messaging.ParseMessage(protocol.Packet{Type: protocol.Msg, Payload: emptyContentJSON}); err != messaging.ErrEmptyMessage {
		t.Errorf("expected ErrEmptyMessage, got: %v", err)
	}
}

func TestParseMessageFallbackFormats(t *testing.T) {
	// Pipe-delimited 3 parts: sender|timestamp|content
	ts := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	p1 := protocol.Packet{
		SequenceNumber: 1,
		Type:           protocol.Msg,
		Payload:        []byte("Alice|" + ts.Format(time.RFC3339) + "|Hello pipe!"),
	}

	msg1, err := messaging.ParseMessage(p1)
	if err != nil {
		t.Fatalf("failed to parse 3-part pipe message: %v", err)
	}
	if msg1.Sender != "Alice" || msg1.Content != "Hello pipe!" || !msg1.Timestamp.Equal(ts) {
		t.Errorf("unexpected parsed message: %+v", msg1)
	}

	// Pipe-delimited 2 parts: sender|content
	p2 := protocol.Packet{
		SequenceNumber: 2,
		Type:           protocol.Msg,
		Payload:        []byte("Bob|Hello 2-part pipe!"),
	}

	msg2, err := messaging.ParseMessage(p2)
	if err != nil {
		t.Fatalf("failed to parse 2-part pipe message: %v", err)
	}
	if msg2.Sender != "Bob" || msg2.Content != "Hello 2-part pipe!" || msg2.Timestamp.IsZero() {
		t.Errorf("unexpected parsed message: %+v", msg2)
	}
}

func TestFormatAndDisplayMessage(t *testing.T) {
	msg := messaging.Message{
		Sender:    "satyam",
		Content:   "Quick brown fox jumps over the lazy dog",
		Timestamp: time.Date(2026, 9, 16, 22, 0, 0, 0, time.UTC),
	}

	formatted := messaging.FormatMessage(msg)
	expectedSubstrings := []string{"2026-09-16 22:00:00", "satyam", "Quick brown fox"}
	for _, sub := range expectedSubstrings {
		if !strings.Contains(formatted, sub) {
			t.Errorf("expected formatted message %q to contain %q", formatted, sub)
		}
	}

	// Calling DisplayMessage should not panic
	messaging.DisplayMessage(msg)
}

func TestSendMessageAndReceiveMessageEndToEnd(t *testing.T) {
	receiverConn, err := network.ListenUDP(0)
	if err != nil {
		t.Fatalf("failed to create receiver socket: %v", err)
	}
	defer receiverConn.Close()

	receiverAddr := &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: receiverConn.LocalAddr().(*net.UDPAddr).Port,
	}

	senderConn, err := network.ListenUDP(0)
	if err != nil {
		t.Fatalf("failed to create sender socket: %v", err)
	}
	defer senderConn.Close()

	sender := "peer-1"
	content := "Hello from peer-1 to peer-2!"
	seq := uint32(50)

	var receivedMsg messaging.Message
	var senderAddr *net.UDPAddr
	var recvErr error
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		receivedMsg, senderAddr, recvErr = messaging.ReceiveMessage(receiverConn, 3*time.Second)
	}()

	err = messaging.SendMessage(senderConn, receiverAddr, sender, content, seq)
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	wg.Wait()

	if recvErr != nil {
		t.Fatalf("ReceiveMessage failed: %v", err)
	}

	if receivedMsg.Sender != sender {
		t.Errorf("expected sender %q, got %q", sender, receivedMsg.Sender)
	}

	if receivedMsg.Content != content {
		t.Errorf("expected content %q, got %q", content, receivedMsg.Content)
	}

	if senderAddr == nil || senderAddr.Port != senderConn.LocalAddr().(*net.UDPAddr).Port {
		t.Errorf("expected senderAddr to match sender socket, got %v", senderAddr)
	}
}

func TestSendMessageValidation(t *testing.T) {
	dummyAddr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345}

	conn, err := network.ListenUDP(0)
	if err != nil {
		t.Fatalf("failed to create conn: %v", err)
	}
	defer conn.Close()

	// Nil conn
	if err := messaging.SendMessage(nil, dummyAddr, "alice", "msg", 1); err != messaging.ErrNilConnection {
		t.Errorf("expected ErrNilConnection, got: %v", err)
	}

	// Nil addr
	if err := messaging.SendMessage(conn, nil, "alice", "msg", 1); err != messaging.ErrNilAddress {
		t.Errorf("expected ErrNilAddress, got: %v", err)
	}

	// SendMessagePacket with wrong packet type
	wrongPacket := protocol.Packet{
		SequenceNumber: 1,
		Type:           protocol.TransferAccept,
		Payload:        nil,
	}
	if err := messaging.SendMessagePacket(conn, dummyAddr, wrongPacket); err != messaging.ErrInvalidPacket {
		t.Errorf("expected ErrInvalidPacket, got: %v", err)
	}

	// ReceiveMessage with nil conn
	if _, _, err := messaging.ReceiveMessage(nil, time.Second); err != messaging.ErrNilConnection {
		t.Errorf("expected ErrNilConnection from ReceiveMessage, got: %v", err)
	}
}

func TestMessageReliabilityWithPacketLossAndRetries(t *testing.T) {
	receiverConn, err := network.ListenUDP(0)
	if err != nil {
		t.Fatalf("failed to create receiver socket: %v", err)
	}
	defer receiverConn.Close()

	receiverAddr := &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: receiverConn.LocalAddr().(*net.UDPAddr).Port,
	}

	senderConn, err := network.ListenUDP(0)
	if err != nil {
		t.Fatalf("failed to create sender socket: %v", err)
	}
	defer senderConn.Close()

	sender := "peer-lossy"
	content := "Reliable message over lossy network"
	seq := uint32(77)

	var attempts int32

	// Custom receiver that drops the 1st packet attempt and ACKs the 2nd
	go func() {
		buf := make([]byte, 1500)
		for {
			_ = receiverConn.SetReadDeadline(time.Now().Add(3 * time.Second))
			n, srcAddr, err := receiverConn.ReadFromUDP(buf)
			if err != nil {
				return
			}

			packet, err := protocol.Decode(buf[:n])
			if err != nil || packet.Type != protocol.Msg {
				continue
			}

			if packet.SequenceNumber == seq {
				attempt := atomic.AddInt32(&attempts, 1)
				if attempt == 1 {
					// Drop packet, do not ACK
					continue
				}
				// ACK on retry
				_ = network.SendACK(receiverConn, packet.SequenceNumber, srcAddr)
				return
			}
		}
	}()

	err = messaging.SendMessage(senderConn, receiverAddr, sender, content, seq)
	if err != nil {
		t.Fatalf("SendMessage failed on retry: %v", err)
	}

	if atomic.LoadInt32(&attempts) < 2 {
		t.Errorf("expected at least 2 transmission attempts, got %d", attempts)
	}
}

func TestMessageReceiverDeduplicationAndOrdering(t *testing.T) {
	receiverConn, err := network.ListenUDP(0)
	if err != nil {
		t.Fatalf("failed to create receiver socket: %v", err)
	}
	defer receiverConn.Close()

	receiverAddr := &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: receiverConn.LocalAddr().(*net.UDPAddr).Port,
	}

	senderConn, err := network.ListenUDP(0)
	if err != nil {
		t.Fatalf("failed to create sender socket: %v", err)
	}
	defer senderConn.Close()

	receiver := messaging.NewMessageReceiver(receiverConn, 10)

	// Create packets for seq 10, 11, 12
	p10, _ := messaging.CreateMessage("alice", "Message 10", 10)
	p11, _ := messaging.CreateMessage("alice", "Message 11", 11)
	p12, _ := messaging.CreateMessage("alice", "Message 12", 12)

	enc10, _ := protocol.Encode(p10)
	enc11, _ := protocol.Encode(p11)
	enc12, _ := protocol.Encode(p12)

	// Step 1: Send packet #12 early (should be buffered, 0 delivered)
	_, _ = senderConn.WriteToUDP(enc12, receiverAddr)
	delivered, _, err := receiver.ReceiveNext(1 * time.Second)
	if err != nil {
		t.Fatalf("ReceiveNext failed: %v", err)
	}
	if len(delivered) != 0 {
		t.Errorf("expected 0 delivered for early packet #12, got %d", len(delivered))
	}

	// Step 2: Send duplicate packet #12 (should be ignored, 0 delivered)
	_, _ = senderConn.WriteToUDP(enc12, receiverAddr)
	delivered, _, err = receiver.ReceiveNext(1 * time.Second)
	if err != nil {
		t.Fatalf("ReceiveNext failed: %v", err)
	}
	if len(delivered) != 0 {
		t.Errorf("expected 0 delivered for duplicate packet #12, got %d", len(delivered))
	}

	// Step 3: Send packet #10 (expected, should deliver #10)
	_, _ = senderConn.WriteToUDP(enc10, receiverAddr)
	delivered, _, err = receiver.ReceiveNext(1 * time.Second)
	if err != nil {
		t.Fatalf("ReceiveNext failed: %v", err)
	}
	if len(delivered) != 1 || delivered[0].Content != "Message 10" {
		t.Fatalf("expected [Message 10] delivered, got %v", delivered)
	}

	// Step 4: Send duplicate packet #10 (already delivered, should be ignored)
	_, _ = senderConn.WriteToUDP(enc10, receiverAddr)
	delivered, _, err = receiver.ReceiveNext(1 * time.Second)
	if err != nil {
		t.Fatalf("ReceiveNext failed: %v", err)
	}
	if len(delivered) != 0 {
		t.Errorf("expected 0 delivered for duplicate already-delivered packet #10, got %d", len(delivered))
	}

	// Step 5: Send missing packet #11 (should deliver #11 and buffered #12 in order!)
	_, _ = senderConn.WriteToUDP(enc11, receiverAddr)
	delivered, _, err = receiver.ReceiveNext(1 * time.Second)
	if err != nil {
		t.Fatalf("ReceiveNext failed: %v", err)
	}
	if len(delivered) != 2 {
		t.Fatalf("expected 2 delivered messages [#11, #12], got %d", len(delivered))
	}
	if delivered[0].Content != "Message 11" || delivered[1].Content != "Message 12" {
		t.Errorf("expected [Message 11, Message 12], got [%s, %s]", delivered[0].Content, delivered[1].Content)
	}
}

func TestReceiveMessageTimeout(t *testing.T) {
	conn, err := network.ListenUDP(0)
	if err != nil {
		t.Fatalf("failed to create socket: %v", err)
	}
	defer conn.Close()

	_, _, err = messaging.ReceiveMessage(conn, 50*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}

	if !strings.Contains(err.Error(), "timeout") && !strings.Contains(err.Error(), "deadline") && !strings.Contains(err.Error(), "i/o timeout") {
		t.Errorf("expected timeout related error, got: %v", err)
	}
}

func TestMessageEncodeDecodeChecksumIntegrity(t *testing.T) {
	packet, err := messaging.CreateMessage("alice", "Checksum test message", 88)
	if err != nil {
		t.Fatalf("CreateMessage failed: %v", err)
	}

	encoded, err := protocol.Encode(packet)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := protocol.Decode(encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if !bytes.Equal(decoded.Payload, packet.Payload) {
		t.Errorf("payload mismatch after decode")
	}

	// Corrupt encoded bytes
	encoded[len(encoded)-2] ^= 0xFF
	_, err = protocol.Decode(encoded)
	if err == nil {
		t.Fatal("expected checksum error on corrupted packet, got nil")
	}
}

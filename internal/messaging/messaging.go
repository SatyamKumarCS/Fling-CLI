package messaging

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/SatyamKumarCS/Fling-CLI/internal/network"
	"github.com/SatyamKumarCS/Fling-CLI/internal/protocol"
	"github.com/SatyamKumarCS/Fling-CLI/internal/security"
)

var (
	ErrEmptyMessage   = errors.New("message content cannot be empty")
	ErrEmptySender    = errors.New("sender cannot be empty")
	ErrInvalidPacket  = errors.New("expected MSG packet")
	ErrEmptyPayload   = errors.New("empty message payload")
	ErrNilConnection  = errors.New("connection cannot be nil")
	ErrNilAddress     = errors.New("address cannot be nil")
	ErrMessageTooLong = errors.New("message exceeds maximum allowed size")
)

const (
	// MaxMessageSize represents the maximum size for a short text message payload.
	MaxMessageSize        = 60000
	DefaultReceiveTimeout = 5 * time.Second
)

// Message represents a text message sent between peers.
type Message struct {
	Sender    string    `json:"sender"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	Encrypted bool      `json:"encrypted,omitempty"`
}

// CreateMessage creates a protocol.Packet of type Msg with the given sender, content,
// current timestamp, and sequence number.
func CreateMessage(
	sender string,
	content string,
	sequenceNumber uint32,
) (protocol.Packet, error) {
	return CreateMessageWithTimestamp(sender, content, time.Now(), sequenceNumber)
}

// CreateEncryptedMessage creates a protocol.Packet of type Msg with JSON payload encrypted using AES-256-GCM.
func CreateEncryptedMessage(
	sender string,
	content string,
	key []byte,
	sequenceNumber uint32,
) (protocol.Packet, error) {
	if len(key) == 32 {
		pkt, err := CreateMessageWithTimestamp(sender, content, time.Now(), sequenceNumber)
		if err != nil {
			return protocol.Packet{}, err
		}
		cipherPayload, err := security.Encrypt(key, pkt.Payload)
		if err != nil {
			return protocol.Packet{}, fmt.Errorf("failed to encrypt message: %w", err)
		}
		pkt.Payload = cipherPayload
		return pkt, nil
	}
	return CreateMessage(sender, content, sequenceNumber)
}

// CreateMessageWithTimestamp creates a protocol.Packet of type Msg with a specified timestamp.
func CreateMessageWithTimestamp(
	sender string,
	content string,
	timestamp time.Time,
	sequenceNumber uint32,
) (protocol.Packet, error) {
	if strings.TrimSpace(sender) == "" {
		return protocol.Packet{}, ErrEmptySender
	}
	if strings.TrimSpace(content) == "" {
		return protocol.Packet{}, ErrEmptyMessage
	}

	msg := Message{
		Sender:    sender,
		Content:   content,
		Timestamp: timestamp.Truncate(time.Second),
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		return protocol.Packet{}, fmt.Errorf("failed to encode message payload: %w", err)
	}

	if len(payload) > MaxMessageSize {
		return protocol.Packet{}, ErrMessageTooLong
	}

	return protocol.Packet{
		SequenceNumber: sequenceNumber,
		Type:           protocol.Msg,
		Payload:        payload,
	}, nil
}

// ParseMessageWithKey parses a message packet, decrypting the payload using AES-256-GCM if a key is provided.
func ParseMessageWithKey(packet protocol.Packet, key []byte) (Message, error) {
	if packet.Type != protocol.Msg {
		return Message{}, ErrInvalidPacket
	}
	if len(packet.Payload) == 0 {
		return Message{}, ErrEmptyPayload
	}

	payload := packet.Payload
	wasEncrypted := false
	if len(key) == 32 {
		if decrypted, err := security.Decrypt(key, payload); err == nil {
			payload = decrypted
			wasEncrypted = true
		}
	}

	tempPacket := protocol.Packet{
		SequenceNumber: packet.SequenceNumber,
		Type:           packet.Type,
		Payload:        payload,
	}
	msg, err := ParseMessage(tempPacket)
	if err == nil {
		msg.Encrypted = wasEncrypted
	}
	return msg, err
}

// ParseMessage extracts and validates a Message from a protocol.Packet.
func ParseMessage(packet protocol.Packet) (Message, error) {
	if packet.Type != protocol.Msg {
		return Message{}, ErrInvalidPacket
	}

	if len(packet.Payload) == 0 {
		return Message{}, ErrEmptyPayload
	}

	var msg Message
	err := json.Unmarshal(packet.Payload, &msg)
	if err != nil {
		// Fallback for pipe-delimited format: sender|timestamp|content or sender|content
		raw := string(packet.Payload)
		parts := strings.SplitN(raw, "|", 3)
		if len(parts) == 3 {
			var ts time.Time
			if t, err := time.Parse(time.RFC3339, parts[1]); err == nil {
				ts = t
			} else {
				ts = time.Now()
			}
			msg = Message{
				Sender:    parts[0],
				Timestamp: ts,
				Content:   parts[2],
			}
		} else if len(parts) == 2 {
			msg = Message{
				Sender:    parts[0],
				Timestamp: time.Now(),
				Content:   parts[1],
			}
		} else {
			return Message{}, fmt.Errorf("failed to parse message: %w", err)
		}
	}

	if strings.TrimSpace(msg.Sender) == "" {
		return Message{}, ErrEmptySender
	}
	if strings.TrimSpace(msg.Content) == "" {
		return Message{}, ErrEmptyMessage
	}
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}

	return msg, nil
}

// FormatMessage formats a message with timestamp and sender for clean display.
func FormatMessage(msg Message) string {
	ts := msg.Timestamp.Format("2006-01-02 15:04:05")
	return fmt.Sprintf("[%s] [%s]: %s", ts, msg.Sender, msg.Content)
}

// DisplayMessage prints the formatted message to stdout.
func DisplayMessage(msg Message) {
	fmt.Println(FormatMessage(msg))
}

// SendMessage sends a text message reliably to the specified UDP address.
func SendMessage(
	conn *net.UDPConn,
	addr *net.UDPAddr,
	sender string,
	content string,
	sequenceNumber uint32,
) error {
	return SendMessageWithKey(conn, addr, sender, content, nil, sequenceNumber)
}

// SendMessageWithKey sends a text message (optionally encrypted with key) reliably to the specified UDP address.
func SendMessageWithKey(
	conn *net.UDPConn,
	addr *net.UDPAddr,
	sender string,
	content string,
	key []byte,
	sequenceNumber uint32,
) error {
	if conn == nil {
		return ErrNilConnection
	}
	if addr == nil {
		return ErrNilAddress
	}

	packet, err := CreateEncryptedMessage(sender, content, key, sequenceNumber)
	if err != nil {
		return err
	}

	return SendMessagePacket(conn, addr, packet)
}

// SendMessagePacket sends a pre-created message packet reliably to the specified UDP address.
func SendMessagePacket(
	conn *net.UDPConn,
	addr *net.UDPAddr,
	packet protocol.Packet,
) error {
	if conn == nil {
		return ErrNilConnection
	}
	if addr == nil {
		return ErrNilAddress
	}
	if packet.Type != protocol.Msg {
		return ErrInvalidPacket
	}

	return network.SendReliable(conn, packet, addr)
}

// ReceiveMessage receives a single Msg packet over the UDP connection, immediately ACKs it,
// and parses the Message.
func ReceiveMessage(
	conn *net.UDPConn,
	timeout time.Duration,
) (Message, *net.UDPAddr, error) {
	if conn == nil {
		return Message{}, nil, ErrNilConnection
	}

	buffer := make([]byte, 65535)
	if timeout > 0 {
		_ = conn.SetReadDeadline(time.Now().Add(timeout))
	} else {
		_ = conn.SetReadDeadline(time.Now().Add(DefaultReceiveTimeout))
	}

	for {
		n, senderAddr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			return Message{}, nil, fmt.Errorf("failed to read message from UDP: %w", err)
		}

		packet, err := protocol.Decode(buffer[:n])
		if err != nil {
			// Corrupt checksum or invalid packet, ignore and let sender retry
			continue
		}

		if packet.Type != protocol.Msg {
			// Ignore non-message packets in this receiver
			continue
		}

		// Always ACK the message packet immediately (no receiver consent needed)
		_ = network.SendACK(conn, packet.SequenceNumber, senderAddr)

		msg, err := ParseMessage(packet)
		if err != nil {
			return Message{}, senderAddr, fmt.Errorf("invalid message: %w", err)
		}

		return msg, senderAddr, nil
	}
}

// MessageReceiver manages ordering, deduplication, and ACK handling for incoming peer messages.
type MessageReceiver struct {
	conn            *net.UDPConn
	orderedReceiver *network.OrderedReceiver
}

// NewMessageReceiver creates a MessageReceiver starting with the expected sequence number.
func NewMessageReceiver(conn *net.UDPConn, startSequence uint32) *MessageReceiver {
	return &MessageReceiver{
		conn:            conn,
		orderedReceiver: network.NewOrderedReceiver(startSequence),
	}
}

// ReceiveNext waits for incoming UDP packets within the timeout, immediately ACKs all
// received MSG packets (including duplicates), processes them through the ordered receiver,
// and returns any delivered deduplicated in-order Messages.
func (r *MessageReceiver) ReceiveNext(timeout time.Duration) ([]Message, *net.UDPAddr, error) {
	if r.conn == nil {
		return nil, nil, ErrNilConnection
	}

	buffer := make([]byte, 65535)
	if timeout > 0 {
		_ = r.conn.SetReadDeadline(time.Now().Add(timeout))
	} else {
		_ = r.conn.SetReadDeadline(time.Now().Add(DefaultReceiveTimeout))
	}

	n, senderAddr, err := r.conn.ReadFromUDP(buffer)
	if err != nil {
		return nil, nil, fmt.Errorf("read failed: %w", err)
	}

	packet, err := protocol.Decode(buffer[:n])
	if err != nil {
		// Ignore corrupted packet
		return nil, senderAddr, nil
	}

	if packet.Type != protocol.Msg {
		// Ignore non-Msg packet
		return nil, senderAddr, nil
	}

	// Always send ACK immediately for reliable UDP
	_ = network.SendACK(r.conn, packet.SequenceNumber, senderAddr)

	deliveredPackets := r.orderedReceiver.Receive(packet)
	var messages []Message
	for _, p := range deliveredPackets {
		msg, err := ParseMessage(p)
		if err == nil {
			messages = append(messages, msg)
		}
	}

	return messages, senderAddr, nil
}

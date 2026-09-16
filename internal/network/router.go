package network

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/SatyamKumarCS/Fling-CLI/internal/protocol"
)

// Router multiplexes incoming and outgoing UDP packets on a single net.UDPConn,
// eliminating socket read contention and managing reliable ACK dispatching.
type Router struct {
	conn       *net.UDPConn
	mu         sync.RWMutex
	ackWaiters map[uint32]chan protocol.Packet
	respWait   map[uint32]chan protocol.Packet

	// Packet Handlers
	OnPresence        func(p protocol.Packet, addr *net.UDPAddr)
	OnMsg             func(p protocol.Packet, addr *net.UDPAddr)
	OnTransferRequest func(p protocol.Packet, addr *net.UDPAddr)
	OnTransferAccept  func(p protocol.Packet, addr *net.UDPAddr)
	OnTransferReject  func(p protocol.Packet, addr *net.UDPAddr)
	OnFileChunk       func(p protocol.Packet, addr *net.UDPAddr)
	OnFileEnd         func(p protocol.Packet, addr *net.UDPAddr)

	stopChan chan struct{}
	closed   bool
}

// NewRouter creates a new UDP packet router for the given connection.
func NewRouter(conn *net.UDPConn) *Router {
	return &Router{
		conn:       conn,
		ackWaiters: make(map[uint32]chan protocol.Packet),
		respWait:   make(map[uint32]chan protocol.Packet),
		stopChan:   make(chan struct{}),
	}
}

// Conn returns the underlying UDP connection.
func (r *Router) Conn() *net.UDPConn {
	return r.conn
}

// Start begins the single-threaded UDP read loop in a background goroutine.
func (r *Router) Start() {
	go r.readLoop()
}

// Close stops the router and closes the UDP socket.
func (r *Router) Close() {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return
	}
	r.closed = true
	close(r.stopChan)
	r.mu.Unlock()

	if r.conn != nil {
		_ = r.conn.Close()
	}
}

func (r *Router) readLoop() {
	buffer := make([]byte, 65535)
	for {
		select {
		case <-r.stopChan:
			return
		default:
		}

		_ = r.conn.SetReadDeadline(time.Time{})
		n, senderAddr, err := r.conn.ReadFromUDP(buffer)
		if err != nil {
			select {
			case <-r.stopChan:
				return
			default:
				// Socket error or closed
				return
			}
		}

		packet, err := protocol.Decode(buffer[:n])
		if err != nil {
			continue
		}

		switch packet.Type {
		case protocol.ACK:
			r.dispatchAck(packet)

		case protocol.Presence:
			if r.OnPresence != nil {
				r.OnPresence(packet, senderAddr)
			}

		case protocol.Msg:
			// Automatically ACK incoming message
			_ = SendACK(r.conn, packet.SequenceNumber, senderAddr)
			if r.OnMsg != nil {
				r.OnMsg(packet, senderAddr)
			}

		case protocol.TransferRequest:
			// Automatically ACK transfer request packet
			_ = SendACK(r.conn, packet.SequenceNumber, senderAddr)
			if r.OnTransferRequest != nil {
				r.OnTransferRequest(packet, senderAddr)
			}

		case protocol.TransferAccept:
			// Dispatch to waiting sender and callback
			_ = SendACK(r.conn, packet.SequenceNumber, senderAddr)
			r.dispatchResponse(packet)
			if r.OnTransferAccept != nil {
				r.OnTransferAccept(packet, senderAddr)
			}

		case protocol.TransferReject:
			// Dispatch to waiting sender and callback
			_ = SendACK(r.conn, packet.SequenceNumber, senderAddr)
			r.dispatchResponse(packet)
			if r.OnTransferReject != nil {
				r.OnTransferReject(packet, senderAddr)
			}

		case protocol.FileChunk:
			// Automatically ACK chunk packet
			_ = SendACK(r.conn, packet.SequenceNumber, senderAddr)
			if r.OnFileChunk != nil {
				r.OnFileChunk(packet, senderAddr)
			}

		case protocol.FileEnd:
			// Automatically ACK file end packet
			_ = SendACK(r.conn, packet.SequenceNumber, senderAddr)
			if r.OnFileEnd != nil {
				r.OnFileEnd(packet, senderAddr)
			}
		}
	}
}

func (r *Router) dispatchAck(packet protocol.Packet) {
	r.mu.RLock()
	ch, ok := r.ackWaiters[packet.SequenceNumber]
	r.mu.RUnlock()

	if ok {
		select {
		case ch <- packet:
		default:
		}
	}
}

func (r *Router) dispatchResponse(packet protocol.Packet) {
	r.mu.RLock()
	ch, ok := r.respWait[packet.SequenceNumber]
	if !ok {
		// Also check sequence - 1 in case request was seq N and response is seq N+1
		ch, ok = r.respWait[packet.SequenceNumber-1]
	}
	r.mu.RUnlock()

	if ok {
		select {
		case ch <- packet:
		default:
		}
	}
}

// RegisterAckWaiter registers a channel to wait for an ACK with the given sequence number.
func (r *Router) RegisterAckWaiter(seqNum uint32) chan protocol.Packet {
	r.mu.Lock()
	defer r.mu.Unlock()
	ch := make(chan protocol.Packet, 2)
	r.ackWaiters[seqNum] = ch
	return ch
}

// DeregisterAckWaiter cleans up the ACK waiter channel.
func (r *Router) DeregisterAckWaiter(seqNum uint32) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.ackWaiters, seqNum)
}

// RegisterResponseWaiter registers a channel to wait for a negotiation response (Accept/Reject).
func (r *Router) RegisterResponseWaiter(seqNum uint32) chan protocol.Packet {
	r.mu.Lock()
	defer r.mu.Unlock()
	ch := make(chan protocol.Packet, 2)
	r.respWait[seqNum] = ch
	return ch
}

// DeregisterResponseWaiter cleans up the negotiation response channel.
func (r *Router) DeregisterResponseWaiter(seqNum uint32) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.respWait, seqNum)
}

// SendReliable transmits a packet reliably via the Router, waiting for an ACK on a dedicated channel.
func (r *Router) SendReliable(packet protocol.Packet, addr *net.UDPAddr) error {
	encoded, err := protocol.Encode(packet)
	if err != nil {
		return err
	}

	ackCh := r.RegisterAckWaiter(packet.SequenceNumber)
	defer r.DeregisterAckWaiter(packet.SequenceNumber)

	for attempt := 1; attempt <= MaxRetries; attempt++ {
		_, err := r.conn.WriteToUDP(encoded, addr)
		if err != nil {
			return err
		}

		select {
		case ack := <-ackCh:
			if ack.SequenceNumber == packet.SequenceNumber {
				return nil
			}
		case <-time.After(AckTimeout):
			// Retry
		case <-r.stopChan:
			return fmt.Errorf("router closed while waiting for ACK")
		}
	}

	return fmt.Errorf("packet #%d failed after %d retries", packet.SequenceNumber, MaxRetries)
}

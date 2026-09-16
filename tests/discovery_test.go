package tests

import (
	"testing"
	"time"

	"github.com/SatyamKumarCS/Fling-CLI/internal/discovery"
	"github.com/SatyamKumarCS/Fling-CLI/internal/protocol"
)

func TestCreateAndParsePresencePacket(t *testing.T) {
	hostname := "alice-pc"
	sessionID := "sess-abc123"
	port := 9999
	seq := uint32(5)

	packet := discovery.CreatePresencePacket(hostname, sessionID, port, seq)

	if packet.Type != protocol.Presence {
		t.Errorf("expected packet type Presence (%d), got %d", protocol.Presence, packet.Type)
	}

	if packet.SequenceNumber != seq {
		t.Errorf("expected sequence %d, got %d", seq, packet.SequenceNumber)
	}

	peer, err := discovery.ParsePresencePacket(packet)
	if err != nil {
		t.Fatalf("ParsePresencePacket failed: %v", err)
	}

	if peer.Hostname != hostname {
		t.Errorf("expected hostname %q, got %q", hostname, peer.Hostname)
	}

	if peer.SessionID != sessionID {
		t.Errorf("expected sessionID %q, got %q", sessionID, peer.SessionID)
	}

	if peer.Port != port {
		t.Errorf("expected port %d, got %d", port, peer.Port)
	}
}

func TestParsePresencePacketErrors(t *testing.T) {
	// Wrong packet type
	wrongType := protocol.Packet{
		SequenceNumber: 1,
		Type:           protocol.Msg,
		Payload:        []byte("alice|sess-123|9999"),
	}
	if _, err := discovery.ParsePresencePacket(wrongType); err == nil {
		t.Error("expected error for wrong packet type, got nil")
	}

	// Empty payload
	emptyPayload := protocol.Packet{
		SequenceNumber: 1,
		Type:           protocol.Presence,
		Payload:        []byte(""),
	}
	if _, err := discovery.ParsePresencePacket(emptyPayload); err == nil {
		t.Error("expected error for empty payload, got nil")
	}
}

func TestDiscoveryHandlePresenceAndGetPeers(t *testing.T) {
	disco := discovery.NewDiscovery("my-node", "my-session-id", 9999)

	// Packet from self (should be ignored)
	selfPacket := discovery.CreatePresencePacket("my-node", "my-session-id", 9999, 1)
	if _, ok := disco.HandlePresence(selfPacket, "127.0.0.1"); ok {
		t.Error("expected self presence to be ignored, got ok = true")
	}

	if len(disco.GetPeers()) != 0 {
		t.Errorf("expected 0 peers, got %d", len(disco.GetPeers()))
	}

	// Packet from external peer
	peerPacket := discovery.CreatePresencePacket("bob-macbook", "bob-session-456", 9998, 2)
	peer, isNew := disco.HandlePresence(peerPacket, "192.168.1.50")
	if !isNew {
		t.Fatal("expected peer presence to be marked as new peer")
	}

	if peer.Hostname != "bob-macbook" {
		t.Errorf("expected hostname 'bob-macbook', got %q", peer.Hostname)
	}

	if peer.Addr() != "192.168.1.50:9998" {
		t.Errorf("expected peer addr '192.168.1.50:9998', got %q", peer.Addr())
	}

	// Subsequent presence from same peer is not new
	_, isNew = disco.HandlePresence(peerPacket, "192.168.1.50")
	if isNew {
		t.Error("expected existing peer presence to return isNew = false")
	}

	peers := disco.GetPeers()
	if len(peers) != 1 {
		t.Fatalf("expected 1 peer, got %d", len(peers))
	}
	if peers[0].SessionID != "bob-session-456" {
		t.Errorf("expected session 'bob-session-456', got %q", peers[0].SessionID)
	}

	// Test FindPeer by peer number ("1" and "#1")
	found, exists := disco.FindPeer("1")
	if !exists || found.SessionID != "bob-session-456" {
		t.Errorf("FindPeer by peer number '1' failed: exists=%v, found=%+v", exists, found)
	}

	found, exists = disco.FindPeer("#1")
	if !exists || found.SessionID != "bob-session-456" {
		t.Errorf("FindPeer by peer number '#1' failed: exists=%v, found=%+v", exists, found)
	}

	// Test FindPeer by hostname
	found, exists = disco.FindPeer("bob-macbook")
	if !exists || found.SessionID != "bob-session-456" {
		t.Errorf("FindPeer by hostname failed: exists=%v, found=%+v", exists, found)
	}

	// Test FindPeer by session ID prefix
	found, exists = disco.FindPeer("bob-sess")
	if !exists || found.SessionID != "bob-session-456" {
		t.Errorf("FindPeer by session prefix failed: exists=%v, found=%+v", exists, found)
	}

	// Test FindPeer by Addr
	found, exists = disco.FindPeer("192.168.1.50:9998")
	if !exists || found.Hostname != "bob-macbook" {
		t.Errorf("FindPeer by addr failed: exists=%v, found=%+v", exists, found)
	}

	// Non-existent peer
	_, exists = disco.FindPeer("unknown-peer")
	if exists {
		t.Error("expected non-existent peer to return exists = false")
	}
}

func TestDiscoveryPeerTimeoutPruning(t *testing.T) {
	disco := discovery.NewDiscovery("node-a", "sess-a", 9999)

	// Add peer with stale LastSeen time
	stalePeer := discovery.Peer{
		Hostname:  "stale-node",
		IP:        "10.0.0.1",
		Port:      9999,
		SessionID: "stale-sess",
		LastSeen:  time.Now().Add(-15 * time.Second), // Older than PeerTimeout (10s)
	}
	disco.Peers[stalePeer.SessionID] = stalePeer

	// PruneStalePeers should remove the stale peer
	removed := disco.PruneStalePeers()
	if len(removed) != 1 || removed[0].SessionID != "stale-sess" {
		t.Fatalf("expected 1 removed stale peer, got: %v", removed)
	}

	// GetPeers should now return 0 peers
	peers := disco.GetPeers()
	if len(peers) != 0 {
		t.Errorf("expected 0 peers after pruning, got %d", len(peers))
	}
}

package discovery

import (
	"encoding/json"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/SatyamKumarCS/Fling-CLI/internal/protocol"
)

const (
	DiscoveryPort = 9999
	PeerTimeout   = 10 * time.Second
)

type Discovery struct {
	Hostname  string
	SessionID string
	Port      int
	Peers     map[string]Peer
	mu        sync.RWMutex
}

// NewDiscovery creates a new Discovery instance.
func NewDiscovery(hostname string, sessionID string, port int) *Discovery {
	if port <= 0 {
		port = DiscoveryPort
	}
	return &Discovery{
		Hostname:  hostname,
		SessionID: sessionID,
		Port:      port,
		Peers:     make(map[string]Peer),
	}
}

// GetLocalIPs returns all active non-loopback IPv4 addresses of the host.
func GetLocalIPs() []string {
	var ips []string
	ifaces, err := net.Interfaces()
	if err != nil {
		return ips
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ipNet, ok := a.(*net.IPNet)
			if ok && ipNet.IP.To4() != nil {
				ips = append(ips, ipNet.IP.To4().String())
			}
		}
	}
	return ips
}

// CreatePresencePacket creates a protocol.Presence packet containing the peer's metadata.
func CreatePresencePacket(
	hostname string,
	sessionID string,
	port int,
	sequenceNumber uint32,
) protocol.Packet {
	if port <= 0 {
		port = DiscoveryPort
	}
	payload := fmt.Sprintf("%s|%s|%d", hostname, sessionID, port)
	return protocol.Packet{
		SequenceNumber: sequenceNumber,
		Type:           protocol.Presence,
		Payload:        []byte(payload),
	}
}

// ParsePresencePacket decodes a protocol.Presence packet into a Peer struct.
func ParsePresencePacket(packet protocol.Packet) (Peer, error) {
	if packet.Type != protocol.Presence {
		return Peer{}, fmt.Errorf("expected PRESENCE packet")
	}

	raw := string(packet.Payload)
	parts := strings.Split(raw, "|")
	if len(parts) >= 2 {
		port := DiscoveryPort
		if len(parts) >= 3 {
			if p, err := strconv.Atoi(parts[2]); err == nil && p > 0 {
				port = p
			}
		}
		return Peer{
			Hostname:  parts[0],
			SessionID: parts[1],
			Port:      port,
			LastSeen:  time.Now(),
		}, nil
	}

	// Try JSON fallback
	var peer Peer
	if err := json.Unmarshal(packet.Payload, &peer); err == nil && peer.Hostname != "" {
		if peer.Port <= 0 {
			peer.Port = DiscoveryPort
		}
		peer.LastSeen = time.Now()
		return peer, nil
	}

	return Peer{}, fmt.Errorf("invalid presence packet payload")
}

// HandlePresence updates the peer map with incoming presence announcement.
// Returns the updated peer and true if this is a newly discovered peer (not previously seen or previously expired).
func (d *Discovery) HandlePresence(packet protocol.Packet, remoteIP string) (Peer, bool) {
	peer, err := ParsePresencePacket(packet)
	if err != nil {
		return Peer{}, false
	}

	// Ignore our own presence broadcasts
	if peer.SessionID == d.SessionID {
		return Peer{}, false
	}

	peer.IP = remoteIP
	if peer.Port <= 0 {
		peer.Port = DiscoveryPort
	}
	peer.LastSeen = time.Now()

	d.mu.Lock()
	defer d.mu.Unlock()

	_, exists := d.Peers[peer.SessionID]
	d.Peers[peer.SessionID] = peer

	return peer, !exists
}

// PruneStalePeers removes peers that have not been seen for longer than PeerTimeout.
// Returns a slice of peers that were removed.
func (d *Discovery) PruneStalePeers() []Peer {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()
	var removed []Peer
	for id, peer := range d.Peers {
		if now.Sub(peer.LastSeen) > PeerTimeout {
			removed = append(removed, peer)
			delete(d.Peers, id)
		}
	}
	return removed
}

// GetPeers returns a slice of active peers, sorted consistently.
func (d *Discovery) GetPeers() []Peer {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()
	var active []Peer
	for id, peer := range d.Peers {
		if now.Sub(peer.LastSeen) > PeerTimeout {
			delete(d.Peers, id)
			continue
		}
		active = append(active, peer)
	}

	// Sort peers consistently by Hostname, then SessionID
	sort.Slice(active, func(i, j int) bool {
		if active[i].Hostname != active[j].Hostname {
			return active[i].Hostname < active[j].Hostname
		}
		return active[i].SessionID < active[j].SessionID
	})

	return active
}

// FindPeer searches for a peer by peer number (e.g. "1", "#1"), hostname, IP:Port, IP, or session ID.
func (d *Discovery) FindPeer(target string) (Peer, bool) {
	peers := d.GetPeers()
	targetTrimmed := strings.TrimSpace(target)
	targetLower := strings.ToLower(targetTrimmed)

	// 1. Match by peer number (1-indexed, e.g. "1", "2", "#1")
	numStr := strings.TrimPrefix(targetTrimmed, "#")
	if idx, err := strconv.Atoi(numStr); err == nil && idx >= 1 && idx <= len(peers) {
		return peers[idx-1], true
	}

	// 2. Exact match by session ID or session ID prefix
	for _, p := range peers {
		if strings.ToLower(p.SessionID) == targetLower || strings.HasPrefix(strings.ToLower(p.SessionID), targetLower) {
			return p, true
		}
	}

	// 3. Match by hostname (case-insensitive)
	for _, p := range peers {
		if strings.ToLower(p.Hostname) == targetLower {
			return p, true
		}
	}

	// 4. Match by Addr (IP:Port) or IP
	for _, p := range peers {
		if p.Addr() == targetTrimmed || p.IP == targetTrimmed {
			return p, true
		}
	}

	return Peer{}, false
}

// BroadcastPresence broadcasts a presence packet over the network to LAN broadcast and loopback ports.
func (d *Discovery) BroadcastPresence(conn *net.UDPConn, targetPort int) error {
	if targetPort <= 0 {
		targetPort = DiscoveryPort
	}

	packet := CreatePresencePacket(d.Hostname, d.SessionID, d.Port, 0)
	encoded, err := protocol.Encode(packet)
	if err != nil {
		return err
	}

	// 1. Send via main listening socket if provided
	if conn != nil {
		destAddrs := getBroadcastDestinations(targetPort, d.Port)
		for _, addr := range destAddrs {
			_, _ = conn.WriteToUDP(encoded, addr)
		}

		// Also send directly to any known peer addresses
		d.mu.RLock()
		for _, p := range d.Peers {
			if p.Port > 0 && p.IP != "" {
				peerAddr := &net.UDPAddr{
					IP:   net.ParseIP(p.IP),
					Port: p.Port,
				}
				_, _ = conn.WriteToUDP(encoded, peerAddr)
			}
		}
		d.mu.RUnlock()
	}

	// 2. Send via interface-specific sockets for physical LAN routing on macOS/Linux
	sendInterfaceBroadcasts(encoded, targetPort)

	return nil
}

func sendInterfaceBroadcasts(encoded []byte, targetPort int) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ipNet, ok := a.(*net.IPNet)
			if !ok || ipNet.IP.To4() == nil {
				continue
			}
			ip := ipNet.IP.To4()
			mask := ipNet.Mask
			if len(mask) == 4 {
				bcastIP := net.IPv4(
					ip[0]|^mask[0],
					ip[1]|^mask[1],
					ip[2]|^mask[2],
					ip[3]|^mask[3],
				)

				// Bind to interface local IP:0 to force transmission out of this specific interface
				localAddr := &net.UDPAddr{IP: ip, Port: 0}
				ifaceConn, err := net.ListenUDP("udp4", localAddr)
				if err == nil {
					_, _ = ifaceConn.WriteToUDP(encoded, &net.UDPAddr{IP: bcastIP, Port: targetPort})
					_, _ = ifaceConn.WriteToUDP(encoded, &net.UDPAddr{IP: net.IPv4bcast, Port: targetPort})
					ifaceConn.Close()
				}
			}
		}
	}
}

func getBroadcastDestinations(defaultPort int, selfPort int) []*net.UDPAddr {
	var addrs []*net.UDPAddr
	seen := make(map[string]bool)

	addAddr := func(ip net.IP, port int) {
		if ip == nil {
			return
		}
		key := fmt.Sprintf("%s:%d", ip.String(), port)
		if !seen[key] {
			seen[key] = true
			addrs = append(addrs, &net.UDPAddr{IP: ip, Port: port})
		}
	}

	ports := []int{defaultPort, 9999, 9998, 9997, 9996}
	if selfPort > 0 {
		ports = append(ports, selfPort)
	}

	for _, port := range ports {
		// Global broadcast
		addAddr(net.IPv4bcast, port)
		// Localhost loopback
		addAddr(net.ParseIP("127.0.0.1"), port)
	}

	// Enumerate all active network interfaces to find subnet broadcast addresses
	ifaces, err := net.Interfaces()
	if err == nil {
		for _, iface := range ifaces {
			if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
				continue
			}
			ifAddrs, err := iface.Addrs()
			if err != nil {
				continue
			}
			for _, a := range ifAddrs {
				ipNet, ok := a.(*net.IPNet)
				if !ok || ipNet.IP.To4() == nil {
					continue
				}
				ip := ipNet.IP.To4()
				mask := ipNet.Mask
				if len(mask) == 4 {
					bcast := net.IPv4(
						ip[0]|^mask[0],
						ip[1]|^mask[1],
						ip[2]|^mask[2],
						ip[3]|^mask[3],
					)
					addAddr(bcast, defaultPort)
					addAddr(bcast, 9999)
				}
			}
		}
	}

	return addrs
}

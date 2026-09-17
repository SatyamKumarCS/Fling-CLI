package discovery

import (
	"fmt"
	"time"
)

type Peer struct {
	Hostname  string    `json:"hostname"`
	IP        string    `json:"ip"`
	Port      int       `json:"port"`
	SessionID string    `json:"sessionId"`
	PublicKey []byte    `json:"publicKey,omitempty"`
	SharedKey []byte    `json:"-"`
	LastSeen  time.Time `json:"lastSeen"`
}

// Addr returns the IP:Port address string for this peer.
func (p Peer) Addr() string {
	port := p.Port
	if port <= 0 {
		port = DiscoveryPort
	}
	return fmt.Sprintf("%s:%d", p.IP, port)
}

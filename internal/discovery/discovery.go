package discovery

import (
	"time"
)

const (
	DiscoveryPort = 9999
	PeerTimeout   = 10 * time.Second
)

type Discovery struct {
	Hostname  string
	SessionID string
	Peers     map[string]Peer
}

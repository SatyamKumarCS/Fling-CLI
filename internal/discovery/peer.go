package discovery

import "time"

type Peer struct {
	Hostname  string
	IP        string
	SessionID string
	LastSeen  time.Time
}

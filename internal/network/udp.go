package network

import (
	"net"
)

func ListenUDP(port int) (*net.UDPConn, error) {
	addr := &net.UDPAddr{
		IP:   net.IPv4zero,
		Port: port,
	}

	conn, err := net.ListenUDP("udp4", addr)

	if err != nil {
		return nil, err
	}

	return conn, nil
}

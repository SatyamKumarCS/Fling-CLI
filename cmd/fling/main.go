package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/SatyamKumarCS/Fling-CLI/internal/cli"
	"github.com/SatyamKumarCS/Fling-CLI/internal/discovery"
	"github.com/SatyamKumarCS/Fling-CLI/internal/handshake"
	"github.com/SatyamKumarCS/Fling-CLI/internal/messaging"
	"github.com/SatyamKumarCS/Fling-CLI/internal/network"
	"github.com/SatyamKumarCS/Fling-CLI/internal/protocol"
	"github.com/SatyamKumarCS/Fling-CLI/internal/transfer"
	"github.com/SatyamKumarCS/Fling-CLI/internal/tui"
)

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		runTUI(discovery.DiscoveryPort, "")
		return
	}

	switch args[0] {
	case "help", "--help", "-h":
		cli.PrintBanner()
		cli.PrintHelp()
	case "version", "--version", "-v":
		fmt.Printf("Fling CLI v%s\n", cli.Version)
	case "uninstall":
		runUninstall()
	case "send":
		runSend(args[1:])
	case "msg", "message":
		runMsg(args[1:])
	case "peers", "list":
		runPeers(args[1:])
	default:
		port := parsePortFlag(args)
		if port <= 0 {
			port = discovery.DiscoveryPort
		}
		peer := parsePeerFlag(args)
		// Support "fling 10.7.12.154" directly
		if peer == "" && len(args) == 1 && (net.ParseIP(args[0]) != nil || strings.Contains(args[0], ".")) {
			peer = args[0]
		}

		if peer != "" || port != discovery.DiscoveryPort || hasFlag(args, "--port", "-p", "--peer", "-c", "--connect") {
			runTUI(port, peer)
			return
		}

		fmt.Printf("Unknown command or invalid arguments: %s\n\n", strings.Join(args, " "))
		cli.PrintHelp()
		os.Exit(1)
	}
}

func parsePeerFlag(args []string) string {
	for i := 0; i < len(args); i++ {
		if (args[i] == "--peer" || args[i] == "-c" || args[i] == "--connect") && i+1 < len(args) {
			return args[i+1]
		}
		if strings.HasPrefix(args[i], "--peer=") {
			return strings.TrimPrefix(args[i], "--peer=")
		}
		if strings.HasPrefix(args[i], "--connect=") {
			return strings.TrimPrefix(args[i], "--connect=")
		}
	}
	return ""
}

func hasFlag(args []string, flags ...string) bool {
	for _, arg := range args {
		for _, f := range flags {
			if arg == f || strings.HasPrefix(arg, f+"=") {
				return true
			}
		}
	}
	return false
}

func parsePortFlag(args []string) int {
	for i := 0; i < len(args); i++ {
		if (args[i] == "--port" || args[i] == "-p") && i+1 < len(args) {
			if p, err := strconv.Atoi(args[i+1]); err == nil && p > 0 && p <= 65535 {
				return p
			}
		}
		if strings.HasPrefix(args[i], "--port=") {
			val := strings.TrimPrefix(args[i], "--port=")
			if p, err := strconv.Atoi(val); err == nil && p > 0 && p <= 65535 {
				return p
			}
		}
	}
	return 0
}

func generateSessionID() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%08x", time.Now().UnixNano()&0xFFFFFFFF)
	}
	return hex.EncodeToString(b)
}

func runTUI(port int, initialPeer string) {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "fling-node"
	}
	sessionID := generateSessionID()

	conn, err := network.ListenUDP(port)
	if err != nil {
		if port == discovery.DiscoveryPort {
			fallbackPort := 9998
			conn, err = network.ListenUDP(fallbackPort)
			if err != nil {
				fmt.Printf("[ERROR] Failed to bind to UDP ports: %v\n", err)
				os.Exit(1)
			}
			port = fallbackPort
		} else {
			fmt.Printf("[ERROR] Failed to bind to UDP port %d: %v\n", port, err)
			os.Exit(1)
		}
	}

	disco := discovery.NewDiscovery(hostname, sessionID, port)
	router := network.NewRouter(conn)

	// If an initial peer IP was provided via CLI, ping it immediately
	if initialPeer != "" {
		_ = disco.PingPeer(conn, initialPeer, discovery.DiscoveryPort)
	}

	// Create Bubbletea TUI Model
	model := tui.NewModel(hostname, sessionID, port, router, disco)
	p := tea.NewProgram(model, tea.WithAltScreen())
	model.Program = p

	// Wire router event handlers to Bubbletea program
	router.OnPresence = func(packet protocol.Packet, addr *net.UDPAddr) {
		peer, isNew := disco.HandlePresence(packet, addr.IP.String())
		if isNew {
			p.Send(tui.PeerEventMsg{})
			// Immediate bidirectional presence response: reply directly to sender
			respPacket := discovery.CreatePresencePacket(disco.Hostname, disco.SessionID, disco.Port, 0)
			if encoded, err := protocol.Encode(respPacket); err == nil {
				// Reply to sender's source port
				_, _ = router.Conn().WriteToUDP(encoded, addr)
				// Also reply to their service port if different
				if peer.Port > 0 && peer.Port != addr.Port {
					_, _ = router.Conn().WriteToUDP(encoded, &net.UDPAddr{
						IP:   addr.IP,
						Port: peer.Port,
					})
				}
			}
		}
	}

	router.OnMsg = func(packet protocol.Packet, addr *net.UDPAddr) {
		msg, err := messaging.ParseMessage(packet)
		if err == nil {
			p.Send(tui.NewIncomingMessageMsg(msg))
		}
	}

	router.OnTransferRequest = func(packet protocol.Packet, addr *net.UDPAddr) {
		req, err := handshake.ParseTransferRequest(packet)
		if err == nil {
			p.Send(tui.IncomingTransferReqMsg{
				Request:    req,
				SenderAddr: addr,
				SeqNum:     packet.SequenceNumber,
			})
		}
	}

	router.OnFileChunk = func(packet protocol.Packet, addr *net.UDPAddr) {
		p.Send(tui.FileChunkReceivedMsg(packet))
	}

	router.OnFileEnd = func(packet protocol.Packet, addr *net.UDPAddr) {
		p.Send(tui.FileEndReceivedMsg(packet))
	}

	// Start router reader loop in background
	router.Start()
	defer router.Close()

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running Fling TUI: %v\n", err)
		os.Exit(1)
	}
}

func runSend(args []string) {
	var filePath string
	var targetPeer string

	for i := 0; i < len(args); i++ {
		if (args[i] == "--to" || args[i] == "-t") && i+1 < len(args) {
			targetPeer = args[i+1]
			i++
		} else if strings.HasPrefix(args[i], "--to=") {
			targetPeer = strings.TrimPrefix(args[i], "--to=")
		} else if filePath == "" && !strings.HasPrefix(args[i], "-") {
			filePath = args[i]
		}
	}

	if filePath == "" || targetPeer == "" {
		fmt.Println("Usage: fling send <file> --to <peer>")
		os.Exit(1)
	}

	resolvedPath, fileInfo, err := transfer.ResolveFilePath(filePath)
	if err != nil {
		fmt.Printf("[ERROR] File not found: %s\n", filePath)
		os.Exit(1)
	}
	if fileInfo.IsDir() {
		fmt.Printf("[ERROR] %s is a directory; only single files are supported\n", filePath)
		os.Exit(1)
	}
	filePath = resolvedPath

	checksum, size, err := transfer.CalculateFileChecksum(filePath)
	if err != nil {
		fmt.Printf("[ERROR] Failed to calculate file checksum: %v\n", err)
		os.Exit(1)
	}

	targetAddr, err := resolvePeerAddr(targetPeer)
	if err != nil {
		fmt.Printf("[ERROR] Failed to resolve peer %q: %v\n", targetPeer, err)
		os.Exit(1)
	}

	conn, err := network.ListenUDP(0)
	if err != nil {
		fmt.Printf("[ERROR] Failed to create socket: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D56F4"))
	fmt.Printf("%s Requesting transfer of %s (%s) to %s...\n", headerStyle.Render("[HANDSHAKE]"), filepath.Base(filePath), cli.FormatBytes(size), targetAddr)

	reqPacket := handshake.CreateTransferRequest(filepath.Base(filePath), size, checksum, 1)
	err = network.SendReliable(conn, reqPacket, targetAddr)
	if err != nil {
		fmt.Printf("[ERROR] Transfer request failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("[HANDSHAKE] Waiting for peer confirmation...")
	_ = conn.SetReadDeadline(time.Now().Add(45 * time.Second))

	buffer := make([]byte, 2048)
	var accepted bool
	for {
		n, _, err := conn.ReadFromUDP(buffer)
		if err != nil {
			fmt.Printf("[ERROR] Timed out waiting for peer response: %v\n", err)
			os.Exit(1)
		}

		packet, err := protocol.Decode(buffer[:n])
		if err != nil {
			continue
		}

		if packet.Type == protocol.TransferReject {
			fmt.Println("[TRANSFER REJECTED] The peer declined the file transfer.")
			os.Exit(1)
		}

		if packet.Type == protocol.TransferAccept {
			accepted = true
			break
		}
	}

	if !accepted {
		fmt.Println("[TRANSFER REJECTED] Transfer was not accepted.")
		os.Exit(1)
	}

	fmt.Printf("[TRANSFER ACCEPTED] Peer accepted. Sending %s...\n", filepath.Base(filePath))
	pb := cli.NewProgressBar(size, filepath.Base(filePath))

	_, err = transfer.SendFile(
		conn,
		targetAddr,
		filePath,
		2,
		func(transferred, total int64) {
			pb.Update(transferred)
		},
	)

	if err != nil {
		fmt.Printf("\n[ERROR] Transfer failed: %v\n", err)
		os.Exit(1)
	}

	pb.Finish()
	successStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#00E676"))
	fmt.Printf("%s Transfer completed successfully! (Checksum: 0x%08x)\n", successStyle.Render("[SUCCESS]"), checksum)
}

func runMsg(args []string) {
	var text string
	var targetPeer string

	for i := 0; i < len(args); i++ {
		if (args[i] == "--to" || args[i] == "-t") && i+1 < len(args) {
			targetPeer = args[i+1]
			i++
		} else if strings.HasPrefix(args[i], "--to=") {
			targetPeer = strings.TrimPrefix(args[i], "--to=")
		} else if text == "" && !strings.HasPrefix(args[i], "-") {
			text = args[i]
		}
	}

	if text == "" || targetPeer == "" {
		fmt.Println("Usage: fling msg \"<text>\" --to <peer>")
		os.Exit(1)
	}

	targetAddr, err := resolvePeerAddr(targetPeer)
	if err != nil {
		fmt.Printf("[ERROR] Failed to resolve peer %q: %v\n", targetPeer, err)
		os.Exit(1)
	}

	conn, err := network.ListenUDP(0)
	if err != nil {
		fmt.Printf("[ERROR] Failed to create socket: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "fling-client"
	}

	err = messaging.SendMessage(conn, targetAddr, hostname, text, 1)
	if err != nil {
		fmt.Printf("[ERROR] Failed to send message to %s: %v\n", targetAddr, err)
		os.Exit(1)
	}

	successStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#00E676"))
	fmt.Printf("%s Message delivered to %s\n", successStyle.Render("[SUCCESS]"), targetAddr)
}

func runPeers(args []string) {
	port := discovery.DiscoveryPort
	p := parsePortFlag(args)
	if p > 0 {
		port = p
	}

	conn, err := network.ListenUDP(0)
	if err != nil {
		fmt.Printf("[ERROR] Failed to create socket: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	hostname, _ := os.Hostname()
	disco := discovery.NewDiscovery(hostname, "probe-session", port)

	localIPs := discovery.GetLocalIPs()
	fmt.Printf("Scanning for active Fling peers on local network (Local IPs: %s)...\n", strings.Join(localIPs, ", "))
	_ = disco.BroadcastPresence(conn, discovery.DiscoveryPort)

	buffer := make([]byte, 2048)
	deadline := time.Now().Add(1500 * time.Millisecond)

	for time.Now().Before(deadline) {
		_ = conn.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
		n, senderAddr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			continue
		}

		packet, err := protocol.Decode(buffer[:n])
		if err != nil || packet.Type != protocol.Presence {
			continue
		}

		disco.HandlePresence(packet, senderAddr.IP.String())
	}

	peers := disco.GetPeers()
	cli.PrintPeers(peers)
}

func resolvePeerAddr(target string) (*net.UDPAddr, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return nil, fmt.Errorf("empty peer target")
	}

	// 1. Direct IP:Port (e.g. 127.0.0.1:9999 or 192.168.1.5:9998)
	if strings.Contains(target, ":") {
		return net.ResolveUDPAddr("udp4", target)
	}

	// 2. Direct IP without port (e.g. 127.0.0.1 or 192.168.1.5) -> default to 9999
	if ip := net.ParseIP(target); ip != nil {
		return &net.UDPAddr{
			IP:   ip,
			Port: discovery.DiscoveryPort,
		}, nil
	}

	// 3. Match against currently discovered peers (by peer number, hostname, or session ID)
	conn, err := network.ListenUDP(0)
	if err == nil {
		defer conn.Close()
		hostname, _ := os.Hostname()
		disco := discovery.NewDiscovery(hostname, "probe", discovery.DiscoveryPort)
		_ = disco.BroadcastPresence(conn, discovery.DiscoveryPort)

		buffer := make([]byte, 2048)
		deadline := time.Now().Add(1200 * time.Millisecond)
		for time.Now().Before(deadline) {
			_ = conn.SetReadDeadline(time.Now().Add(250 * time.Millisecond))
			n, senderAddr, err := conn.ReadFromUDP(buffer)
			if err != nil {
				continue
			}
			packet, err := protocol.Decode(buffer[:n])
			if err != nil || packet.Type != protocol.Presence {
				continue
			}
			disco.HandlePresence(packet, senderAddr.IP.String())
		}

		if peer, ok := disco.FindPeer(target); ok {
			return net.ResolveUDPAddr("udp4", peer.Addr())
		}

		// If peers are found but target didn't match, print available peers
		if peers := disco.GetPeers(); len(peers) > 0 {
			fmt.Printf("\n[INFO] Peer %q not found. Active peers on network:\n", target)
			cli.PrintPeers(peers)
		}
	}

	// Fallback: try resolving target as a hostname with default port
	return net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", target, discovery.DiscoveryPort))
}

func runUninstall() {
	execPath, err := os.Executable()
	if err != nil {
		fmt.Printf("[ERROR] Could not determine executable location: %v\n", err)
		os.Exit(1)
	}

	resolvedPath, err := filepath.EvalSymlinks(execPath)
	if err == nil {
		execPath = resolvedPath
	}

	fmt.Printf("==> Removing Fling binary from %s...\n", execPath)
	err = os.Remove(execPath)
	if err != nil {
		fmt.Printf("[ERROR] Failed to remove %s: %v\n", execPath, err)
		fmt.Println("Tip: Try running with sudo: sudo rm -f", execPath)
		os.Exit(1)
	}

	successStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#00E676"))
	fmt.Printf("%s Successfully uninstalled Fling from %s\n", successStyle.Render("[SUCCESS]"), execPath)
}

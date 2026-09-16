package main

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/SatyamKumarCS/Fling-CLI/internal/cli"
	"github.com/SatyamKumarCS/Fling-CLI/internal/discovery"
	"github.com/SatyamKumarCS/Fling-CLI/internal/handshake"
	"github.com/SatyamKumarCS/Fling-CLI/internal/messaging"
	"github.com/SatyamKumarCS/Fling-CLI/internal/network"
	"github.com/SatyamKumarCS/Fling-CLI/internal/protocol"
	"github.com/SatyamKumarCS/Fling-CLI/internal/transfer"
)

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		runListener(discovery.DiscoveryPort)
		return
	}

	switch args[0] {
	case "help", "--help", "-h":
		cli.PrintBanner()
		cli.PrintHelp()
	case "version", "--version", "-v":
		fmt.Printf("Fling CLI v%s\n", cli.Version)
	case "send":
		runSend(args[1:])
	case "msg", "message":
		runMsg(args[1:])
	case "peers", "list":
		runPeers(args[1:])
	default:
		// Check for --port or -p flag when running as listener
		port := parsePortFlag(args)
		if port > 0 {
			runListener(port)
			return
		}

		fmt.Printf("Unknown command or invalid arguments: %s\n\n", strings.Join(args, " "))
		cli.PrintHelp()
		os.Exit(1)
	}
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

func runListener(port int) {
	cli.PrintBanner()

	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "fling-node"
	}
	sessionID := generateSessionID()

	conn, err := network.ListenUDP(port)
	if err != nil {
		fmt.Printf("[ERROR] Failed to bind to UDP port %d: %v\n", port, err)
		// Try fallback port if 9999 is taken
		if port == discovery.DiscoveryPort {
			fallbackPort := 9998
			fmt.Printf("[INFO] Attempting fallback port %d...\n", fallbackPort)
			conn, err = network.ListenUDP(fallbackPort)
			if err != nil {
				fmt.Printf("[ERROR] Fallback port also failed: %v\n", err)
				os.Exit(1)
			}
			port = fallbackPort
		} else {
			os.Exit(1)
		}
	}
	defer conn.Close()

	disco := discovery.NewDiscovery(hostname, sessionID, port)

	// Display node information and local network IP addresses
	localIPs := discovery.GetLocalIPs()
	primaryIP := "127.0.0.1"
	if len(localIPs) > 0 {
		primaryIP = localIPs[0]
	}

	fmt.Printf(" [NODE] Hostname:       %s\n", hostname)
	fmt.Printf(" [NODE] Session ID:     %s\n", sessionID)
	fmt.Printf(" [NODE] Primary LAN IP: %s:%d\n", primaryIP, port)
	fmt.Printf(" [NODE] Listening on:   0.0.0.0:%d\n", port)
	fmt.Println(" ==================================================")
	fmt.Println(" [*] Available Network Addresses on your device:")
	for _, ip := range localIPs {
		fmt.Printf("     • %s:%d (Wi-Fi / LAN)\n", ip, port)
	}
	fmt.Printf("     • 127.0.0.1:%d (Localhost)\n", port)
	fmt.Println()
	fmt.Println(" [*] Share with friends on the same Wi-Fi / network:")
	fmt.Printf("     fling send <file> --to %s:%d\n", primaryIP, port)
	fmt.Printf("     fling msg \"<text>\" --to %s:%d\n", primaryIP, port)
	fmt.Println(" ==================================================")
	fmt.Println(" [*] Auto-discovery active. Searching for peers on LAN & Localhost...")
	fmt.Println(" [*] You can type directly in this terminal:")
	fmt.Println("     • msg 1 Hello there!          (Send message to Peer #1)")
	fmt.Println("     • send 1 tests/test.txt       (Send file to Peer #1)")
	fmt.Println("     • peers                       (View active discovered peers)")
	fmt.Println(" [*] Press Ctrl+C to exit.")

	// Initial peers table
	cli.PrintPeers(disco.GetPeers())

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\n\nShutting down Fling node...")
		conn.Close()
		os.Exit(0)
	}()

	// Background presence broadcaster and stale peer pruner
	stopBroadcast := make(chan struct{})
	defer close(stopBroadcast)
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		// Initial broadcast
		_ = disco.BroadcastPresence(conn, discovery.DiscoveryPort)

		for {
			select {
			case <-stopBroadcast:
				return
			case <-ticker.C:
				// Broadcast presence to all LAN interfaces and local ports
				_ = disco.BroadcastPresence(conn, discovery.DiscoveryPort)

				// Prune stale peers (inactive > 10s)
				removed := disco.PruneStalePeers()
				if len(removed) > 0 {
					for _, rp := range removed {
						fmt.Printf("\n[-] Peer timed out (10s inactive): %s (%s) [Session: %s]\n", rp.Hostname, rp.Addr(), rp.SessionID)
					}
					cli.PrintPeers(disco.GetPeers())
				}
			}
		}
	}()

	// Interactive command reader on stdin
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			rawLine := strings.TrimSpace(scanner.Text())
			if rawLine == "" {
				cli.PrintPeers(disco.GetPeers())
				continue
			}

			// Clean up leading "./fling " or "fling " prefixes if user pasted full command
			line := rawLine
			if strings.HasPrefix(line, "./fling ") {
				line = strings.TrimSpace(strings.TrimPrefix(line, "./fling "))
			} else if strings.HasPrefix(line, "fling ") {
				line = strings.TrimSpace(strings.TrimPrefix(line, "fling "))
			}

			args := splitCommandLine(line)
			if len(args) == 0 {
				cli.PrintPeers(disco.GetPeers())
				continue
			}

			cmd := strings.ToLower(args[0])
			switch cmd {
			case "peers", "list", "ls":
				cli.PrintPeers(disco.GetPeers())

			case "help", "?":
				fmt.Println("\nInteractive Commands:")
				fmt.Println("  peers / list                      - Show active discovered peers")
				fmt.Println("  msg <peer> <text>                 - Send message (e.g. msg 1 Hello!)")
				fmt.Println("  msg \"<text>\" --to <peer>          - Send message using flags")
				fmt.Println("  send <peer> <file>                - Send file (e.g. send 1 tests/test.txt)")
				fmt.Println("  send <file> --to <peer>           - Send file using flags")
				fmt.Println("  clear                             - Clear screen and show node info")
				fmt.Println("  exit / quit                       - Stop Fling node")
				fmt.Println()

			case "msg", "message":
				target, text, err := parseInteractiveMsg(line)
				if err != nil {
					fmt.Printf("[ERROR] %v\n", err)
					continue
				}
				targetAddr, err := resolvePeerAddrWithDisco(target, disco)
				if err != nil {
					fmt.Printf("[ERROR] %v\n", err)
					continue
				}
				err = messaging.SendMessage(conn, targetAddr, hostname, text, uint32(time.Now().UnixNano()&0xFFFF))
				if err != nil {
					fmt.Printf("[ERROR] %v\n", err)
				} else {
					fmt.Printf("[SUCCESS] Message delivered to %s\n", targetAddr)
				}

			case "send":
				target, filePath, err := parseInteractiveSend(line)
				if err != nil {
					fmt.Printf("[ERROR] %v\n", err)
					continue
				}
				targetAddr, err := resolvePeerAddrWithDisco(target, disco)
				if err != nil {
					fmt.Printf("[ERROR] %v\n", err)
					continue
				}
				fileInfo, err := os.Stat(filePath)
				if err != nil {
					fmt.Printf("[ERROR] File not found: %s\n", filePath)
					continue
				}
				if fileInfo.IsDir() {
					fmt.Printf("[ERROR] Directories not supported\n")
					continue
				}
				checksum, size, err := transfer.CalculateFileChecksum(filePath)
				if err != nil {
					fmt.Printf("[ERROR] Checksum calculation failed: %v\n", err)
					continue
				}

				fmt.Printf("[HANDSHAKE] Requesting transfer of %s (%s) to %s...\n", filepath.Base(filePath), cli.FormatBytes(size), targetAddr)
				reqPacket := handshake.CreateTransferRequest(filepath.Base(filePath), size, checksum, uint32(time.Now().UnixNano()&0xFFFF))
				err = network.SendReliable(conn, reqPacket, targetAddr)
				if err != nil {
					fmt.Printf("[ERROR] Transfer request failed: %v\n", err)
					continue
				}
				fmt.Printf("[HANDSHAKE] Transfer request sent. Waiting for peer accept...\n")

			case "clear":
				cli.PrintBanner()
				fmt.Printf(" [NODE] Hostname:       %s\n", hostname)
				fmt.Printf(" [NODE] Session ID:     %s\n", sessionID)
				fmt.Printf(" [NODE] Primary LAN IP: %s:%d\n", primaryIP, port)
				fmt.Printf(" [NODE] Listening on:   0.0.0.0:%d\n", port)
				cli.PrintPeers(disco.GetPeers())

			case "exit", "quit", "q":
				fmt.Println("\nShutting down Fling node...")
				conn.Close()
				os.Exit(0)

			default:
				fmt.Printf("Unknown command %q. Type 'help' or 'peers'.\n", cmd)
			}
		}
	}()

	// Main packet receive loop
	buffer := make([]byte, 65535)
	for {
		_ = conn.SetReadDeadline(time.Time{}) // Clear deadline for continuous listening
		n, senderAddr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			// Check if connection was closed
			select {
			case <-sigCh:
				return
			default:
				continue
			}
		}

		packet, err := protocol.Decode(buffer[:n])
		if err != nil {
			// Corrupt or invalid packet, ignore
			continue
		}

		switch packet.Type {
		case protocol.Presence:
			peer, isNew := disco.HandlePresence(packet, senderAddr.IP.String())
			if isNew {
				fmt.Printf("\n[+] Discovered new peer: %s (%s) [Session: %s]\n", peer.Hostname, peer.Addr(), peer.SessionID)
				cli.PrintPeers(disco.GetPeers())
			}

		case protocol.Msg:
			// Immediately ACK message packet
			_ = network.SendACK(conn, packet.SequenceNumber, senderAddr)

			msg, err := messaging.ParseMessage(packet)
			if err != nil {
				fmt.Printf("[ERROR] Failed to parse message from %s: %v\n", senderAddr, err)
				continue
			}
			messaging.DisplayMessage(msg)

		case protocol.TransferRequest:
			// Immediately ACK transfer request packet
			_ = network.SendACK(conn, packet.SequenceNumber, senderAddr)

			req, err := handshake.ParseTransferRequest(packet)
			if err != nil {
				fmt.Printf("[ERROR] Invalid transfer request from %s: %v\n", senderAddr, err)
				continue
			}

			accepted, promptErr := cli.PromptAcceptReject(senderAddr.String(), req.Filename, req.FileSize)
			if promptErr != nil || !accepted {
				fmt.Printf("[TRANSFER] Rejected file transfer %q from %s\n\n", req.Filename, senderAddr)
				rejectPacket := handshake.CreateTransferReject(packet.SequenceNumber + 1)
				if encodedReject, err := protocol.Encode(rejectPacket); err == nil {
					_, _ = conn.WriteToUDP(encodedReject, senderAddr)
				}
				continue
			}

			// User accepted transfer
			acceptPacket := handshake.CreateTransferAccept(packet.SequenceNumber + 1)
			encodedAccept, err := protocol.Encode(acceptPacket)
			if err != nil {
				fmt.Printf("[ERROR] Failed to encode transfer accept packet: %v\n", err)
				continue
			}
			_, _ = conn.WriteToUDP(encodedAccept, senderAddr)

			destPath := getDestinationPath(req.Filename)
			fmt.Printf("[TRANSFER] Receiving %s (%s)...\n", req.Filename, cli.FormatBytes(req.FileSize))
			pb := cli.NewProgressBar(req.FileSize, req.Filename)

			recvErr := transfer.ReceiveFile(
				conn,
				packet.SequenceNumber+1,
				req.FileSize,
				req.Checksum,
				destPath,
				func(transferred, total int64) {
					pb.Update(transferred)
				},
			)

			if recvErr != nil {
				fmt.Printf("\n[TRANSFER ERROR] Failed to receive file: %v\n\n", recvErr)
			} else {
				pb.Finish()
				fmt.Printf("[SUCCESS] File saved: %s (Checksum: 0x%08x)\n\n", destPath, req.Checksum)
			}

		case protocol.ACK, protocol.TransferAccept, protocol.TransferReject, protocol.FileChunk, protocol.FileEnd:
			// Unexpected standalone packet in general listener loop, ignore
			continue
		}
	}
}

func parseInteractiveMsg(input string) (string, string, error) {
	args := splitCommandLine(input)
	if len(args) == 0 {
		return "", "", fmt.Errorf("empty message command")
	}
	// Strip "msg" or "message"
	args = args[1:]

	var target string
	var textParts []string

	for i := 0; i < len(args); i++ {
		if (args[i] == "--to" || args[i] == "-t") && i+1 < len(args) {
			target = args[i+1]
			i++
		} else if strings.HasPrefix(args[i], "--to=") {
			target = strings.TrimPrefix(args[i], "--to=")
		} else {
			textParts = append(textParts, args[i])
		}
	}

	if target == "" && len(textParts) >= 2 {
		target = textParts[0]
		textParts = textParts[1:]
	}

	text := strings.Join(textParts, " ")
	if target == "" || text == "" {
		return "", "", fmt.Errorf("usage: msg <peer> <text> or msg \"<text>\" --to <peer>")
	}
	return target, text, nil
}

func parseInteractiveSend(input string) (string, string, error) {
	args := splitCommandLine(input)
	if len(args) == 0 {
		return "", "", fmt.Errorf("empty send command")
	}
	args = args[1:]

	var target string
	var fileParts []string

	for i := 0; i < len(args); i++ {
		if (args[i] == "--to" || args[i] == "-t") && i+1 < len(args) {
			target = args[i+1]
			i++
		} else if strings.HasPrefix(args[i], "--to=") {
			target = strings.TrimPrefix(args[i], "--to=")
		} else {
			fileParts = append(fileParts, args[i])
		}
	}

	var filePath string
	if len(fileParts) > 0 {
		filePath = fileParts[0]
	}

	if target == "" && len(fileParts) >= 2 {
		if _, err := os.Stat(fileParts[0]); err == nil {
			filePath = fileParts[0]
			target = fileParts[1]
		} else {
			target = fileParts[0]
			filePath = fileParts[1]
		}
	}

	if target == "" || filePath == "" {
		return "", "", fmt.Errorf("usage: send <peer> <file> or send <file> --to <peer>")
	}
	return target, filePath, nil
}

func splitCommandLine(line string) []string {
	var args []string
	var current strings.Builder
	inQuote := false
	quoteChar := rune(0)

	for _, r := range line {
		if inQuote {
			if r == quoteChar {
				inQuote = false
			} else {
				current.WriteRune(r)
			}
		} else {
			if r == '"' || r == '\'' {
				inQuote = true
				quoteChar = r
			} else if r == ' ' || r == '\t' {
				if current.Len() > 0 {
					args = append(args, current.String())
					current.Reset()
				}
			} else {
				current.WriteRune(r)
			}
		}
	}
	if current.Len() > 0 {
		args = append(args, current.String())
	}
	return args
}

func getDestinationPath(filename string) string {
	cleanName := filepath.Base(filename)
	if cleanName == "" || cleanName == "." {
		cleanName = "downloaded_file.bin"
	}

	dest := cleanName
	if _, err := os.Stat(dest); err == nil {
		// File exists, prepend received_
		dest = "received_" + cleanName
	}
	return dest
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

	fileInfo, err := os.Stat(filePath)
	if err != nil {
		fmt.Printf("[ERROR] File not found: %s\n", filePath)
		os.Exit(1)
	}
	if fileInfo.IsDir() {
		fmt.Printf("[ERROR] %s is a directory; only single files are supported\n", filePath)
		os.Exit(1)
	}

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

	fmt.Printf("[HANDSHAKE] Requesting transfer of %s (%s) to %s...\n", filepath.Base(filePath), cli.FormatBytes(size), targetAddr)

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
	fmt.Printf("[SUCCESS] Transfer completed successfully! (Checksum: 0x%08x)\n", checksum)
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

	fmt.Printf("[SUCCESS] Message delivered to %s\n", targetAddr)
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

func resolvePeerAddrWithDisco(target string, disco *discovery.Discovery) (*net.UDPAddr, error) {
	if peer, ok := disco.FindPeer(target); ok {
		return net.ResolveUDPAddr("udp4", peer.Addr())
	}
	return resolvePeerAddr(target)
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

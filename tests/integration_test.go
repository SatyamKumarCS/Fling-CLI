package tests

import (
	"bytes"
	"crypto/rand"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/SatyamKumarCS/Fling-CLI/internal/discovery"
	"github.com/SatyamKumarCS/Fling-CLI/internal/handshake"
	"github.com/SatyamKumarCS/Fling-CLI/internal/messaging"
	"github.com/SatyamKumarCS/Fling-CLI/internal/network"
	"github.com/SatyamKumarCS/Fling-CLI/internal/protocol"
	"github.com/SatyamKumarCS/Fling-CLI/internal/transfer"
)

func TestIntegrationPeerDiscovery(t *testing.T) {
	nodeAConn, err := network.ListenUDP(0)
	if err != nil {
		t.Fatalf("failed to create nodeA socket: %v", err)
	}
	defer nodeAConn.Close()
	nodeAPort := nodeAConn.LocalAddr().(*net.UDPAddr).Port

	nodeBConn, err := network.ListenUDP(0)
	if err != nil {
		t.Fatalf("failed to create nodeB socket: %v", err)
	}
	defer nodeBConn.Close()
	nodeBPort := nodeBConn.LocalAddr().(*net.UDPAddr).Port

	discoA := discovery.NewDiscovery("node-A", "sess-A", nodeAPort)
	discoB := discovery.NewDiscovery("node-B", "sess-B", nodeBPort)

	// Node B sends presence packet directly to Node A
	packetB := discovery.CreatePresencePacket("node-B", "sess-B", nodeBPort, 1)
	encodedB, err := protocol.Encode(packetB)
	if err != nil {
		t.Fatalf("failed to encode presence packet: %v", err)
	}

	_, err = nodeBConn.WriteToUDP(encodedB, &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: nodeAPort})
	if err != nil {
		t.Fatalf("failed to send presence packet: %v", err)
	}

	// Node A receives and handles presence
	buf := make([]byte, 2048)
	_ = nodeAConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, senderAddr, err := nodeAConn.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("nodeA failed to read packet: %v", err)
	}

	decoded, err := protocol.Decode(buf[:n])
	if err != nil {
		t.Fatalf("failed to decode packet: %v", err)
	}

	peer, ok := discoA.HandlePresence(decoded, senderAddr.IP.String())
	if !ok {
		t.Fatal("expected peer presence to be accepted")
	}

	if peer.Hostname != "node-B" {
		t.Errorf("expected hostname 'node-B', got %q", peer.Hostname)
	}

	if peer.Port != nodeBPort {
		t.Errorf("expected port %d, got %d", nodeBPort, peer.Port)
	}

	// Node A searches for Node B
	found, exists := discoA.FindPeer("node-B")
	if !exists || found.SessionID != "sess-B" {
		t.Errorf("FindPeer failed to find node-B: exists=%v, found=%+v", exists, found)
	}
	_ = discoB
}

func TestIntegrationMessagingDirect(t *testing.T) {
	receiverConn, err := network.ListenUDP(0)
	if err != nil {
		t.Fatalf("failed to create receiver socket: %v", err)
	}
	defer receiverConn.Close()
	receiverAddr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: receiverConn.LocalAddr().(*net.UDPAddr).Port}

	senderConn, err := network.ListenUDP(0)
	if err != nil {
		t.Fatalf("failed to create sender socket: %v", err)
	}
	defer senderConn.Close()

	var receivedMsg messaging.Message
	var recvErr error
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		receivedMsg, _, recvErr = messaging.ReceiveMessage(receiverConn, 3*time.Second)
	}()

	msgText := "Integration test text message: Fling is fast!"
	err = messaging.SendMessage(senderConn, receiverAddr, "alice", msgText, 1)
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	wg.Wait()

	if recvErr != nil {
		t.Fatalf("ReceiveMessage error: %v", recvErr)
	}

	if receivedMsg.Sender != "alice" || receivedMsg.Content != msgText {
		t.Errorf("unexpected received message: %+v", receivedMsg)
	}

	formatted := messaging.FormatMessage(receivedMsg)
	if !strings.Contains(formatted, "alice") || !strings.Contains(formatted, msgText) {
		t.Errorf("unexpected formatted output: %s", formatted)
	}
}

func TestIntegrationFileTransferAcceptAndVerify(t *testing.T) {
	tempDir := t.TempDir()
	srcFile := filepath.Join(tempDir, "test_payload.bin")
	dstFile := filepath.Join(tempDir, "received_payload.bin")

	// Generate 5000 bytes of arbitrary test data
	testData := make([]byte, 5000)
	_, _ = rand.Read(testData)
	if err := os.WriteFile(srcFile, testData, 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	checksum, size, err := transfer.CalculateFileChecksum(srcFile)
	if err != nil {
		t.Fatalf("CalculateFileChecksum failed: %v", err)
	}

	receiverConn, err := network.ListenUDP(0)
	if err != nil {
		t.Fatalf("failed to create receiver socket: %v", err)
	}
	defer receiverConn.Close()
	receiverAddr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: receiverConn.LocalAddr().(*net.UDPAddr).Port}

	senderConn, err := network.ListenUDP(0)
	if err != nil {
		t.Fatalf("failed to create sender socket: %v", err)
	}
	defer senderConn.Close()

	var wg sync.WaitGroup
	var receiverErr error

	// Receiver routine
	wg.Add(1)
	go func() {
		defer wg.Done()
		buf := make([]byte, 2048)
		_ = receiverConn.SetReadDeadline(time.Now().Add(5 * time.Second))
		n, senderAddr, err := receiverConn.ReadFromUDP(buf)
		if err != nil {
			receiverErr = err
			return
		}

		packet, err := protocol.Decode(buf[:n])
		if err != nil {
			receiverErr = err
			return
		}

		// ACK the request
		_ = network.SendACK(receiverConn, packet.SequenceNumber, senderAddr)

		req, err := handshake.ParseTransferRequest(packet)
		if err != nil {
			receiverErr = err
			return
		}

		// Send Accept packet
		acceptPacket := handshake.CreateTransferAccept(packet.SequenceNumber + 1)
		encodedAccept, err := protocol.Encode(acceptPacket)
		if err != nil {
			receiverErr = err
			return
		}
		_, err = receiverConn.WriteToUDP(encodedAccept, senderAddr)
		if err != nil {
			receiverErr = err
			return
		}

		// Receive file chunks
		receiverErr = transfer.ReceiveFile(
			receiverConn,
			packet.SequenceNumber+1,
			req.FileSize,
			req.Checksum,
			dstFile,
			nil,
		)
	}()

	// Sender routine: initiate handshake
	reqPacket := handshake.CreateTransferRequest(filepath.Base(srcFile), size, checksum, 1)
	err = network.SendReliable(senderConn, reqPacket, receiverAddr)
	if err != nil {
		t.Fatalf("sender failed to send TransferRequest: %v", err)
	}

	// Wait for Accept
	buf := make([]byte, 2048)
	_ = senderConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, _, err := senderConn.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("sender failed to receive Accept: %v", err)
	}

	acceptPacket, err := protocol.Decode(buf[:n])
	if err != nil || acceptPacket.Type != protocol.TransferAccept {
		t.Fatalf("expected TransferAccept packet, got type %d, err %v", acceptPacket.Type, err)
	}

	// Send file chunks
	nextSeq, err := transfer.SendFile(senderConn, receiverAddr, srcFile, 2, nil)
	if err != nil {
		t.Fatalf("SendFile failed: %v", err)
	}
	if nextSeq != 7 { // 5 chunks (seq 2..6) + 1 FileEnd (seq 7) -> returns 8 or 7
		t.Logf("next sequence number: %d", nextSeq)
	}

	wg.Wait()

	if receiverErr != nil {
		t.Fatalf("receiver failed: %v", receiverErr)
	}

	// Verify destination file content and checksum
	receivedBytes, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("failed to read destination file: %v", err)
	}

	if !bytes.Equal(receivedBytes, testData) {
		t.Fatalf("received bytes do not match original test data")
	}

	dstChecksum, dstSize, err := transfer.CalculateFileChecksum(dstFile)
	if err != nil {
		t.Fatalf("CalculateFileChecksum on dstFile failed: %v", err)
	}

	if dstChecksum != checksum {
		t.Errorf("checksum mismatch: expected 0x%08x, got 0x%08x", checksum, dstChecksum)
	}
	if dstSize != size {
		t.Errorf("size mismatch: expected %d, got %d", size, dstSize)
	}
}

func TestIntegrationFileTransferRejection(t *testing.T) {
	tempDir := t.TempDir()
	srcFile := filepath.Join(tempDir, "rejected_file.bin")
	if err := os.WriteFile(srcFile, []byte("sensitive content"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	checksum, size, _ := transfer.CalculateFileChecksum(srcFile)

	receiverConn, err := network.ListenUDP(0)
	if err != nil {
		t.Fatalf("failed to create receiver: %v", err)
	}
	defer receiverConn.Close()
	receiverAddr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: receiverConn.LocalAddr().(*net.UDPAddr).Port}

	senderConn, err := network.ListenUDP(0)
	if err != nil {
		t.Fatalf("failed to create sender: %v", err)
	}
	defer senderConn.Close()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		buf := make([]byte, 2048)
		_ = receiverConn.SetReadDeadline(time.Now().Add(5 * time.Second))
		n, senderAddr, err := receiverConn.ReadFromUDP(buf)
		if err != nil {
			return
		}

		packet, err := protocol.Decode(buf[:n])
		if err != nil {
			return
		}

		// ACK request
		_ = network.SendACK(receiverConn, packet.SequenceNumber, senderAddr)

		// Send Reject packet
		rejectPacket := handshake.CreateTransferReject(packet.SequenceNumber + 1)
		_ = network.SendReliable(receiverConn, rejectPacket, senderAddr)
	}()

	// Sender sends request
	reqPacket := handshake.CreateTransferRequest(filepath.Base(srcFile), size, checksum, 1)
	err = network.SendReliable(senderConn, reqPacket, receiverAddr)
	if err != nil {
		t.Fatalf("SendReliable failed: %v", err)
	}

	// Wait for response
	buf := make([]byte, 2048)
	_ = senderConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, _, err := senderConn.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("sender failed to read response: %v", err)
	}

	respPacket, err := protocol.Decode(buf[:n])
	if err != nil {
		t.Fatalf("failed to decode response packet: %v", err)
	}

	if respPacket.Type != protocol.TransferReject {
		t.Errorf("expected TransferReject (%d), got %d", protocol.TransferReject, respPacket.Type)
	}

	wg.Wait()
}

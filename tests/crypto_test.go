package tests

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/SatyamKumarCS/Fling-CLI/internal/discovery"
	"github.com/SatyamKumarCS/Fling-CLI/internal/messaging"
	"github.com/SatyamKumarCS/Fling-CLI/internal/security"
	"github.com/SatyamKumarCS/Fling-CLI/internal/transfer"
)

func TestKeyPairGenerationAndPublicKeyLength(t *testing.T) {
	kp, err := security.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}
	if kp.PrivateKey == nil || kp.PublicKey == nil {
		t.Fatalf("KeyPair keys must not be nil")
	}
	pubBytes := kp.PublicKeyBytes()
	if len(pubBytes) != 32 {
		t.Fatalf("Expected 32-byte public key, got %d bytes", len(pubBytes))
	}
}

func TestECDHSharedKeyDerivation(t *testing.T) {
	alice, err := security.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Alice GenerateKeyPair failed: %v", err)
	}

	bob, err := security.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Bob GenerateKeyPair failed: %v", err)
	}

	// Alice computes shared key using Bob's public key
	aliceShared, err := security.DeriveSharedKey(alice.PrivateKey, bob.PublicKeyBytes())
	if err != nil {
		t.Fatalf("Alice DeriveSharedKey failed: %v", err)
	}
	if len(aliceShared) != 32 {
		t.Fatalf("Expected 32-byte shared key, got %d bytes", len(aliceShared))
	}

	// Bob computes shared key using Alice's public key
	bobShared, err := security.DeriveSharedKey(bob.PrivateKey, alice.PublicKeyBytes())
	if err != nil {
		t.Fatalf("Bob DeriveSharedKey failed: %v", err)
	}
	if len(bobShared) != 32 {
		t.Fatalf("Expected 32-byte shared key, got %d bytes", len(bobShared))
	}

	// The two independently derived keys must be strictly identical
	if !bytes.Equal(aliceShared, bobShared) {
		t.Fatalf("Shared keys do not match! Alice: %x, Bob: %x", aliceShared, bobShared)
	}
}

func TestAES256GCMEncryptDecryptRoundtrip(t *testing.T) {
	alice, _ := security.GenerateKeyPair()
	bob, _ := security.GenerateKeyPair()
	key, _ := security.DeriveSharedKey(alice.PrivateKey, bob.PublicKeyBytes())

	originalPlaintext := []byte("Sensitive local payload: Hello peer over untrusted Wi-Fi!")

	ciphertext, err := security.Encrypt(key, originalPlaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// Ensure ciphertext is not identical to plaintext
	if bytes.Equal(ciphertext, originalPlaintext) {
		t.Fatalf("Ciphertext matches plaintext! Encryption did not occur.")
	}

	// Decrypt
	decrypted, err := security.Decrypt(key, ciphertext)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if !bytes.Equal(decrypted, originalPlaintext) {
		t.Fatalf("Decrypted text %q does not match original %q", string(decrypted), string(originalPlaintext))
	}
}

func TestAES256GCMTamperResistance(t *testing.T) {
	alice, _ := security.GenerateKeyPair()
	bob, _ := security.GenerateKeyPair()
	key, _ := security.DeriveSharedKey(alice.PrivateKey, bob.PublicKeyBytes())

	plaintext := []byte("Financial or authentication datagram")
	ciphertext, err := security.Encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// Corrupt one byte of ciphertext (simulate on-wire tampering)
	corrupted := make([]byte, len(ciphertext))
	copy(corrupted, ciphertext)
	corrupted[len(corrupted)-1] ^= 0xFF

	_, err = security.Decrypt(key, corrupted)
	if err == nil {
		t.Fatalf("Decryption succeeded on tampered ciphertext! GCM tag check failed to trigger.")
	}
}

func TestEncryptedMessagingRoundtrip(t *testing.T) {
	aliceKP, _ := security.GenerateKeyPair()
	bobKP, _ := security.GenerateKeyPair()
	sharedKey, _ := security.DeriveSharedKey(aliceKP.PrivateKey, bobKP.PublicKeyBytes())

	secretText := "Top Secret: Rendezvous at 18:00"
	packet, err := messaging.CreateEncryptedMessage("Alice", secretText, sharedKey, 42)
	if err != nil {
		t.Fatalf("CreateEncryptedMessage failed: %v", err)
	}

	// Verify on-wire packet payload is encrypted (does NOT contain the secret in plaintext)
	if bytes.Contains(packet.Payload, []byte("Top Secret")) {
		t.Fatalf("Wire packet contains plaintext secret! Encryption was skipped.")
	}

	// Bob parses with his shared key
	msg, err := messaging.ParseMessageWithKey(packet, sharedKey)
	if err != nil {
		t.Fatalf("ParseMessageWithKey failed: %v", err)
	}

	if msg.Sender != "Alice" {
		t.Errorf("Expected sender Alice, got %s", msg.Sender)
	}
	if msg.Content != secretText {
		t.Errorf("Expected content %q, got %q", secretText, msg.Content)
	}
	if !msg.Encrypted {
		t.Errorf("Expected msg.Encrypted to be true")
	}
}

func TestEncryptedFileTransferReassembly(t *testing.T) {
	aliceKP, _ := security.GenerateKeyPair()
	bobKP, _ := security.GenerateKeyPair()
	sharedKey, _ := security.DeriveSharedKey(aliceKP.PrivateKey, bobKP.PublicKeyBytes())

	fileContent := []byte("Large secret report payload containing confidential data chunks...")
	checksum := transfer.CalculateBytesChecksum(fileContent)

	// Encrypt the full file payload
	encryptedData, err := security.Encrypt(sharedKey, fileContent)
	if err != nil {
		t.Fatalf("Failed to encrypt file data: %v", err)
	}

	// Chunk the encrypted bytes
	chunks := transfer.ChunkBytes(encryptedData, 10)
	if len(chunks) == 0 {
		t.Fatalf("Expected at least 1 chunk")
	}

	// Reassemble with key
	reassembler := transfer.NewReassembler(int64(len(fileContent)), checksum)
	for _, chunk := range chunks {
		reassembler.AddChunk(chunk.Payload)
	}

	// Finalize with decryption
	decryptedBytes, err := reassembler.FinalizeWithKey(sharedKey)
	if err != nil {
		t.Fatalf("FinalizeWithKey failed: %v", err)
	}

	if !bytes.Equal(decryptedBytes, fileContent) {
		t.Fatalf("Reassembled file content does not match original!")
	}

	// Test SaveToFileWithKey
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "saved_secret.txt")
	err = reassembler.SaveToFileWithKey(outPath, sharedKey)
	if err != nil {
		t.Fatalf("SaveToFileWithKey failed: %v", err)
	}

	readBack, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if !bytes.Equal(readBack, fileContent) {
		t.Fatalf("Saved file content does not match original!")
	}
}

func TestDiscoveryPresenceKeyExchange(t *testing.T) {
	aliceKP, _ := security.GenerateKeyPair()
	bobKP, _ := security.GenerateKeyPair()

	aliceDisco := discovery.NewDiscovery("alice-node", "alice-sess", 9999, aliceKP)
	bobDisco := discovery.NewDiscovery("bob-node", "bob-sess", 9998, bobKP)

	// Alice broadcasts presence packet containing her public key
	alicePacket := discovery.CreatePresencePacket(aliceDisco.Hostname, aliceDisco.SessionID, aliceDisco.Port, 1, aliceKP.PublicKeyBytes())

	// Bob receives Alice's presence packet
	peer, isNew := bobDisco.HandlePresence(alicePacket, "10.7.5.141")
	if !isNew {
		t.Fatalf("Expected isNew to be true for Alice")
	}
	if len(peer.PublicKey) != 32 {
		t.Fatalf("Expected Bob to extract 32-byte public key for Alice, got %d", len(peer.PublicKey))
	}
	if len(peer.SharedKey) != 32 {
		t.Fatalf("Expected Bob to automatically derive 32-byte shared key for Alice, got %d", len(peer.SharedKey))
	}

	// Bob replies with his presence packet containing his public key
	bobPacket := discovery.CreatePresencePacket(bobDisco.Hostname, bobDisco.SessionID, bobDisco.Port, 1, bobKP.PublicKeyBytes())

	// Alice receives Bob's presence packet
	alicePeer, isNewAlice := aliceDisco.HandlePresence(bobPacket, "10.7.12.154")
	if !isNewAlice {
		t.Fatalf("Expected isNew to be true for Bob")
	}
	if len(alicePeer.SharedKey) != 32 {
		t.Fatalf("Expected Alice to derive 32-byte shared key for Bob, got %d", len(alicePeer.SharedKey))
	}

	// Both derived keys must match identically
	if !bytes.Equal(peer.SharedKey, alicePeer.SharedKey) {
		t.Fatalf("Derived shared keys do not match between Alice and Bob!")
	}

	// Test GetPeerSharedKey and GetPeerSharedKeyByAddr
	key1, ok1 := bobDisco.GetPeerSharedKey("alice-sess")
	if !ok1 || !bytes.Equal(key1, peer.SharedKey) {
		t.Errorf("GetPeerSharedKey failed")
	}

	key2, ok2 := bobDisco.GetPeerSharedKeyByAddr("10.7.5.141:9999")
	if !ok2 || !bytes.Equal(key2, peer.SharedKey) {
		t.Errorf("GetPeerSharedKeyByAddr failed")
	}
}

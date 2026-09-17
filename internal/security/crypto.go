package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
)

var (
	ErrNilKey          = errors.New("encryption key cannot be nil or empty")
	ErrInvalidKeySize  = errors.New("encryption key must be 32 bytes for AES-256")
	ErrCiphertextShort = errors.New("ciphertext too short to contain nonce and tag")
	ErrDecryptionFail  = errors.New("decryption failed: ciphertext corrupted or invalid key")
)

// KeyPair holds an ephemeral X25519 private key and its corresponding public key.
type KeyPair struct {
	PrivateKey *ecdh.PrivateKey
	PublicKey  *ecdh.PublicKey
}

// GenerateKeyPair generates a new cryptographically secure X25519 ephemeral keypair.
func GenerateKeyPair() (*KeyPair, error) {
	priv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate X25519 keypair: %w", err)
	}
	return &KeyPair{
		PrivateKey: priv,
		PublicKey:  priv.PublicKey(),
	}, nil
}

// PublicKeyBytes returns the 32-byte raw representation of the public key.
func (kp *KeyPair) PublicKeyBytes() []byte {
	if kp == nil || kp.PublicKey == nil {
		return nil
	}
	return kp.PublicKey.Bytes()
}

// DeriveSharedKey performs an X25519 ECDH key exchange with a remote peer's public key,
// hashing the resulting shared secret with SHA-256 to produce a 32-byte (256-bit) symmetric key.
func DeriveSharedKey(priv *ecdh.PrivateKey, remotePubBytes []byte) ([]byte, error) {
	if priv == nil {
		return nil, errors.New("private key cannot be nil")
	}
	if len(remotePubBytes) != 32 {
		return nil, fmt.Errorf("invalid public key length: expected 32 bytes, got %d", len(remotePubBytes))
	}

	remotePub, err := ecdh.X25519().NewPublicKey(remotePubBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse remote X25519 public key: %w", err)
	}

	sharedSecret, err := priv.ECDH(remotePub)
	if err != nil {
		return nil, fmt.Errorf("ECDH key agreement failed: %w", err)
	}

	// Derive 256-bit symmetric key via SHA-256
	derivedKey := sha256.Sum256(sharedSecret)
	return derivedKey[:], nil
}

// Encrypt encrypts plaintext using AES-256-GCM authenticated cipher with a 12-byte cryptographically
// random nonce prepended to the resulting ciphertext.
func Encrypt(key []byte, plaintext []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, ErrInvalidKeySize
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM block: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate random nonce: %w", err)
	}

	// Seal appends ciphertext and authentication tag to nonce
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt decrypts and authenticates AES-256-GCM ciphertext using the provided 32-byte key.
// Expects the 12-byte nonce to be prepended to the ciphertext.
func Decrypt(key []byte, ciphertext []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, ErrInvalidKeySize
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM block: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize+gcm.Overhead() {
		return nil, ErrCiphertextShort
	}

	nonce, actualCiphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, actualCiphertext, nil)
	if err != nil {
		return nil, ErrDecryptionFail
	}

	return plaintext, nil
}

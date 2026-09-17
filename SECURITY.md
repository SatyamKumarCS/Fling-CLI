# Security Policy & Cryptographic Architecture

## 1. Supported Versions

Security updates, patches, and vulnerability fixes are provided for the following releases:

| Version | Supported          | Security Status |
| ------- | ------------------ | --------------- |
| 1.0.x   | :white_check_mark: | Active (E2EE)   |
| < 1.0.0 | :x:                | Deprecated      |

---

## 2. End-to-End Encryption (E2EE) Architecture

Fling implements zero-configuration End-to-End Encryption (E2EE) for all local network communication (instant messaging and file transfers). All data transmitted over UDP is encrypted at the application layer before reaching the network interface.

```
+-------------------------------------------------------------------------------+
|                             FLING SECURITY MODEL                             |
+-------------------------------------------------------------------------------+

  [ Node A (Alice) ]                                  [ Node B (Bob) ]
          |                                                   |
          | 1. Generate Ephemeral X25519 Keypair              | 1. Generate Ephemeral X25519 Keypair
          |    (privA, pubA)                                  |    (privB, pubB)
          |                                                   |
          | 2. Broadcast Presence Beacon                      | 2. Broadcast Presence Beacon
          |    "Alice|sessionA|port|pubA"                     |    "Bob|sessionB|port|pubB"
          | ------------------------------------------------> |
          | <------------------------------------------------ |
          |                                                   |
          | 3. ECDH Key Agreement:                            | 3. ECDH Key Agreement:
          |    secret = ECDH(privA, pubB)                     |    secret = ECDH(privB, pubA)
          |    key = SHA-256(secret)                          |    key = SHA-256(secret)
          |    [ Identical 256-bit Key Derived ]              |    [ Identical 256-bit Key Derived ]
          |                                                   |
          | 4. Encrypted Datagram (AES-256-GCM):              |
          |    Nonce (12B) + Ciphertext + Tag (16B)           |
          | ----------------================================> |
          |                                                   | 5. Authenticate & Decrypt:
          |                                                   |    Verify GCM Tag -> Plaintext
```

### 2.1 Cryptographic Primitives

| Component | Standard / Algorithm | Key / Nonce Size | Standard Library Package |
|---|---|---|---|
| **Key Agreement** | X25519 ECDH (RFC 7748 / Curve25519) | 256-bit (32 bytes) | `crypto/ecdh` |
| **Key Derivation** | SHA-256 Hash Function | 256-bit (32 bytes) | `crypto/sha256` |
| **Symmetric Cipher** | AES-256-GCM (Galois/Counter Mode) | 256-bit Key, 96-bit Nonce | `crypto/aes`, `crypto/cipher` |
| **Authentication Tag** | GCM Poly1305 / GHASH Tag | 128-bit (16 bytes) | `crypto/cipher` |
| **Random Nonces** | Cryptographically Secure PRNG | 96-bit (12 bytes) | `crypto/rand` |
| **File Integrity** | IEEE 802.3 CRC32 Checksum | 32-bit (4 bytes) | `hash/crc32` |

### 2.2 Zero External Cryptographic Dependencies
All cryptographic operations in Fling are built exclusively on the **Go standard library** (`crypto/*`). No third-party C bindings, unverified packages, or external assembly dependencies are used, eliminating supply chain attack vectors.

---

## 3. Threat Model & Security Guarantees

### 3.1 What Fling Protects Against

1. **Eavesdropping on Local Wi-Fi / LAN:**
   - Anyone sniffing local network packets (via Wireshark, promiscuous mode, or open Wi-Fi APs) will only observe encrypted binary ciphertext.
   - Zero plaintext chat messages, filenames, or file contents are exposed on the wire.

2. **Tampering & In-Transit Data Modification:**
   - AES-256-GCM provides authenticated encryption. Any modification of ciphertext or nonce in transit causes GCM authentication tag verification to fail.
   - Corrupted or modified datagrams are dropped immediately with zero execution.

3. **Replay Attacks & Out-of-Order Delivery:**
   - Stop-and-Wait ARQ with monotonic sequence numbers, sliding duplicate cache, and router-level deduplication ensures replayed packets cannot trigger duplicate actions or state manipulation.

4. **Silent Data Corruption:**
   - Reassembled files are decrypted and verified against the sender's IEEE CRC32 checksum before being written to disk. If the checksum mismatches, the file is rejected and purged.

5. **Unsolicited Transfers (Consent-First Handshake):**
   - No incoming file transfer can execute without explicit interactive confirmation from the recipient (`[y/n]` prompt). Senders cannot force files onto the recipient's filesystem.

### 3.2 Out of Scope / Environmental Assumptions
- **Compromised Host:** If the local operating system or user account is compromised, the attacker may access local application memory.
- **Physical Device Access:** Local files stored after download are subject to operating system user permissions (`0644` / `0755`).

---

## 4. Security Verification

Fling's cryptographic and protocol implementations are validated via continuous integration and automated test suites with race detection:

```bash
# Run full cryptographic and network security test suite
go test -v -race ./...
```

### Automated Security Tests (`tests/crypto_test.go`):
- `TestKeyPairGenerationAndPublicKeyLength`: Verifies 32-byte X25519 keypair generation.
- `TestECDHSharedKeyDerivation`: Verifies Alice and Bob independently derive identical 256-bit symmetric keys.
- `TestAES256GCMEncryptDecryptRoundtrip`: Verifies plaintext-ciphertext confidentiality.
- `TestAES256GCMTamperResistance`: Verifies single-bit corruption triggers GCM tag verification failure.
- `TestEncryptedMessagingRoundtrip`: Verifies wire packets contain zero plaintext text.
- `TestEncryptedFileTransferReassembly`: Verifies end-to-end file chunk encryption, reassembly, decryption, and CRC32 integrity.
- `TestDiscoveryPresenceKeyExchange`: Verifies automatic presence key distribution and shared key derivation.

---

## 5. Reporting a Vulnerability

We take the security of Fling seriously. If you discover a security vulnerability, please report it privately:

1. **GitHub Security Advisory (Preferred):**
   - Open a private report at [GitHub Security Advisories](https://github.com/SatyamKumarCS/Fling-CLI/security/advisories/new).
2. **Email Disclosure:**
   - Email the maintainer directly with reproduction steps and proof of concept.

### Response Commitments:
- **Initial Response:** Within 48 hours of report submission.
- **Triage & Reproduction:** Within 5 business days.
- **Patch & Advisory Release:** A security patch will be released with full credit attributed to the reporter.

Please do not open public GitHub issues for sensitive security vulnerabilities before a patch has been coordinated.

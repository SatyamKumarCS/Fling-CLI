# Fling Wire Protocol Specification (RFC-Style)

**Status:** Informational / Standard Specification  
**Version:** 1.0.0  
**Author:** Satyam  
**Transport:** Raw UDP / IPv4  

---

## 1. Abstract

Fling is a lightweight, zero-configuration peer-to-peer (P2P) file transfer and direct messaging protocol designed for local area networks (LANs). It operates exclusively over raw UDP datagrams without reliance on centralized servers, cloud relays, or TCP transport, providing built-in reliability (Stop-and-Wait ARQ / Selective Retransmission), packet deduplication, out-of-order reassembly, and end-to-end CRC32 integrity verification.

---

## 2. Packet Wire Format

Every Fling packet consists of a fixed **11-byte header** in Network Byte Order (Big Endian), followed by a variable-length payload.

```text
 0                   1                   2                   3
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                       Sequence Number                         |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|     Type      |        Payload Length         |    CRC32 ...  |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|      ... Checksum             |            Payload ...        |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                          ... Payload                          |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

### 2.1 Header Fields

1. **Sequence Number (4 Bytes, `uint32`, Big-Endian):**
   - Identifies the packet in an ordered stream or exchange.
   - For `ACK` packets, corresponds to the acknowledged packet sequence number.

2. **Packet Type (1 Byte, `uint8`):**
   - Declares the control or data category of the datagram (see Section 3).

3. **Payload Length (2 Bytes, `uint16`, Big-Endian):**
   - Length of the trailing payload buffer in bytes (0 to 65,535).

4. **Checksum (4 Bytes, `uint32`, Big-Endian):**
   - IEEE 802.3 CRC32 checksum computed across the first 7 header bytes plus the entire payload buffer.
   - Corrupted datagrams failing CRC verification are dropped immediately without ACK.

---

## 3. Packet Types

| Type Code | Constant Name | Semantics | Payload Format |
|---|---|---|---|
| `0x01` | `PRESENCE` | LAN broadcast / subnet beacon | Pipe-delimited: `<hostname>\|<session_id>\|<port>\|<hex_x25519_pubkey>` |
| `0x02` | `TRANSFER_REQUEST` | Outgoing file transfer initiation | Pipe-delimited: `<filename>\|<filesize_bytes>\|<crc32_hex>` |
| `0x03` | `TRANSFER_ACCEPT` | Recipient consent to receive file | Empty or optional handshake response |
| `0x04` | `TRANSFER_REJECT` | Recipient refusal of transfer | Empty or optional rejection reason |
| `0x05` | `MSG` | Direct P2P text message | AES-256-GCM Encrypted JSON: `{"sender":"...","content":"...","timestamp":"..."}` |
| `0x06` | `FILE_CHUNK` | Binary file slice | AES-256-GCM Encrypted slice (default: 1024 bytes) |
| `0x07` | `FILE_END` | Transfer completion sentinel | Empty |
| `0x08` | `ACK` | Transmission acknowledgement | Empty |

---

## 4. Protocol Phases

### 4.1 Peer Discovery & Key Exchange
1. Nodes periodically (every 1.0s) broadcast a `PRESENCE` packet containing their hostname, session ID, service port, and **32-byte X25519 Public Key** to LAN broadcast and across the active subnet.
2. Nodes listen on UDP port 9999 (or fallback ports 9998, 9997...).
3. Upon receiving a peer's presence announcement, both nodes perform **X25519 Elliptic Curve Diffie-Hellman (ECDH)** key agreement:
   $$\text{SharedSecret} = \text{ECDH}(\text{LocalPrivKey}, \text{RemotePubKey})$$
   $$\text{SymmetricKey} = \text{SHA-256}(\text{SharedSecret})$$
4. The derived 256-bit symmetric key is cached for all subsequent chat messages and file transfers with that peer.
5. Nodes not heard from within **10 seconds** are expired and pruned from the peer list.

### 4.2 Consent Handshake
1. **Initiator:** Sends `TRANSFER_REQUEST` with sequence $S$.
2. **Responder:** Interactively displays prompt with filename and human-formatted size.
3. **Response:**
   - If accepted, responder sends `TRANSFER_ACCEPT` with sequence $S+1$.
   - If declined, responder sends `TRANSFER_REJECT` with sequence $S+1$.
4. **Timeout:** If no response is received within **30 seconds**, initiator aborts cleanly.

### 4.3 Reliable Encrypted Data Transfer (Stop-and-Wait ARQ + AES-256-GCM)
1. Sender encrypts the file byte stream using **AES-256-GCM** with the derived symmetric key and a random 12-byte nonce.
2. Sender segments the encrypted payload into 1024-byte `FILE_CHUNK` packets with sequential sequence numbers.
3. For each chunk:
   - Sender transmits packet and starts retransmission timer (initial: 500ms).
   - Receiver validates CRC32, reorders chunk into memory buffer, and replies with `ACK` (matching sequence number).
   - If ACK is not received before deadline, sender retransmits with exponential backoff up to **6 retries**.
   - If maximum retries are exhausted, transfer is aborted with error.
4. Upon transmitting all chunks, sender transmits `FILE_END` packet.

### 4.4 End-to-End Decryption & Verification
1. Receiver verifies all contiguous sequence chunks $[S_{start}, S_{end}]$ are present.
2. Receiver decrypts the reassembled byte stream using AES-256-GCM with the peer's shared key and validates the GCM authentication tag.
3. Receiver calculates IEEE CRC32 checksum of the decrypted file data.
4. Receiver compares computed CRC32 against expected checksum received in `TRANSFER_REQUEST`.
5. If checksums match, file is committed to disk (`<filename>` or `received_<filename>`). If mismatch or authentication failure, file is discarded with error.

---

## 5. Security & Cryptographic Architecture
- **Zero Configuration End-to-End Encryption (E2EE):** Automatic key exchange over local Wi-Fi with no certificate authorities or centralized servers required.
- **Key Exchange:** Ephemeral X25519 Elliptic Curve Diffie-Hellman (RFC 7748 / NIST Curve25519) + SHA-256 key derivation.
- **Authenticated Symmetric Cipher:** AES-256 in Galois/Counter Mode (GCM) with 96-bit random nonce and 128-bit authentication tag.
- **Tamper Resistance:** Any in-transit packet modification or eavesdropping attempt on Wi-Fi fails GCM tag verification and is dropped immediately.
- **Pure Go Standard Library:** Zero external cryptographic dependencies (`crypto/ecdh`, `crypto/aes`, `crypto/cipher`, `crypto/sha256`, `crypto/rand`).

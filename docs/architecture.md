# Fling Architecture Design (HLD & LLD)

**Project:** Fling  
**Type:** System Architecture (High-Level & Low-Level Design)  
**Version:** 1.0.0  
**Author:** Satyam Kumar  

---

## 1. High-Level Design (HLD)

### 1.1 Overview
Fling is a serverless, peer-to-peer (P2P) file transfer and direct messaging application for local area networks. It operates entirely over raw UDP sockets (`net.UDPConn`), implementing its own acknowledgment-based transport layer (Stop-and-Wait ARQ) with zero reliance on TCP or cloud relays.

```text
┌──────────────────────────────────────────────────────────┐
│             Presentation Layer (Bubbletea TUI / CLI)     │
├──────────────────────────────────────────────────────────┤
│             Application Services (Discovery, Handshake)  │
├──────────────────────────────────────────────────────────┤
│             Reliable Transport & Router (ARQ, CRC32)     │
├──────────────────────────────────────────────────────────┤
│             Raw UDP Sockets (Port 9999 / Subnet Bcast)   │
└──────────────────────────────────────────────────────────┘
```

### 1.2 Core Subsystems

| Subsystem | Responsibilities | Key Package |
|---|---|---|
| **Peer Discovery** | Broadcasts presence beacons every 2s, listens on LAN, prunes inactive peers (>10s). | `internal/discovery` |
| **Handshake Manager** | Negotiates recipient consent before streaming file data (`REQUEST` $\rightarrow$ `ACCEPT`/`REJECT`). | `internal/handshake` |
| **Network Router** | Demultiplexes single UDP socket across concurrent readers/writers, routes ACKs & responses. | `internal/network` |
| **File Transfer** | 1024B chunking, out-of-order reassembly, end-to-end CRC32 verification, Finder reveal. | `internal/transfer` |
| **Messaging** | Real-time direct plain-text datagram delivery without file consent flow. | `internal/messaging` |
| **Terminal UI** | Full-screen multi-pane dashboard (Bubbletea + Lipgloss) with 2D modal compositing. | `internal/tui` |

---

## 2. Low-Level Design (LLD)

### 2.1 11-Byte Wire Packet Format

Every datagram uses a deterministic 11-byte binary header in Network Byte Order (Big-Endian):

```text
 0                   1                   2                   3
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                       Sequence Number (4B)                    |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|   Type (1B)   |      Payload Length (2B)      |   CRC32 (2B)  |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|       ... CRC32 Checksum (2B)                 |  Payload ...  |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                          ... Payload (Variable)               |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

| Field | Size | Description |
|---|---|---|
| **Sequence Number** | 4 Bytes (`uint32`) | Monotonically increasing packet counter |
| **Packet Type** | 1 Byte (`uint8`) | `Presence`(1), `TransferRequest`(2), `TransferAccept`(3), `TransferReject`(4), `Msg`(5), `FileChunk`(6), `FileEnd`(7), `ACK`(8) |
| **Payload Length** | 2 Bytes (`uint16`) | Size of trailing payload buffer (0–65,535 bytes) |
| **CRC32 Checksum** | 4 Bytes (`uint32`) | IEEE 802.3 CRC32 computed over header + payload |

---

### 2.2 Stop-and-Wait ARQ & Socket Multiplexing

To prevent socket contention while listening and sending simultaneously on port 9999:

1. **Background Reader Loop**: A single goroutine reads from `net.UDPConn`, validates CRC32, and dispatches packets:
   - `ACK` packets unblock active sender channels registered in `ackWaiters[seq]`.
   - Handshake responses unblock `respWaiters[seq]`.
   - Presence, Chat, and File chunks trigger asynchronous event callbacks (`OnPresence`, `OnMsg`, `OnFileChunk`).
2. **Retransmission Engine**:
   - Transmits packet with initial timeout of 500ms.
   - Retries up to **6 times** using exponential backoff ($500\text{ms} \times 1.5^{\text{retry}}$).
   - Aborts gracefully with error if max retries are exceeded.
3. **Chunk Reassembly**:
   - `StreamReceiver` buffers incoming chunks in memory indexed by sequence number.
   - Reorders chunks, detects duplicates, verifies sequence contiguity $[S_{\text{start}}, S_{\text{end}}]$, and validates the entire reconstructed file against the handshake CRC32 checksum.

---

### 2.3 Concurrency & Edge Cases

| Area | Solution |
|---|---|
| **Port Conflicts** | Automatic fallback cascading to ports `9998`, `9997`... if `9999` is bound. |
| **Subnet Isolation** | Calculates directed broadcast masks (`ip | ^mask`) across all active network interfaces. |
| **Thread Safety** | `sync.RWMutex` on peer maps and transaction router channels; `sync.Mutex` on chunk buffers. |
| **UI Rendering** | 2D ANSI line cutting (`cutANSILine`) overlays centered dialogs without cursor drift or screen tearing. |
| **Path Handling** | Auto-resolves tildes, quotes, drag-and-drop escaped spaces, and case variations. |

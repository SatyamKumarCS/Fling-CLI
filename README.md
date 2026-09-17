# Fling

> **Fast, Zero-Configuration Peer-to-Peer File Transfer & Messaging for your Terminal.**  
> AirDrop-like simplicity over local networks, built from scratch on raw UDP with a custom reliable ARQ protocol, automatic LAN peer discovery, and a modern Lazygit-style Terminal UI (TUI).

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://golang.org)
[![Mentioned in Awesome Go](https://awesome.re/mentioned-badge.svg)](https://github.com/avelino/awesome-go)
[![CI & PR Checks](https://github.com/SatyamKumarCS/Fling-CLI/actions/workflows/ci.yml/badge.svg)](https://github.com/SatyamKumarCS/Fling-CLI/actions/workflows/ci.yml)
[![Platform](https://img.shields.io/badge/Platform-macOS%20|%20Linux%20|%20Windows-blueviolet?style=flat)](https://github.com/SatyamKumarCS/Fling-CLI)
[![Tests](https://img.shields.io/badge/Tests-42%20Passed%20(100%25)-50FA7B?style=flat)](https://github.com/SatyamKumarCS/Fling-CLI)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

<br/>

<p align="center">
  <img src="assets/dashboard.png" alt="Fling Interactive TUI Dashboard" width="95%" />
</p>

---

## Features

- **Zero-Config Discovery**: Automatically finds active peers on the LAN via UDP subnet broadcast (stale peers pruned after 10s).
- **End-to-End Encryption (E2EE)**: Automatic zero-config **X25519 Elliptic Curve Diffie-Hellman (ECDH)** key agreement and **AES-256-GCM** authenticated encryption across all instant messages and file transfers (zero plaintext on wire).
- **Custom Reliable UDP Protocol**: Stop-and-Wait ARQ, sequence numbers, packet deduplication, and out-of-order chunk reassembly built directly on raw sockets (no TCP/HTTP).
- **Consent-First Handshake**: Senders request transfer with file metadata; transfers only begin upon recipient confirmation.
- **End-to-End CRC32 Integrity**: Packets and fully reassembled files are verified with IEEE CRC32 checksums (zero silent corruption).
- **Native File Manager Reveal**: Press <kbd>o</kbd> to immediately highlight received files in macOS Finder, Windows Explorer, or Linux File Managers.
- **Direct Encrypted Instant Messaging**: Chat directly with any online peer without initiating file transfers.

---

## Installation

> [!NOTE]
> **Prerequisite:** The **Go compiler (Go 1.21+)** is required to build and install Fling. Download Go from [go.dev/dl](https://go.dev/dl/) or install via your package manager (`brew install go` on macOS, `sudo apt install golang` on Debian/Ubuntu).

### Option 1: One-Line Installer (macOS & Linux)
```bash
curl -fsSL https://raw.githubusercontent.com/SatyamKumarCS/Fling-CLI/main/install.sh | bash
```

### Option 2: Go Install
```bash
go install github.com/SatyamKumarCS/Fling-CLI/cmd/fling@latest
```

### Option 3: Build from Source
```bash
git clone https://github.com/SatyamKumarCS/Fling-CLI.git
cd Fling-CLI
make install
```

---

## Usage

### 1. Interactive Dashboard (TUI)

Launch the multi-pane interface:
```bash
fling
```
*(Tip: use `fling --port 9998` to test multiple nodes on the same computer)*

| Key | Action | Description |
|---|---|---|
| <kbd>Tab</kbd> / <kbd>Shift+Tab</kbd> | **Switch Pane** | Cycle focus between Peers, Info, Chat, and Transfers |
| <kbd>1</kbd>–<kbd>4</kbd> | **Jump to Pane** | Directly activate Pane 1, 2, 3, or 4 |
| <kbd>↑</kbd> / <kbd>↓</kbd> or <kbd>k</kbd> / <kbd>j</kbd> | **Select Peer** | Highlight a peer in the Discovered Peers list |
| <kbd>F</kbd> / <kbd>s</kbd> | **Send File** | Open file dialog (press <kbd>Ctrl+O</kbd> for Finder picker or drag-and-drop) |
| <kbd>Enter</kbd> / <kbd>c</kbd> | **Chat** | Focus Chat pane to type a live message |
| <kbd>o</kbd> / <kbd>O</kbd> | **Reveal in Finder** | Reveal the received file in Finder / File Explorer |
| <kbd>y</kbd> / <kbd>n</kbd> | **Accept / Decline** | Accept or reject incoming transfer requests |
| <kbd>r</kbd> | **Scan** | Send immediate UDP presence beacon to refresh discovery |
| <kbd>?</kbd> | **Help** | Toggle keybindings cheat sheet modal |
| <kbd>q</kbd> | **Quit** | Exit Fling |

---

### 2. Headless CLI Mode

Use Fling in shell scripts or direct terminal commands:

```bash
# Discover active peers on your LAN
fling peers

# Send a file (by index, hostname, or IP)
fling send report.pdf --to 1
fling send archive.tar.gz --to Satyam-MacBook.local
fling send photo.png --to 192.168.1.50:9999

# Send a direct instant message
fling msg "Deployment complete on staging server." --to 1
```

---

## Architecture & Security Specification

- **System Architecture (HLD & LLD)**: For the full High-Level Design, Low-Level Design, and component state machines, see **[docs/architecture.md](docs/architecture.md)**.
- **Wire Protocol RFC Specification**: For the 11-byte binary packet wire format, Stop-and-Wait ARQ state machines, and retransmission timing, see **[docs/protocol.md](docs/protocol.md)**.
- **Security & Cryptography Policy**: For the complete End-to-End Encryption (E2EE), threat model, and vulnerability reporting policy, see **[SECURITY.md](SECURITY.md)**.

---

## Verification, Updates & Uninstallation

### Verify Installation
```bash
# Check installed version
fling version

# Check binary path in $PATH
which fling

# View command guide & flags
fling --help
```

### Updating Fling
```bash
# Option 1: Native CLI command
fling update

# Option 2: Via updater script
curl -fsSL https://raw.githubusercontent.com/SatyamKumarCS/Fling-CLI/main/update.sh | bash

# Option 3: Via Makefile
make update
```

### Uninstallation
```bash
# Option 1: Native CLI command
fling uninstall

# Option 2: Via Makefile
make uninstall
```

---

## Testing

Run the full automated test suite (including 30% simulated packet loss and race detection):

```bash
go test -v -race ./...
```

---

## Contributing

Contributions and issues are welcome! Feel free to check the [issues page](https://github.com/SatyamKumarCS/Fling-CLI/issues).

---

## License

MIT License © 2026 [Satyam Kumar](https://github.com/SatyamKumarCS)

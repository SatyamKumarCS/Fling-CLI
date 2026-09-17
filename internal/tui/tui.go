package tui

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/SatyamKumarCS/Fling-CLI/internal/cli"
	"github.com/SatyamKumarCS/Fling-CLI/internal/discovery"
	"github.com/SatyamKumarCS/Fling-CLI/internal/handshake"
	"github.com/SatyamKumarCS/Fling-CLI/internal/messaging"
	"github.com/SatyamKumarCS/Fling-CLI/internal/network"
	"github.com/SatyamKumarCS/Fling-CLI/internal/protocol"
	"github.com/SatyamKumarCS/Fling-CLI/internal/transfer"
)

// Pane identifiers (Lazygit-style multi-pane layout)
const (
	PanePeers = iota
	PaneNetwork
	PaneChat
	PaneTransfers
	TotalPanes
)

// ChatMessage represents a message in the live chat log
type ChatMessage struct {
	Sender    string
	Content   string
	Timestamp time.Time
	IsMe      bool
}

// TransferRecord represents a file transfer entry
type TransferRecord struct {
	Filename    string
	Size        int64
	Checksum    uint32
	Peer        string
	IsIncoming  bool
	Status      string // "In Progress", "Completed", "Rejected", "Failed"
	Transferred int64
	StartedAt   time.Time
}

// LogRecord represents a protocol event log entry
type LogRecord struct {
	Timestamp time.Time
	Level     string // "INFO", "DISCOVERY", "TRANSFER", "ERROR"
	Message   string
}

// IncomingTransferPrompt holds state for the accept/reject modal
type IncomingTransferPrompt struct {
	Request    handshake.TransferRequest
	SenderAddr *net.UDPAddr
	SeqNum     uint32
}

// Model is the main Bubbletea TUI model
type Model struct {
	Hostname  string
	SessionID string
	Port      int
	LANIPs    []string
	Router    *network.Router
	Discovery *discovery.Discovery
	Program   *tea.Program

	ActivePane      int
	SelectedPeerIdx int
	Width           int
	Height          int

	ChatInput   textinput.Model
	FileInput   textinput.Model
	ProgressBar progress.Model

	Peers     []discovery.Peer
	Messages  []ChatMessage
	Transfers []TransferRecord
	Logs      []LogRecord

	IncomingModal        *IncomingTransferPrompt
	ActiveTransfer       *TransferRecord
	ActiveStreamReceiver *transfer.StreamReceiver
	LastSavedFile        string
	Notification         string
	NotifExpiresAt       time.Time
	ShowHelp             bool
	SelectingFile        bool
	ConnectingPeer       bool
	PeerInput            textinput.Model
}

// Custom Bubbletea Messages
type (
	TickMsg                time.Time
	PeerEventMsg           struct{}
	NewIncomingMessageMsg  messaging.Message
	IncomingTransferReqMsg IncomingTransferPrompt
	FileChunkReceivedMsg   protocol.Packet
	FileEndReceivedMsg     protocol.Packet
	TransferProgressMsg    struct{ Transferred, Total int64 }
	TransferCompletedMsg   struct {
		Filename string
		Err      error
	}
	TransferRejectedMsg struct{ Filename string }
	AddLogMsg           LogRecord
	FilePickedMsg       string
)

// NewModel initializes the TUI model
func NewModel(hostname, sessionID string, port int, router *network.Router, disco *discovery.Discovery) Model {
	ti := textinput.New()
	ti.Placeholder = "Type a message... (Press Enter to send, Esc to cancel)"
	ti.CharLimit = 1000
	ti.Width = 40

	fi := textinput.New()
	fi.Placeholder = "Enter path to file (e.g. tests/test.txt)..."
	fi.CharLimit = 500
	fi.Width = 50

	pi := textinput.New()
	pi.Placeholder = "Enter peer IP (e.g. 10.7.5.142 or 192.168.1.50)..."
	pi.CharLimit = 100
	pi.Width = 50

	pb := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(30),
	)

	return Model{
		Hostname:        hostname,
		SessionID:       sessionID,
		Port:            port,
		LANIPs:          discovery.GetLocalIPs(),
		Router:          router,
		Discovery:       disco,
		ActivePane:      PanePeers,
		SelectedPeerIdx: 0,
		ChatInput:       ti,
		FileInput:       fi,
		PeerInput:       pi,
		ProgressBar:     pb,
		Peers:           disco.GetPeers(),
		Messages:        make([]ChatMessage, 0),
		Transfers:       make([]TransferRecord, 0),
		Logs:            make([]LogRecord, 0),
		Width:           100,
		Height:          30,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(),
		textinput.Blink,
	)
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

// Update handles state changes and user input
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		rightColWidth := m.Width - max(34, int(float64(m.Width)*0.38)) - 4
		if rightColWidth > 10 {
			m.ChatInput.Width = rightColWidth - 4
			m.FileInput.Width = rightColWidth - 4
			m.ProgressBar.Width = rightColWidth - 8
		}

	case TickMsg:
		if m.Router != nil {
			_ = m.Discovery.BroadcastPresence(m.Router.Conn(), discovery.DiscoveryPort)
		}
		removed := m.Discovery.PruneStalePeers()
		if len(removed) > 0 {
			for _, rp := range removed {
				m.addLog("DISCOVERY", fmt.Sprintf("Peer timed out: %s (%s)", rp.Hostname, rp.Addr()))
			}
		}
		m.Peers = m.Discovery.GetPeers()
		if m.SelectedPeerIdx >= len(m.Peers) && len(m.Peers) > 0 {
			m.SelectedPeerIdx = len(m.Peers) - 1
		}
		cmds = append(cmds, tickCmd())

	case PeerEventMsg:
		m.Peers = m.Discovery.GetPeers()

	case NewIncomingMessageMsg:
		// Deduplicate: check if last message from same sender with identical content arrived in last 3 seconds
		isDup := false
		now := time.Now()
		for i := len(m.Messages) - 1; i >= 0 && i >= len(m.Messages)-5; i-- {
			if !m.Messages[i].IsMe &&
				m.Messages[i].Sender == msg.Sender &&
				m.Messages[i].Content == msg.Content &&
				now.Sub(m.Messages[i].Timestamp) < 3*time.Second {
				isDup = true
				break
			}
		}
		if isDup {
			return m, nil
		}

		m.Messages = append(m.Messages, ChatMessage{
			Sender:    msg.Sender,
			Content:   msg.Content,
			Timestamp: msg.Timestamp,
			IsMe:      false,
		})
		m.addLog("MSG", fmt.Sprintf("From %s: %s", msg.Sender, msg.Content))
		m.setNotification(fmt.Sprintf("Message from %s", msg.Sender))

	case IncomingTransferReqMsg:
		prompt := IncomingTransferPrompt(msg)
		m.IncomingModal = &prompt
		m.addLog("TRANSFER", fmt.Sprintf("Incoming file request: %s (%s)", msg.Request.Filename, cli.FormatBytes(msg.Request.FileSize)))
		m.setNotification(fmt.Sprintf("Transfer request from %s", msg.SenderAddr.String()))

	case FileChunkReceivedMsg:
		if m.ActiveStreamReceiver != nil {
			done, err := m.ActiveStreamReceiver.ProcessChunk(protocol.Packet(msg))
			if done {
				m.ActiveStreamReceiver = nil
				if m.ActiveTransfer != nil {
					if err != nil {
						m.ActiveTransfer.Status = "Failed"
						m.addLog("ERROR", fmt.Sprintf("Transfer failed: %v", err))
						m.setNotification(fmt.Sprintf("Transfer failed: %v", err))
					} else {
						m.ActiveTransfer.Status = "Completed"
						savedFile := m.ActiveTransfer.Filename
						if abs, err := filepath.Abs(savedFile); err == nil {
							m.LastSavedFile = abs
						} else {
							m.LastSavedFile = savedFile
						}
						m.addLog("TRANSFER", fmt.Sprintf("File saved: %s (Press 'o' to reveal in Finder)", savedFile))
						m.setNotification(fmt.Sprintf("Saved %s (Press 'o' for Finder)", filepath.Base(savedFile)))
					}
					m.ActiveTransfer = nil
				}
			}
		}

	case FileEndReceivedMsg:
		if m.ActiveStreamReceiver != nil {
			done, err := m.ActiveStreamReceiver.ProcessChunk(protocol.Packet(msg))
			if done {
				m.ActiveStreamReceiver = nil
				if m.ActiveTransfer != nil {
					if err != nil {
						m.ActiveTransfer.Status = "Failed"
						m.addLog("ERROR", fmt.Sprintf("Transfer failed: %v", err))
						m.setNotification(fmt.Sprintf("Transfer failed: %v", err))
					} else {
						m.ActiveTransfer.Status = "Completed"
						savedFile := m.ActiveTransfer.Filename
						if abs, err := filepath.Abs(savedFile); err == nil {
							m.LastSavedFile = abs
						} else {
							m.LastSavedFile = savedFile
						}
						m.addLog("TRANSFER", fmt.Sprintf("File saved: %s (Press 'o' to reveal in Finder)", savedFile))
						m.setNotification(fmt.Sprintf("Saved %s (Press 'o' for Finder)", filepath.Base(savedFile)))
					}
					m.ActiveTransfer = nil
				}
			}
		}

	case TransferProgressMsg:
		if m.ActiveTransfer != nil {
			m.ActiveTransfer.Transferred = msg.Transferred
		}
		var pct float64
		if msg.Total > 0 {
			pct = float64(msg.Transferred) / float64(msg.Total)
		}
		cmds = append(cmds, m.ProgressBar.SetPercent(pct))

	case TransferCompletedMsg:
		if m.ActiveTransfer != nil {
			if msg.Err != nil {
				m.ActiveTransfer.Status = "Failed"
				m.addLog("ERROR", fmt.Sprintf("Transfer failed for %s: %v", msg.Filename, msg.Err))
				m.setNotification(fmt.Sprintf("Transfer failed: %v", msg.Err))
			} else {
				m.ActiveTransfer.Status = "Completed"
				if msg.Filename != "" && m.LastSavedFile == "" {
					if abs, err := filepath.Abs(msg.Filename); err == nil {
						m.LastSavedFile = abs
					} else {
						m.LastSavedFile = msg.Filename
					}
				}
				m.addLog("TRANSFER", fmt.Sprintf("Transfer completed: %s", msg.Filename))
				m.setNotification(fmt.Sprintf("Transferred: %s", filepath.Base(msg.Filename)))
			}
			m.ActiveTransfer = nil
		}

	case TransferRejectedMsg:
		if m.ActiveTransfer != nil {
			m.ActiveTransfer.Status = "Rejected"
			m.addLog("TRANSFER", fmt.Sprintf("Transfer declined: %s", msg.Filename))
			m.setNotification(fmt.Sprintf("Peer declined file: %s", msg.Filename))
			m.ActiveTransfer = nil
		}

	case AddLogMsg:
		m.addLog(msg.Level, msg.Message)

	case FilePickedMsg:
		if string(msg) != "" {
			m.FileInput.SetValue(string(msg))
			m.setNotification(fmt.Sprintf("Selected: %s", filepath.Base(string(msg))))
		}

	case tea.KeyMsg:
		// Help modal handling (close on Esc, ?, q, Enter, Space)
		if m.ShowHelp {
			switch msg.String() {
			case "esc", "q", "?", "enter", " ":
				m.ShowHelp = false
				return m, nil
			}
			return m, nil
		}

		// Modal handling (Accept / Reject incoming transfer)
		if m.IncomingModal != nil {
			switch msg.String() {
			case "y", "Y", "enter":
				prompt := *m.IncomingModal
				m.IncomingModal = nil
				cmds = append(cmds, m.acceptIncomingTransfer(prompt))
				return m, tea.Batch(cmds...)
			case "n", "N", "esc":
				prompt := *m.IncomingModal
				m.IncomingModal = nil
				cmds = append(cmds, m.rejectIncomingTransfer(prompt))
				return m, tea.Batch(cmds...)
			}
			return m, nil
		}

		// Direct Peer IP connection dialog handling
		if m.ConnectingPeer {
			switch msg.String() {
			case "esc":
				m.ConnectingPeer = false
				m.PeerInput.Reset()
				return m, nil
			case "enter":
				targetIP := strings.TrimSpace(m.PeerInput.Value())
				m.ConnectingPeer = false
				m.PeerInput.Reset()
				if targetIP != "" && m.Router != nil {
					err := m.Discovery.PingPeer(m.Router.Conn(), targetIP, discovery.DiscoveryPort)
					if err != nil {
						m.addLog("ERROR", fmt.Sprintf("Failed to ping peer %s: %v", targetIP, err))
						m.setNotification(fmt.Sprintf("Ping failed: %v", err))
					} else {
						m.addLog("DISCOVERY", fmt.Sprintf("Sent direct presence ping to %s", targetIP))
						m.setNotification(fmt.Sprintf("Pinging peer at %s...", targetIP))
					}
				}
				return m, nil
			}
			var cmd tea.Cmd
			m.PeerInput, cmd = m.PeerInput.Update(msg)
			return m, cmd
		}

		// File path prompt dialog handling
		if m.SelectingFile {
			switch msg.String() {
			case "esc":
				m.SelectingFile = false
				m.FileInput.Reset()
				return m, nil
			case "ctrl+o", "ctrl+f":
				return m, func() tea.Msg {
					path, err := cli.PickFileWithNativeDialog()
					if err != nil || path == "" {
						return nil
					}
					return FilePickedMsg(path)
				}
			case "enter":
				filePath := strings.TrimSpace(m.FileInput.Value())
				m.SelectingFile = false
				m.FileInput.Reset()
				if filePath != "" {
					cmds = append(cmds, m.initiateFileSend(filePath))
				}
				return m, tea.Batch(cmds...)
			}
			var cmd tea.Cmd
			m.FileInput, cmd = m.FileInput.Update(msg)
			return m, cmd
		}

		// Lazygit-style navigation & shortcuts
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit

		case "q":
			if !m.ChatInput.Focused() {
				return m, tea.Quit
			}

		case "tab":
			m.ActivePane = (m.ActivePane + 1) % TotalPanes
			if m.ActivePane == PaneChat {
				m.ChatInput.Focus()
			} else {
				m.ChatInput.Blur()
			}
			return m, nil

		case "shift+tab":
			m.ActivePane = (m.ActivePane + TotalPanes - 1) % TotalPanes
			if m.ActivePane == PaneChat {
				m.ChatInput.Focus()
			} else {
				m.ChatInput.Blur()
			}
			return m, nil

		case "1":
			if !m.ChatInput.Focused() {
				m.ActivePane = PanePeers
				m.ChatInput.Blur()
				return m, nil
			}

		case "2":
			if !m.ChatInput.Focused() {
				m.ActivePane = PaneNetwork
				m.ChatInput.Blur()
				return m, nil
			}

		case "3":
			if !m.ChatInput.Focused() {
				m.ActivePane = PaneChat
				m.ChatInput.Focus()
				return m, nil
			}

		case "4":
			if !m.ChatInput.Focused() {
				m.ActivePane = PaneTransfers
				m.ChatInput.Blur()
				return m, nil
			}

		case "s", "S", "f", "F":
			if !m.ChatInput.Focused() {
				m.SelectingFile = true
				m.FileInput.Focus()
				if len(m.Peers) == 0 {
					m.setNotification("Note: No peers discovered yet on LAN")
				}
				return m, nil
			}

		case "c", "C", "m", "M", " ":
			if !m.ChatInput.Focused() {
				m.ActivePane = PaneChat
				m.ChatInput.Focus()
				return m, nil
			}

		case "p", "P", "a", "A":
			if !m.ChatInput.Focused() {
				m.ConnectingPeer = true
				m.PeerInput.Focus()
				return m, nil
			}

		case "r", "R":
			if !m.ChatInput.Focused() {
				if m.Router != nil {
					_ = m.Discovery.BroadcastPresence(m.Router.Conn(), discovery.DiscoveryPort)
				}
				m.setNotification("Scanning LAN & local subnet for active peers...")
				return m, nil
			}

		case "o", "O", "v", "V":
			if !m.ChatInput.Focused() {
				target := m.LastSavedFile
				if target == "" {
					target = "."
				}
				err := cli.RevealInFileManager(target)
				if err != nil {
					m.addLog("ERROR", fmt.Sprintf("Failed to open Finder: %v", err))
					m.setNotification(fmt.Sprintf("Finder error: %v", err))
				} else {
					if m.LastSavedFile != "" {
						m.addLog("INFO", fmt.Sprintf("Revealed in Finder: %s", filepath.Base(m.LastSavedFile)))
						m.setNotification(fmt.Sprintf("Revealed in Finder: %s", filepath.Base(m.LastSavedFile)))
					} else {
						m.addLog("INFO", "Opened working folder in Finder")
						m.setNotification("Opened working folder in Finder")
					}
				}
				return m, nil
			}

		case "esc":
			if m.ChatInput.Focused() {
				m.ChatInput.Blur()
				m.ActivePane = PanePeers
				return m, nil
			}

		case "up", "k":
			if m.ActivePane == PanePeers && m.SelectedPeerIdx > 0 {
				m.SelectedPeerIdx--
				return m, nil
			}

		case "down", "j":
			if m.ActivePane == PanePeers && m.SelectedPeerIdx < len(m.Peers)-1 {
				m.SelectedPeerIdx++
				return m, nil
			}

		case "?":
			if !m.ChatInput.Focused() {
				m.ShowHelp = !m.ShowHelp
				return m, nil
			}

		case "enter":
			if m.ActivePane == PanePeers {
				// Jump to chat from peers list on Enter
				m.ActivePane = PaneChat
				m.ChatInput.Focus()
				return m, nil
			}

			if m.ActivePane == PaneChat && strings.TrimSpace(m.ChatInput.Value()) != "" {
				text := strings.TrimSpace(m.ChatInput.Value())
				m.ChatInput.Reset()

				if len(m.Peers) == 0 {
					m.setNotification("No peers online to receive message.")
					m.addLog("WARN", "Message not sent: No peers discovered yet.")
					return m, nil
				}

				// Immediately append to local messages so sender sees their own message
				m.Messages = append(m.Messages, ChatMessage{
					Sender:    m.Hostname,
					Content:   text,
					Timestamp: time.Now(),
					IsMe:      true,
				})

				cmds = append(cmds, m.sendChatMessage(text))
				return m, tea.Batch(cmds...)
			}
		}
	}

	// Update active textinput
	if m.ActivePane == PaneChat && !m.SelectingFile {
		var cmd tea.Cmd
		m.ChatInput, cmd = m.ChatInput.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) addLog(level, message string) {
	m.Logs = append(m.Logs, LogRecord{
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
	})
	if len(m.Logs) > 100 {
		m.Logs = m.Logs[len(m.Logs)-100:]
	}
}

func (m *Model) setNotification(text string) {
	m.Notification = text
	m.NotifExpiresAt = time.Now().Add(4 * time.Second)
}

func (m Model) sendChatMessage(text string) tea.Cmd {
	return func() tea.Msg {
		if len(m.Peers) == 0 {
			return AddLogMsg{Timestamp: time.Now(), Level: "WARN", Message: "Cannot send message: no peers online."}
		}

		targetPeer := m.Peers[m.SelectedPeerIdx]
		targetAddr, err := net.ResolveUDPAddr("udp4", targetPeer.Addr())
		if err != nil {
			return AddLogMsg{Timestamp: time.Now(), Level: "ERROR", Message: fmt.Sprintf("Invalid peer addr: %v", err)}
		}

		sharedKey, _ := m.Discovery.GetPeerSharedKey(targetPeer.SessionID)
		packet, err := messaging.CreateEncryptedMessage(m.Hostname, text, sharedKey, uint32(time.Now().UnixNano()&0xFFFF))
		if err != nil {
			return AddLogMsg{Timestamp: time.Now(), Level: "ERROR", Message: fmt.Sprintf("Message create failed: %v", err)}
		}

		err = m.Router.SendReliable(packet, targetAddr)
		if err != nil {
			return AddLogMsg{Timestamp: time.Now(), Level: "ERROR", Message: fmt.Sprintf("Send failed: %v", err)}
		}

		return AddLogMsg{
			Timestamp: time.Now(),
			Level:     "MSG",
			Message:   fmt.Sprintf("To %s: %s", targetPeer.Hostname, text),
		}
	}
}

func (m Model) initiateFileSend(filePath string) tea.Cmd {
	return func() tea.Msg {
		if len(m.Peers) == 0 {
			if m.Program != nil {
				m.Program.Send(TransferCompletedMsg{
					Filename: filepath.Base(filePath),
					Err:      fmt.Errorf("no peers online to receive file"),
				})
			}
			return AddLogMsg{Timestamp: time.Now(), Level: "WARN", Message: "Cannot send file: no peers online."}
		}

		targetPeer := m.Peers[m.SelectedPeerIdx]
		targetAddr, err := net.ResolveUDPAddr("udp4", targetPeer.Addr())
		if err != nil {
			return AddLogMsg{Timestamp: time.Now(), Level: "ERROR", Message: fmt.Sprintf("Invalid peer: %v", err)}
		}

		resolvedPath, fileInfo, err := transfer.ResolveFilePath(filePath)
		if err != nil {
			return AddLogMsg{Timestamp: time.Now(), Level: "ERROR", Message: fmt.Sprintf("File not found: %s", filePath)}
		}
		if fileInfo.IsDir() {
			return AddLogMsg{Timestamp: time.Now(), Level: "ERROR", Message: "Directories cannot be transferred directly."}
		}
		filePath = resolvedPath

		checksum, size, err := transfer.CalculateFileChecksum(filePath)
		if err != nil {
			return AddLogMsg{Timestamp: time.Now(), Level: "ERROR", Message: fmt.Sprintf("Checksum failed: %v", err)}
		}

		seq := uint32(time.Now().UnixNano() & 0xFFFF)
		reqPacket := handshake.CreateTransferRequest(filepath.Base(filePath), size, checksum, seq)

		// Register response waiter on router for TransferAccept / TransferReject
		respCh := m.Router.RegisterResponseWaiter(seq)
		defer m.Router.DeregisterResponseWaiter(seq)

		err = m.Router.SendReliable(reqPacket, targetAddr)
		if err != nil {
			return AddLogMsg{Timestamp: time.Now(), Level: "ERROR", Message: fmt.Sprintf("Handshake request failed: %v", err)}
		}

		if m.Program != nil {
			m.Program.Send(AddLogMsg{
				Timestamp: time.Now(),
				Level:     "TRANSFER",
				Message:   fmt.Sprintf("Requested transfer: %s (%s) to %s. Waiting for accept...", filepath.Base(filePath), cli.FormatBytes(size), targetPeer.Hostname),
			})
		}

		// Wait for peer accept/reject
		select {
		case resp := <-respCh:
			if resp.Type == protocol.TransferReject {
				if m.Program != nil {
					m.Program.Send(TransferRejectedMsg{Filename: filepath.Base(filePath)})
				}
				return nil
			}
		case <-time.After(30 * time.Second):
			if m.Program != nil {
				m.Program.Send(TransferCompletedMsg{
					Filename: filepath.Base(filePath),
					Err:      fmt.Errorf("peer response timed out"),
				})
			}
			return nil
		}

		// Transfer Accepted! Send File Chunks (Encrypted with E2EE shared key if available)
		if m.Program != nil {
			m.Program.Send(AddLogMsg{
				Timestamp: time.Now(),
				Level:     "TRANSFER",
				Message:   fmt.Sprintf("Transfer accepted! Uploading %s...", filepath.Base(filePath)),
			})
		}

		sharedKey, _ := m.Discovery.GetPeerSharedKey(targetPeer.SessionID)
		_, sendErr := transfer.SendFileWithRouterAndKey(
			m.Router,
			targetAddr,
			filePath,
			sharedKey,
			seq+2,
			func(transferred, total int64) {
				if m.Program != nil {
					m.Program.Send(TransferProgressMsg{Transferred: transferred, Total: total})
				}
			},
		)

		if m.Program != nil {
			m.Program.Send(TransferCompletedMsg{Filename: filepath.Base(filePath), Err: sendErr})
		}

		return nil
	}
}

func (m *Model) acceptIncomingTransfer(prompt IncomingTransferPrompt) tea.Cmd {
	acceptPacket := handshake.CreateTransferAccept(prompt.SeqNum + 1)
	encoded, err := protocol.Encode(acceptPacket)
	if err == nil {
		_, _ = m.Router.Conn().WriteToUDP(encoded, prompt.SenderAddr)
	}

	destPath := filepath.Base(prompt.Request.Filename)
	if _, err := os.Stat(destPath); err == nil {
		destPath = "received_" + destPath
	}

	m.ActiveTransfer = &TransferRecord{
		Filename:    destPath,
		Size:        prompt.Request.FileSize,
		Checksum:    prompt.Request.Checksum,
		Peer:        prompt.SenderAddr.String(),
		IsIncoming:  true,
		Status:      "Downloading",
		Transferred: 0,
		StartedAt:   time.Now(),
	}

	m.ActiveStreamReceiver = transfer.NewStreamReceiver(
		prompt.SeqNum+2,
		prompt.Request.FileSize,
		prompt.Request.Checksum,
		destPath,
		func(transferred, total int64) {
			if m.Program != nil {
				m.Program.Send(TransferProgressMsg{Transferred: transferred, Total: total})
			}
		},
	)
	sharedKey, _ := m.Discovery.GetPeerSharedKeyByAddr(prompt.SenderAddr.String())
	m.ActiveStreamReceiver.SetKey(sharedKey)

	return func() tea.Msg {
		return AddLogMsg{
			Timestamp: time.Now(),
			Level:     "TRANSFER",
			Message:   fmt.Sprintf("Downloading %s (%s)...", destPath, cli.FormatBytes(prompt.Request.FileSize)),
		}
	}
}

func (m Model) rejectIncomingTransfer(prompt IncomingTransferPrompt) tea.Cmd {
	return func() tea.Msg {
		rejectPacket := handshake.CreateTransferReject(prompt.SeqNum + 1)
		encoded, _ := protocol.Encode(rejectPacket)
		_, _ = m.Router.Conn().WriteToUDP(encoded, prompt.SenderAddr)

		return AddLogMsg{
			Timestamp: time.Now(),
			Level:     "TRANSFER",
			Message:   fmt.Sprintf("Declined file transfer: %s", prompt.Request.Filename),
		}
	}
}

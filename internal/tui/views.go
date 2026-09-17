package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/SatyamKumarCS/Fling-CLI/internal/cli"
)

// View renders the Lazygit-style multi-pane full-screen interface
func (m Model) View() string {
	width := m.Width
	height := m.Height
	if width < 60 {
		width = 60
	}
	if height < 15 {
		height = 15
	}

	// 1. Top Brand & Status Header (Fixed 1 line)
	header := m.renderHeader(width)

	// 2. Multi-Pane Grid (Fixed height - 2 lines)
	availableHeight := height - 2
	if availableHeight < 10 {
		availableHeight = 10
	}

	leftColWidth := int(float64(width) * 0.38)
	if leftColWidth < 34 {
		leftColWidth = 34
	}
	rightColWidth := width - leftColWidth - 2
	if rightColWidth < 30 {
		rightColWidth = 30
	}

	topPaneHeight := int(float64(availableHeight) * 0.56)
	if topPaneHeight < 6 {
		topPaneHeight = 6
	}
	bottomPaneHeight := availableHeight - topPaneHeight
	if bottomPaneHeight < 4 {
		bottomPaneHeight = 4
	}

	// Left Column Panes
	leftTopPane := m.renderPeersPane(leftColWidth, topPaneHeight)
	leftBottomPane := m.renderNetworkPane(leftColWidth, bottomPaneHeight)
	leftColumn := lipgloss.JoinVertical(lipgloss.Left, leftTopPane, leftBottomPane)

	// Right Column Panes
	rightTopPane := m.renderChatPane(rightColWidth, topPaneHeight)
	rightBottomPane := m.renderTransfersLogsPane(rightColWidth, bottomPaneHeight)
	rightColumn := lipgloss.JoinVertical(lipgloss.Left, rightTopPane, rightBottomPane)

	// Join Columns Side by Side into Main Grid
	mainGrid := lipgloss.JoinHorizontal(lipgloss.Top, leftColumn, " ", rightColumn)

	// 3. Footer Quick Command Bar (Fixed 1 line)
	footer := m.renderFooter(width)

	// Join all 3 vertical tiers into exact full-screen layout
	baseScreen := lipgloss.JoinVertical(lipgloss.Left, header, mainGrid, footer)

	// 4. If any Modal/Panel is active, overlay it directly on top of the running screen in the center
	if m.ShowHelp {
		modal := m.renderHelpBox(width, height)
		return overlayModal(baseScreen, modal, width, height)
	}

	if m.SelectingFile {
		modal := m.renderFileDialogBox(width, height)
		return overlayModal(baseScreen, modal, width, height)
	}

	if m.IncomingModal != nil {
		modal := m.renderIncomingModalBox(width, height)
		return overlayModal(baseScreen, modal, width, height)
	}

	return baseScreen
}

func (m Model) renderHeader(width int) string {
	title := StyleAppTitle.Render("FLING")
	version := StyleVersionBadge.Render("v" + cli.Version)

	primaryIP := "127.0.0.1"
	if len(m.LANIPs) > 0 {
		primaryIP = m.LANIPs[0]
	}

	nodePill := StyleStatusPill.Render("NODE: " + truncateString(m.Hostname, 18))
	ipPill := lipgloss.NewStyle().
		Foreground(ColorSecondary).
		Background(ColorSubtle).
		Bold(true).
		Padding(0, 1).
		MarginLeft(1).
		Render(fmt.Sprintf("IP: %s:%d", primaryIP, m.Port))

	sessionPill := lipgloss.NewStyle().
		Foreground(ColorMuted).
		Background(ColorBgDark).
		Padding(0, 1).
		MarginLeft(1).
		Render("ID:" + m.SessionID)

	onlinePill := StyleStatusOnline.Render(" [ONLINE]")

	var items []string
	items = append(items, title, version, nodePill, ipPill, sessionPill, onlinePill)

	if m.Notification != "" && time.Now().Before(m.NotifExpiresAt) {
		notifBadge := lipgloss.NewStyle().
			Background(ColorWarning).
			Foreground(ColorBgDark).
			Bold(true).
			Padding(0, 1).
			MarginLeft(1).
			Render("[!] " + truncateString(m.Notification, 32))
		items = append(items, notifBadge)
	}

	return lipgloss.JoinHorizontal(lipgloss.Center, items...)
}

func (m Model) renderPeersPane(width, height int) string {
	focused := m.ActivePane == PanePeers
	style := MakePane(focused, width, height)

	var titleStyle lipgloss.Style
	if focused {
		titleStyle = StylePaneTitleActive
	} else {
		titleStyle = StylePaneTitleInactive
	}
	title := titleStyle.Render(fmt.Sprintf("[1] DISCOVERED PEERS (%d)", len(m.Peers)))

	var content strings.Builder
	content.WriteString(title + "\n")

	if len(m.Peers) == 0 {
		cardStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorSubtle).
			Padding(0, 1).
			MarginTop(1).
			Width(width - 6)

		cardContent := lipgloss.JoinVertical(
			lipgloss.Left,
			lipgloss.NewStyle().Bold(true).Foreground(ColorSecondary).Render("LAN Peer Discovery Active"),
			lipgloss.NewStyle().Foreground(ColorTextDim).Render("Scanning local subnet for peers..."),
			"",
			lipgloss.NewStyle().Foreground(ColorMuted).Render("Test 2nd node in a new terminal tab:"),
			lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Render("$ fling --port 9998"),
		)
		content.WriteString(cardStyle.Render(cardContent))
	} else {
		content.WriteString(StyleTableHeader.Render(fmt.Sprintf(" %-4s %-15s %-16s %s", "#", "HOSTNAME", "ADDRESS", "STATUS")) + "\n")
		for i, p := range m.Peers {
			cursor := "  "
			rowStyle := StyleTableRow
			statusBadge := lipgloss.NewStyle().Foreground(ColorSuccess).Render("[ACTIVE]")

			if i == m.SelectedPeerIdx {
				cursor = "> "
				rowStyle = StyleTableRowSelected
				statusBadge = lipgloss.NewStyle().Bold(true).Foreground(ColorSuccess).Render("[ONLINE]")
			}
			rowText := fmt.Sprintf("%s%-3d %-15s %-16s %s", cursor, i+1, truncateString(p.Hostname, 14), truncateString(p.Addr(), 15), statusBadge)
			content.WriteString(rowStyle.Render(rowText) + "\n")
		}
	}

	return style.Render(content.String())
}

func (m Model) renderNetworkPane(width, height int) string {
	focused := m.ActivePane == PaneNetwork
	style := MakePane(focused, width, height)

	var titleStyle lipgloss.Style
	if focused {
		titleStyle = StylePaneTitleActive
	} else {
		titleStyle = StylePaneTitleInactive
	}
	title := titleStyle.Render("[2] LOCAL NODE INFO")

	primaryIP := "127.0.0.1"
	if len(m.LANIPs) > 0 {
		primaryIP = m.LANIPs[0]
	}

	lblStyle := lipgloss.NewStyle().Foreground(ColorMuted).Width(10)
	valStyle := lipgloss.NewStyle().Bold(true).Foreground(ColorSecondary)
	txtStyle := lipgloss.NewStyle().Foreground(ColorTextLight)

	var content strings.Builder
	content.WriteString(title + "\n")
	content.WriteString(fmt.Sprintf(" %s %s\n", lblStyle.Render("LAN IP:"), valStyle.Render(fmt.Sprintf("%s:%d", primaryIP, m.Port))))
	content.WriteString(fmt.Sprintf(" %s %s\n", lblStyle.Render("Protocol:"), txtStyle.Render("Reliable UDP (ARQ + CRC32)")))
	content.WriteString(fmt.Sprintf(" %s %s\n", lblStyle.Render("Session:"), lipgloss.NewStyle().Foreground(ColorPrimary).Render(m.SessionID)))
	content.WriteString(fmt.Sprintf(" %s %s\n", lblStyle.Render("Invite:"), lipgloss.NewStyle().Foreground(ColorAccent).Render(fmt.Sprintf("fling send <file> --to %s:%d", primaryIP, m.Port))))

	return style.Render(content.String())
}

func (m Model) renderChatPane(width, height int) string {
	focused := m.ActivePane == PaneChat
	style := MakePane(focused, width, height)

	var titleStyle lipgloss.Style
	if focused {
		titleStyle = StylePaneTitleActive
	} else {
		titleStyle = StylePaneTitleInactive
	}

	var targetDesc string
	if len(m.Peers) > 0 && m.SelectedPeerIdx < len(m.Peers) {
		p := m.Peers[m.SelectedPeerIdx]
		targetDesc = fmt.Sprintf("with %s (%s)", p.Hostname, p.Addr())
	} else {
		targetDesc = "(select peer to chat)"
	}
	title := titleStyle.Render(fmt.Sprintf("[3] LIVE CHAT %s", targetDesc))

	var content strings.Builder
	content.WriteString(title + "\n")

	// Message history list
	maxMsgLines := height - 5
	if maxMsgLines < 2 {
		maxMsgLines = 2
	}

	startIdx := 0
	if len(m.Messages) > maxMsgLines {
		startIdx = len(m.Messages) - maxMsgLines
	}

	if len(m.Messages) == 0 {
		welcomeCard := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorSubtle).
			Padding(0, 1).
			MarginTop(1).
			Width(width - 6)

		welcomeText := lipgloss.JoinVertical(
			lipgloss.Left,
			lipgloss.NewStyle().Bold(true).Foreground(ColorSecondary).Render("Direct P2P Instant Messaging"),
			lipgloss.NewStyle().Foreground(ColorTextDim).Render("Encrypted datagrams delivered directly between peers without servers."),
			"",
			lipgloss.NewStyle().Foreground(ColorMuted).Render("Press [Enter] or [m] to type a message."),
		)
		content.WriteString(welcomeCard.Render(welcomeText) + "\n")
	} else {
		for _, msg := range m.Messages[startIdx:] {
			ts := StyleChatTimestamp.Render(msg.Timestamp.Format("[15:04]"))
			var sender string
			if msg.IsMe {
				sender = StyleChatSenderMe.Render("You: ")
			} else {
				sender = StyleChatSenderPeer.Render(fmt.Sprintf("[%s]: ", truncateString(msg.Sender, 10)))
			}
			text := StyleChatMessageText.Render(truncateString(msg.Content, width-26))
			content.WriteString(fmt.Sprintf(" %s %s%s\n", ts, sender, text))
		}
	}

	// Bottom chat input container
	inputBoxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorSubtle)
	if focused {
		inputBoxStyle = inputBoxStyle.BorderForeground(ColorSecondary)
	}

	content.WriteString("\n")
	content.WriteString(inputBoxStyle.Render(" > " + m.ChatInput.View()))

	return style.Render(content.String())
}

func (m Model) renderTransfersLogsPane(width, height int) string {
	focused := m.ActivePane == PaneTransfers
	style := MakePane(focused, width, height)

	var titleStyle lipgloss.Style
	if focused {
		titleStyle = StylePaneTitleActive
	} else {
		titleStyle = StylePaneTitleInactive
	}
	title := titleStyle.Render("[4] TRANSFERS & PROTOCOL ACTIVITY")

	var content strings.Builder
	content.WriteString(title + "\n")

	// Active transfer bar if in progress
	if m.ActiveTransfer != nil {
		transferBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorWarning).
			Padding(0, 1).
			Width(width - 6)

		transferContent := lipgloss.JoinVertical(
			lipgloss.Left,
			lipgloss.NewStyle().Bold(true).Foreground(ColorWarning).Render(
				fmt.Sprintf("Transferring: %s (%s)", m.ActiveTransfer.Filename, cli.FormatBytes(m.ActiveTransfer.Size)),
			),
			m.ProgressBar.View(),
		)
		content.WriteString(transferBox.Render(transferContent) + "\n")
	}

	// Recent Logs
	maxLogs := height - 4
	if maxLogs < 2 {
		maxLogs = 2
	}

	startLog := 0
	if len(m.Logs) > maxLogs {
		startLog = len(m.Logs) - maxLogs
	}

	if len(m.Logs) == 0 {
		content.WriteString(lipgloss.NewStyle().Foreground(ColorMuted).Render("  Listening for UDP datagrams & discovery beacons...\n"))
	} else {
		for _, l := range m.Logs[startLog:] {
			ts := StyleChatTimestamp.Render(l.Timestamp.Format("15:04:05"))
			var lvlBg lipgloss.Color
			var lvlFg lipgloss.Color = ColorBgDark
			switch l.Level {
			case "DISCOVERY":
				lvlBg = ColorSecondary
			case "TRANSFER":
				lvlBg = ColorWarning
			case "ERROR":
				lvlBg = ColorDanger
				lvlFg = ColorTextWhite
			default:
				lvlBg = ColorPrimary
			}
			badge := lipgloss.NewStyle().Foreground(lvlFg).Background(lvlBg).Bold(true).Padding(0, 1).Render(l.Level)
			content.WriteString(fmt.Sprintf(" %s %s %s\n", ts, badge, truncateString(l.Message, width-26)))
		}
	}

	return style.Render(content.String())
}

func (m Model) renderFileDialogBox(width, height int) string {
	boxWidth := 60
	if boxWidth > width-6 {
		boxWidth = width - 6
	}

	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorBgDark).
		Background(ColorSecondary).
		Padding(0, 1).
		Render("SEND FILE TO PEER")

	var targetInfo string
	if len(m.Peers) > 0 && m.SelectedPeerIdx < len(m.Peers) {
		p := m.Peers[m.SelectedPeerIdx]
		targetInfo = lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Bold(true).
			Render(fmt.Sprintf("Target: %s (%s)", p.Hostname, p.Addr()))
	} else {
		targetInfo = lipgloss.NewStyle().
			Foreground(ColorWarning).
			Render("Target: No peer selected (will scan network)")
	}

	hint := lipgloss.NewStyle().
		Foreground(ColorTextDim).
		Render("Type path, drag-and-drop from Finder, or press [Ctrl+O] to browse:")

	footer := lipgloss.NewStyle().
		Foreground(ColorMuted).
		Render("Press [Enter] Send  •  [Ctrl+O] Browse Finder  •  [Esc] Cancel")

	box := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		"",
		targetInfo,
		"",
		hint,
		"",
		m.FileInput.View(),
		"",
		footer,
	)

	return lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(ColorSecondary).
		Background(ColorBgPanel).
		Padding(1, 2).
		Width(boxWidth).
		Render(box)
}

func (m Model) renderIncomingModalBox(width, height int) string {
	boxWidth := 60
	if boxWidth > width-6 {
		boxWidth = width - 6
	}

	req := m.IncomingModal.Request

	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorBgDark).
		Background(ColorWarning).
		Padding(0, 1).
		Render("INCOMING FILE TRANSFER REQUEST")

	fromLine := fmt.Sprintf("From:     %s", lipgloss.NewStyle().Bold(true).Foreground(ColorSecondary).Render(m.IncomingModal.SenderAddr.String()))
	fileLine := fmt.Sprintf("File:     %s", lipgloss.NewStyle().Bold(true).Foreground(ColorTextWhite).Render(req.Filename))
	sizeLine := fmt.Sprintf("Size:     %s (%d bytes)", cli.FormatBytes(req.FileSize), req.FileSize)
	hashLine := fmt.Sprintf("Checksum: 0x%08x", req.Checksum)

	buttons := lipgloss.JoinHorizontal(
		lipgloss.Center,
		lipgloss.NewStyle().Bold(true).Foreground(ColorBgDark).Background(ColorSuccess).Padding(0, 2).MarginRight(2).Render("[Y] Accept"),
		lipgloss.NewStyle().Bold(true).Foreground(ColorTextWhite).Background(ColorDanger).Padding(0, 2).Render("[N] Decline"),
	)

	box := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		"",
		fromLine,
		fileLine,
		sizeLine,
		hashLine,
		"",
		buttons,
	)

	return lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(ColorWarning).
		Background(ColorBgPanel).
		Padding(1, 2).
		Width(boxWidth).
		Render(box)
}

func (m Model) renderHelpBox(width, height int) string {
	boxWidth := 68
	if boxWidth > width-6 {
		boxWidth = width - 6
	}

	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorBgDark).
		Background(ColorPrimary).
		Padding(0, 1).
		Render("FLING P2P SHORTCUTS & HELP")

	table := []struct {
		Key  string
		Desc string
	}{
		{"Tab / Shift+Tab", "Switch active focus between Panes"},
		{"1, 2, 3, 4", "Jump directly to Pane (1-4)"},
		{"Up / Down", "Navigate through Discovered Peers"},
		{"Enter / c / m", "Open live chat with selected peer"},
		{"F / s", "Send file modal (Ctrl+O to browse Finder)"},
		{"o / O", "Reveal received file / folder in Finder"},
		{"r", "Scan / refresh LAN peer discovery"},
		{"y / n", "Accept / Decline incoming transfer"},
		{"Esc", "Close modal / Back to peers"},
		{"?", "Toggle this help menu"},
		{"q / Ctrl+C", "Quit Fling"},
	}

	var rows strings.Builder
	for _, r := range table {
		keyPill := lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorSecondary).
			Background(ColorSubtle).
			Padding(0, 1).
			Width(17).
			Render(r.Key)
		desc := lipgloss.NewStyle().
			Foreground(ColorTextLight).
			Render(r.Desc)
		rows.WriteString(fmt.Sprintf(" %s  %s\n", keyPill, desc))
	}

	footer := lipgloss.NewStyle().
		Foreground(ColorMuted).
		Italic(true).
		Render("Press [?] or [Esc] to close")

	box := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		"",
		rows.String(),
		footer,
	)

	return lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(ColorPrimary).
		Background(ColorBgPanel).
		Padding(1, 2).
		Width(boxWidth).
		Render(box)
}

func (m Model) renderFooter(width int) string {
	keys := []struct {
		Key  string
		Desc string
	}{
		{"Tab", "Next"},
		{"1-4", "Jump"},
		{"Up/Dn", "Select"},
		{"F", "Send File"},
		{"Enter", "Chat"},
		{"o", "Finder"},
		{"r", "Scan"},
		{"?", "Help"},
		{"q", "Quit"},
	}

	var parts []string
	for _, k := range keys {
		keyBadge := lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorBgDark).
			Background(ColorSecondary).
			Padding(0, 1).
			Render(k.Key)
		descText := lipgloss.NewStyle().
			Foreground(ColorTextDim).
			PaddingLeft(1).
			Render(k.Desc)
		parts = append(parts, fmt.Sprintf("%s%s", keyBadge, descText))
	}

	return lipgloss.JoinHorizontal(lipgloss.Center, strings.Join(parts, "   "))
}

// overlayModal overlays a rendered modal box in the center on top of the running dashboard background
func overlayModal(background string, modal string, width, height int) string {
	bgLines := strings.Split(background, "\n")
	modalLines := strings.Split(modal, "\n")

	if len(bgLines) == 0 || len(modalLines) == 0 {
		return background
	}

	// Ensure background lines match height
	for len(bgLines) < height {
		bgLines = append(bgLines, strings.Repeat(" ", width))
	}
	if len(bgLines) > height {
		bgLines = bgLines[:height]
	}

	modalHeight := len(modalLines)
	modalWidth := 0
	for _, l := range modalLines {
		w := lipgloss.Width(l)
		if w > modalWidth {
			modalWidth = w
		}
	}

	startY := (len(bgLines) - modalHeight) / 2
	if startY < 1 {
		startY = 1
	}

	startX := (width - modalWidth) / 2
	if startX < 0 {
		startX = 0
	}

	var result strings.Builder
	for y, bgLine := range bgLines {
		if y >= startY && y < startY+modalHeight {
			row := y - startY
			var mLine string
			if row < len(modalLines) {
				mLine = modalLines[row]
			}
			// Pad modal line to full modal width
			mVisWidth := lipgloss.Width(mLine)
			if mVisWidth < modalWidth {
				mLine = mLine + strings.Repeat(" ", modalWidth-mVisWidth)
			}

			leftPart, rightPart := cutANSILine(bgLine, startX, startX+modalWidth)
			result.WriteString(leftPart)
			result.WriteString(mLine)
			result.WriteString("\x1b[0m")
			result.WriteString(rightPart)
		} else {
			result.WriteString(bgLine)
		}
		if y < len(bgLines)-1 {
			result.WriteString("\n")
		}
	}

	return result.String()
}

// cutANSILine extracts the left and right segments of an ANSI-styled line around a center overlay range
func cutANSILine(line string, leftCut, rightCut int) (string, string) {
	var left strings.Builder
	var right strings.Builder

	curVisualCol := 0
	inEscape := false

	runes := []rune(line)
	for i := 0; i < len(runes); i++ {
		r := runes[i]

		if r == '\x1b' {
			inEscape = true
			if curVisualCol < leftCut {
				left.WriteRune(r)
			} else if curVisualCol >= rightCut {
				right.WriteRune(r)
			}
			continue
		}

		if inEscape {
			if curVisualCol < leftCut {
				left.WriteRune(r)
			} else if curVisualCol >= rightCut {
				right.WriteRune(r)
			}
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEscape = false
			}
			continue
		}

		// Printable rune
		if curVisualCol < leftCut {
			left.WriteRune(r)
		} else if curVisualCol >= rightCut {
			right.WriteRune(r)
		}

		curVisualCol++
	}

	leftVis := lipgloss.Width(left.String())
	if leftVis < leftCut {
		left.WriteString(strings.Repeat(" ", leftCut-leftVis))
	}

	return left.String(), right.String()
}

func truncateString(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max < 4 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

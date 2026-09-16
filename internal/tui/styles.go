package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// Catppuccin Mocha & Tokyo Night Curated Palette
var (
	ColorPrimary   = lipgloss.Color("#BD93F9") // Vibrant Mauve Violet
	ColorSecondary = lipgloss.Color("#00F5D4") // Cyber Mint Cyan
	ColorAccent    = lipgloss.Color("#8BE9FD") // Electric Sky Blue
	ColorSuccess   = lipgloss.Color("#50FA7B") // Neon Emerald
	ColorWarning   = lipgloss.Color("#FFB86C") // Warm Sunset Orange / Amber
	ColorDanger    = lipgloss.Color("#FF5555") // Coral Red
	ColorMuted     = lipgloss.Color("#6272A4") // Slate Lavender
	ColorSubtle    = lipgloss.Color("#2E3046") // Surface Dark Grey
	ColorBgDark    = lipgloss.Color("#0D0E15") // Deep Obsidian Background
	ColorBgPanel   = lipgloss.Color("#161622") // Card Surface Background
	ColorBgInput   = lipgloss.Color("#1E1F2E") // Input Field Background
	ColorTextLight = lipgloss.Color("#F8F8F2") // Crisp White
	ColorTextDim   = lipgloss.Color("#A0A5BD") // Dimmed Text
	ColorTextWhite = lipgloss.Color("#FFFFFF") // Absolute White
)

// UI Component Styles
var (
	// Header & Status Badges
	StyleAppTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorBgDark).
			Background(ColorSecondary).
			Padding(0, 1).
			MarginRight(1)

	StyleVersionBadge = lipgloss.NewStyle().
				Foreground(ColorPrimary).
				Background(ColorBgPanel).
				Padding(0, 1).
				Bold(true)

	StyleStatusOnline = lipgloss.NewStyle().
				Foreground(ColorSuccess).
				Bold(true)

	StyleStatusPill = lipgloss.NewStyle().
			Foreground(ColorTextLight).
			Background(ColorSubtle).
			Padding(0, 1).
			MarginLeft(1)

	StyleAccentPill = lipgloss.NewStyle().
			Foreground(ColorBgDark).
			Background(ColorPrimary).
			Bold(true).
			Padding(0, 1).
			MarginLeft(1)

	// Pane Title Header Styles
	StylePaneTitleActive = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorBgDark).
				Background(ColorPrimary).
				Padding(0, 1)

	StylePaneTitleInactive = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorTextDim).
				Background(ColorSubtle).
				Padding(0, 1)

	// Peer Table Styles
	StyleTableHeader = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorSecondary).
				Background(ColorBgPanel).
				Padding(0, 1)

	StyleTableRow = lipgloss.NewStyle().
			Foreground(ColorTextLight).
			Padding(0, 1)

	StyleTableRowSelected = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorTextWhite).
				Background(lipgloss.Color("#282A36")).
				Padding(0, 1)

	// Chat Messages
	StyleChatTimestamp = lipgloss.NewStyle().
				Foreground(ColorMuted)

	StyleChatSenderMe = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorSecondary)

	StyleChatSenderPeer = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorPrimary)

	StyleChatMessageText = lipgloss.NewStyle().
				Foreground(ColorTextLight)

	// Modals & Dialogs
	StyleModalBox = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(ColorPrimary).
			Padding(1, 2).
			Background(ColorBgPanel)

	StyleModalTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary)

	StyleSubPanel = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorSubtle).
			Padding(0, 1).
			Background(ColorBgPanel)

	StyleBtnAccept = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorBgDark).
			Background(ColorSuccess).
			Padding(0, 2).
			MarginRight(2)

	StyleBtnReject = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorTextWhite).
			Background(ColorDanger).
			Padding(0, 2)

	// Footer Keycaps
	StyleFooterKey = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorBgDark).
			Background(ColorSecondary).
			Padding(0, 1)

	StyleFooterDesc = lipgloss.NewStyle().
			Foreground(ColorTextDim).
			PaddingLeft(1).
			PaddingRight(2)
)

// MakePane creates a styled border container for a dashboard pane
func MakePane(focused bool, width, height int) lipgloss.Style {
	borderColor := ColorSubtle
	if focused {
		borderColor = ColorPrimary
	}
	w := width - 2
	if w < 10 {
		w = 10
	}
	h := height - 2
	if h < 3 {
		h = 3
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(w).
		Height(h).
		MaxWidth(width).
		MaxHeight(height).
		Background(ColorBgDark)
}

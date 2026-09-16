package cli

import (
	"fmt"
	"strings"
)

// ProgressFunc is a callback function to report transfer progress.
type ProgressFunc func(transferred int64, total int64)

// ProgressBar manages rendering a terminal progress bar for transfers.
type ProgressBar struct {
	TotalBytes       int64
	TransferredBytes int64
	Filename         string
	Width            int
}

// NewProgressBar creates a new ProgressBar instance.
func NewProgressBar(totalBytes int64, filename string) *ProgressBar {
	return &ProgressBar{
		TotalBytes: totalBytes,
		Filename:   filename,
		Width:      25,
	}
}

// Update updates the transferred byte count and renders the progress bar to the terminal.
func (p *ProgressBar) Update(transferred int64) {
	p.TransferredBytes = transferred
	p.Print()
}

// Format returns the formatted progress bar string.
func (p *ProgressBar) Format() string {
	var percent float64
	if p.TotalBytes > 0 {
		percent = float64(p.TransferredBytes) / float64(p.TotalBytes) * 100.0
		if percent > 100.0 {
			percent = 100.0
		}
	} else if p.TransferredBytes == 0 {
		percent = 100.0
	}

	completedWidth := int(float64(p.Width) * (percent / 100.0))
	if completedWidth > p.Width {
		completedWidth = p.Width
	}

	var bar strings.Builder
	bar.WriteString("[")
	for i := 0; i < completedWidth; i++ {
		if i == completedWidth-1 && completedWidth < p.Width {
			bar.WriteString(">")
		} else {
			bar.WriteString("=")
		}
	}
	for i := completedWidth; i < p.Width; i++ {
		bar.WriteString(" ")
	}
	bar.WriteString("]")

	filename := p.Filename
	if filename != "" {
		filename = filename + ": "
	}

	return fmt.Sprintf(
		"%s%s %d/%d bytes (%.1f%%)",
		filename,
		bar.String(),
		p.TransferredBytes,
		p.TotalBytes,
		percent,
	)
}

// Print writes the progress bar line to stdout using a carriage return.
func (p *ProgressBar) Print() {
	fmt.Printf("\r%s", p.Format())
}

// Finish prints the completed progress line and a final newline.
func (p *ProgressBar) Finish() {
	p.TransferredBytes = p.TotalBytes
	fmt.Printf("\r%s\n", p.Format())
}

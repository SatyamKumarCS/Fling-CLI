package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// FormatBytes formats byte sizes into a human-readable string (B, KB, MB, GB).
func FormatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// PromptAcceptReject prompts the user on stdin whether to accept or reject an incoming file.
func PromptAcceptReject(sender string, filename string, fileSize int64) (bool, error) {
	return PromptAcceptRejectWithReader(os.Stdin, sender, filename, fileSize)
}

// PromptAcceptRejectWithReader allows prompting with a custom reader (useful for tests).
func PromptAcceptRejectWithReader(r io.Reader, sender string, filename string, fileSize int64) (bool, error) {
	fmt.Println()
	fmt.Println("==================================================")
	fmt.Printf(" [!] INCOMING FILE TRANSFER REQUEST\n")
	fmt.Printf(" From:     %s\n", sender)
	fmt.Printf(" File:     %s\n", filename)
	fmt.Printf(" Size:     %s (%d bytes)\n", FormatBytes(fileSize), fileSize)
	fmt.Println("==================================================")
	fmt.Print(" Accept file transfer? [y/N]: ")

	scanner := bufio.NewScanner(r)
	if scanner.Scan() {
		input := strings.TrimSpace(strings.ToLower(scanner.Text()))
		if input == "y" || input == "yes" {
			return true, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return false, err
	}
	return false, nil
}

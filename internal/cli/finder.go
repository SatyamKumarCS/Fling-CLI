package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// RevealInFileManager opens the native operating system file manager (Finder on macOS,
// File Explorer on Windows, xdg-open on Linux) and selects the given file or opens the directory.
func RevealInFileManager(targetPath string) error {
	targetPath = stringsTrim(targetPath)
	if targetPath == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		targetPath = cwd
	}

	absPath, err := filepath.Abs(targetPath)
	if err != nil {
		absPath = targetPath
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		if fi, err := os.Stat(absPath); err == nil && !fi.IsDir() {
			// Select and highlight the specific file in Finder
			cmd = exec.Command("open", "-R", absPath)
		} else {
			// Open the folder
			dir := absPath
			if fi, err := os.Stat(absPath); err == nil && !fi.IsDir() {
				dir = filepath.Dir(absPath)
			}
			cmd = exec.Command("open", dir)
		}
	case "windows":
		if fi, err := os.Stat(absPath); err == nil && !fi.IsDir() {
			cmd = exec.Command("explorer", "/select,", absPath)
		} else {
			cmd = exec.Command("explorer", absPath)
		}
	default: // Linux / BSD
		dir := absPath
		if fi, err := os.Stat(absPath); err == nil && !fi.IsDir() {
			dir = filepath.Dir(absPath)
		}
		cmd = exec.Command("xdg-open", dir)
	}

	if cmd == nil {
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	return cmd.Start()
}

func stringsTrim(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' || s[0] == '\n' || s[0] == '\r' || s[0] == '"' || s[0] == '\'') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t' || s[len(s)-1] == '\n' || s[len(s)-1] == '\r' || s[len(s)-1] == '"' || s[len(s)-1] == '\'') {
		s = s[:len(s)-1]
	}
	return s
}

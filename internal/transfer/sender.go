package transfer

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/SatyamKumarCS/Fling-CLI/internal/cli"
	"github.com/SatyamKumarCS/Fling-CLI/internal/network"
	"github.com/SatyamKumarCS/Fling-CLI/internal/protocol"
	"github.com/SatyamKumarCS/Fling-CLI/internal/security"
)

// SendFile reads the specified file, splits it into chunks, reliably transmits each FileChunk packet,
// reports transfer progress, and sends a FileEnd packet upon completion.
// Returns the next available sequence number.
func SendFile(
	conn *net.UDPConn,
	addr *net.UDPAddr,
	filePath string,
	startSequence uint32,
	progress cli.ProgressFunc,
) (uint32, error) {
	return SendFileWithKey(conn, addr, filePath, nil, startSequence, progress)
}

// SendFileWithKey reads the specified file, splits it into chunks, reliably transmits each FileChunk packet,
// optionally encrypting the entire payload with key, reports transfer progress, and sends a FileEnd packet upon completion.
func SendFileWithKey(
	conn *net.UDPConn,
	addr *net.UDPAddr,
	filePath string,
	key []byte,
	startSequence uint32,
	progress cli.ProgressFunc,
) (uint32, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return startSequence, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	return SendBytesWithKey(conn, addr, data, key, startSequence, progress)
}

// SendBytes sends a byte slice as chunks reliably over UDP to the specified destination address.
func SendBytes(
	conn *net.UDPConn,
	addr *net.UDPAddr,
	data []byte,
	startSequence uint32,
	progress cli.ProgressFunc,
) (uint32, error) {
	return SendBytesWithKey(conn, addr, data, nil, startSequence, progress)
}

// SendBytesWithKey sends a byte slice as chunks reliably over UDP, optionally encrypted with key.
func SendBytesWithKey(
	conn *net.UDPConn,
	addr *net.UDPAddr,
	data []byte,
	key []byte,
	startSequence uint32,
	progress cli.ProgressFunc,
) (uint32, error) {
	if len(key) == 32 {
		encrypted, err := security.Encrypt(key, data)
		if err != nil {
			return startSequence, fmt.Errorf("file encryption failed: %w", err)
		}
		data = encrypted
	}

	totalBytes := int64(len(data))
	chunks := ChunkBytes(data, startSequence)

	var transferred int64
	currentSeq := startSequence

	for _, chunk := range chunks {
		err := network.SendReliable(conn, chunk, addr)
		if err != nil {
			return currentSeq, fmt.Errorf("failed to send chunk #%d: %w", chunk.SequenceNumber, err)
		}

		transferred += int64(len(chunk.Payload))
		if progress != nil {
			progress(transferred, totalBytes)
		}

		currentSeq++
	}

	// Send FileEnd packet to signal transfer completion
	fileEndPacket := protocol.Packet{
		SequenceNumber: currentSeq,
		Type:           protocol.FileEnd,
		Payload:        nil,
	}

	err := network.SendReliable(conn, fileEndPacket, addr)
	if err != nil {
		return currentSeq, fmt.Errorf("failed to send FileEnd packet #%d: %w", currentSeq, err)
	}

	currentSeq++

	return currentSeq, nil
}

// SendFileWithRouter sends a file using the network.Router.
func SendFileWithRouter(
	router *network.Router,
	addr *net.UDPAddr,
	filePath string,
	startSequence uint32,
	progress cli.ProgressFunc,
) (uint32, error) {
	return SendFileWithRouterAndKey(router, addr, filePath, nil, startSequence, progress)
}

// SendFileWithRouterAndKey sends a file (optionally encrypted with key) using the network.Router.
func SendFileWithRouterAndKey(
	router *network.Router,
	addr *net.UDPAddr,
	filePath string,
	key []byte,
	startSequence uint32,
	progress cli.ProgressFunc,
) (uint32, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return startSequence, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	return SendBytesWithRouterAndKey(router, addr, data, key, startSequence, progress)
}

// SendBytesWithRouter sends a byte slice using the network.Router.
func SendBytesWithRouter(
	router *network.Router,
	addr *net.UDPAddr,
	data []byte,
	startSequence uint32,
	progress cli.ProgressFunc,
) (uint32, error) {
	return SendBytesWithRouterAndKey(router, addr, data, nil, startSequence, progress)
}

// SendBytesWithRouterAndKey sends a byte slice (optionally encrypted with key) using the network.Router.
func SendBytesWithRouterAndKey(
	router *network.Router,
	addr *net.UDPAddr,
	data []byte,
	key []byte,
	startSequence uint32,
	progress cli.ProgressFunc,
) (uint32, error) {
	if len(key) == 32 {
		encrypted, err := security.Encrypt(key, data)
		if err != nil {
			return startSequence, fmt.Errorf("file encryption failed: %w", err)
		}
		data = encrypted
	}

	totalBytes := int64(len(data))
	chunks := ChunkBytes(data, startSequence)

	var transferred int64
	currentSeq := startSequence

	for _, chunk := range chunks {
		err := router.SendReliable(chunk, addr)
		if err != nil {
			return currentSeq, fmt.Errorf("failed to send chunk #%d: %w", chunk.SequenceNumber, err)
		}

		transferred += int64(len(chunk.Payload))
		if progress != nil {
			progress(transferred, totalBytes)
		}

		currentSeq++
	}

	fileEndPacket := protocol.Packet{
		SequenceNumber: currentSeq,
		Type:           protocol.FileEnd,
		Payload:        nil,
	}

	err := router.SendReliable(fileEndPacket, addr)
	if err != nil {
		return currentSeq, fmt.Errorf("failed to send FileEnd packet #%d: %w", currentSeq, err)
	}

	currentSeq++
	return currentSeq, nil
}

// ResolveFilePath resolves a user-provided file path, handling tilde expansion,
// quotes, drag-and-drop escaped spaces, home directory prefixes, case-insensitivity,
// and common directory variations (e.g. test vs tests, desktop vs Desktop).
func ResolveFilePath(raw string) (string, os.FileInfo, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.Trim(raw, `"'`)
	raw = strings.ReplaceAll(raw, `\ `, " ")
	if raw == "" {
		return "", nil, fmt.Errorf("empty file path")
	}

	// 1. Tilde expansion (~/Desktop/... or ~)
	if strings.HasPrefix(raw, "~/") || raw == "~" {
		home, err := os.UserHomeDir()
		if err == nil {
			if raw == "~" {
				raw = home
			} else {
				raw = filepath.Join(home, raw[2:])
			}
		}
	}

	// 2. Direct check
	if info, err := os.Stat(raw); err == nil {
		return raw, info, nil
	}

	// 3. Check relative to current working directory
	if cwd, err := os.Getwd(); err == nil {
		cwdRel := filepath.Join(cwd, raw)
		if info, err := os.Stat(cwdRel); err == nil {
			return cwdRel, info, nil
		}
	}

	// 4. Check relative to user home directory
	if home, err := os.UserHomeDir(); err == nil {
		homeRel := filepath.Join(home, raw)
		if info, err := os.Stat(homeRel); err == nil {
			return homeRel, info, nil
		}
	}

	// 5. Split path and perform case-insensitive and variation fuzzy matching
	cleanPath := filepath.Clean(raw)
	parts := strings.Split(cleanPath, string(filepath.Separator))
	// Remove empty leading part if path was absolute
	var relParts []string
	for _, p := range parts {
		if p != "" && p != "." {
			relParts = append(relParts, p)
		}
	}

	if len(relParts) > 0 {
		var bases []string
		if filepath.IsAbs(raw) {
			bases = []string{string(filepath.Separator)}
		} else {
			if cwd, err := os.Getwd(); err == nil {
				bases = append(bases, cwd)
				bases = append(bases, filepath.Dir(cwd))
				bases = append(bases, filepath.Dir(filepath.Dir(cwd)))
			}
			if home, err := os.UserHomeDir(); err == nil {
				bases = append(bases, home)
				bases = append(bases, filepath.Join(home, "Desktop"))
				bases = append(bases, filepath.Join(home, "Downloads"))
				bases = append(bases, filepath.Join(home, "Documents"))
			}
			bases = append(bases, ".")
		}

		for _, base := range bases {
			if resolved, info, err := matchFuzzySegments(base, relParts); err == nil && !info.IsDir() {
				return resolved, info, nil
			}
		}
	}

	// 6. Search for filename in common subfolders (tests/, testdata/, etc.)
	baseName := filepath.Base(raw)
	if cwd, err := os.Getwd(); err == nil {
		searchDirs := []string{
			cwd,
			filepath.Dir(cwd),
		}
		commonSubdirs := []string{"tests", "testdata", "test", "docs"}
		for _, sDir := range searchDirs {
			for _, sub := range commonSubdirs {
				candidate := filepath.Join(sDir, sub, baseName)
				if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
					return candidate, info, nil
				}
			}
		}
	}

	return "", nil, fmt.Errorf("file not found: %s", raw)
}

// matchFuzzySegments recursively resolves directory segments case-insensitively with common typo tolerances.
func matchFuzzySegments(current string, segments []string) (string, os.FileInfo, error) {
	if len(segments) == 0 {
		info, err := os.Stat(current)
		if err != nil {
			return "", nil, err
		}
		return current, info, nil
	}

	target := segments[0]
	entries, err := os.ReadDir(current)
	if err != nil {
		return "", nil, err
	}

	// First pass: exact match (fast path)
	for _, entry := range entries {
		if entry.Name() == target {
			next := filepath.Join(current, entry.Name())
			return matchFuzzySegments(next, segments[1:])
		}
	}

	// Second pass: case-insensitive match
	for _, entry := range entries {
		if strings.EqualFold(entry.Name(), target) {
			next := filepath.Join(current, entry.Name())
			return matchFuzzySegments(next, segments[1:])
		}
	}

	// Third pass: variation matches (e.g. test vs tests, download vs downloads)
	for _, entry := range entries {
		eName := entry.Name()
		if isVariationMatch(eName, target) {
			next := filepath.Join(current, eName)
			return matchFuzzySegments(next, segments[1:])
		}
	}

	return "", nil, fmt.Errorf("segment %q not matched in %s", target, current)
}

func isVariationMatch(entryName, target string) bool {
	eLow := strings.ToLower(entryName)
	tLow := strings.ToLower(target)
	if eLow == tLow {
		return true
	}
	// Plural vs singular
	if eLow == tLow+"s" || eLow+"s" == tLow {
		return true
	}
	// test vs testdata
	if (eLow == "testdata" && tLow == "test") || (eLow == "test" && tLow == "testdata") {
		return true
	}
	return false
}



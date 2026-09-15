package protocol

import "hash/crc32"

func calculateChecksum(data []byte) uint32 {
	return crc32.ChecksumIEEE(data)
}

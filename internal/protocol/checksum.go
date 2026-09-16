package protocol

import "hash/crc32"

func calculateChecksum(data []byte) uint32 {
	return crc32.ChecksumIEEE(data)
}

// CalculateChecksum calculates the CRC32 IEEE checksum of the provided byte slice.
func CalculateChecksum(data []byte) uint32 {
	return crc32.ChecksumIEEE(data)
}

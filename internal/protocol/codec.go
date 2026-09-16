package protocol

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

func Encode(packet Packet) ([]byte, error) {
	payloadLength := len(packet.Payload)

	if payloadLength > 65535 {
		return nil, fmt.Errorf("payload too large: %d bytes", payloadLength)
	}

	headerWithoutChecksum := bytes.NewBuffer(nil)

	binary.Write(
		headerWithoutChecksum,
		binary.BigEndian,
		packet.SequenceNumber,
	)

	binary.Write(
		headerWithoutChecksum,
		binary.BigEndian,
		packet.Type,
	)

	binary.Write(
		headerWithoutChecksum,
		binary.BigEndian,
		uint16(payloadLength),
	)

	dataToChecksum := append(
		headerWithoutChecksum.Bytes(),
		packet.Payload...,
	)

	checksum := calculateChecksum(dataToChecksum)

	packetBytes := bytes.NewBuffer(nil)

	packetBytes.Write(headerWithoutChecksum.Bytes())

	binary.Write(
		packetBytes,
		binary.BigEndian,
		checksum,
	)

	packetBytes.Write(packet.Payload)

	return packetBytes.Bytes(), nil
}

func Decode(data []byte) (Packet, error) {
	const headerSize = 11

	if len(data) < headerSize {
		return Packet{}, fmt.Errorf("packet too small")
	}

	reader := bytes.NewReader(data)

	var sequenceNumber uint32
	var packetType PacketType
	var payloadLength uint16
	var receivedChecksum uint32

	if err := binary.Read(reader, binary.BigEndian, &sequenceNumber); err != nil {
		return Packet{}, err
	}

	if err := binary.Read(reader, binary.BigEndian, &packetType); err != nil {
		return Packet{}, err
	}

	if err := binary.Read(reader, binary.BigEndian, &payloadLength); err != nil {
		return Packet{}, err
	}

	if err := binary.Read(reader, binary.BigEndian, &receivedChecksum); err != nil {
		return Packet{}, err
	}

	expectedPacketSize := headerSize + int(payloadLength)

	if len(data) != expectedPacketSize {
		return Packet{}, fmt.Errorf(
			"invalid packet size: expected %d, got %d",
			expectedPacketSize,
			len(data),
		)
	}

	payload := make([]byte, payloadLength)

	if payloadLength > 0 {
		if _, err := reader.Read(payload); err != nil {
			return Packet{}, err
		}
	}

	// Recreate the data that was originally checksummed.
	checksumData := bytes.NewBuffer(nil)

	binary.Write(
		checksumData,
		binary.BigEndian,
		sequenceNumber,
	)

	binary.Write(
		checksumData,
		binary.BigEndian,
		packetType,
	)

	binary.Write(
		checksumData,
		binary.BigEndian,
		payloadLength,
	)

	checksumData.Write(payload)

	expectedChecksum := calculateChecksum(checksumData.Bytes())

	if receivedChecksum != expectedChecksum {
		return Packet{}, fmt.Errorf(
			"checksum mismatch: expected %08x, got %08x",
			expectedChecksum,
			receivedChecksum,
		)
	}

	return Packet{
		SequenceNumber: sequenceNumber,
		Type:           packetType,
		Payload:        payload,
	}, nil
}

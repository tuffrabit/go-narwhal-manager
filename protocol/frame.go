package protocol

import (
	"encoding/binary"
	"fmt"
)

// Frame represents a protocol frame
type Frame struct {
	Cmd     byte
	Payload []byte
}

// BuildFrame creates a complete frame with sync byte, header, payload, and CRC.
// Format: [SYNC:1][CMD:1][LEN:2][PAYLOAD:LEN][CRC:2]
func BuildFrame(cmd byte, payload []byte) []byte {
	length := len(payload)
	frame := make([]byte, MinFrameSize+length)

	// Sync byte
	frame[0] = SyncByte

	// Command
	frame[1] = cmd

	// Length (little-endian uint16)
	binary.LittleEndian.PutUint16(frame[2:4], uint16(length))

	// Payload
	copy(frame[4:4+length], payload)

	// CRC over [CMD][LEN][PAYLOAD]
	crcData := frame[1 : 4+length]
	crc := CRC16CCITT(crcData)
	binary.LittleEndian.PutUint16(frame[4+length:6+length], crc)

	return frame
}

// ParseFrame parses a received frame and extracts command/status and payload.
// Returns the status/command byte and payload (CRC is validated but not returned).
// Format: [SYNC:1][STATUS:1][LEN:2][PAYLOAD:LEN][CRC:2]
func ParseFrame(data []byte) (status byte, payload []byte, err error) {
	if len(data) < MinFrameSize {
		return 0, nil, fmt.Errorf("%w: got %d bytes, need at least %d", ErrFrameTooShort, len(data), MinFrameSize)
	}

	// Check sync byte
	if data[0] != SyncByte {
		return 0, nil, fmt.Errorf("%w: expected 0x%02X, got 0x%02X", ErrInvalidSync, SyncByte, data[0])
	}

	// Extract status/command
	status = data[1]

	// Extract length
	length := binary.LittleEndian.Uint16(data[2:4])

	// Validate total frame size
	expectedSize := MinFrameSize + int(length)
	if len(data) < expectedSize {
		return 0, nil, fmt.Errorf("%w: got %d bytes, expected %d for payload length %d", ErrFrameTooShort, len(data), expectedSize, length)
	}

	// Extract payload (may be empty)
	if length > 0 {
		payload = make([]byte, length)
		copy(payload, data[4:4+length])
	}

	// Verify CRC
	crcData := data[1 : 4+length]
	receivedCRC := binary.LittleEndian.Uint16(data[4+length : 6+length])
	calculatedCRC := CRC16CCITT(crcData)

	if receivedCRC != calculatedCRC {
		return 0, nil, fmt.Errorf("%w: received 0x%04X, calculated 0x%04X", ErrCRCMismatch, receivedCRC, calculatedCRC)
	}

	return status, payload, nil
}

// BuildDiscoverFrame creates a discovery request frame.
// Request: AA 7F 00 00 [CRC:2]
func BuildDiscoverFrame() []byte {
	return BuildFrame(CmdDiscover, nil)
}

// ParseDiscoverResponse checks if a response is a valid discovery response.
// Expected: AA 00 07 00 74 75 66 66 70 61 64 [CRC:2] (payload = "tuffpad")
func ParseDiscoverResponse(data []byte) (bool, error) {
	status, payload, err := ParseFrame(data)
	if err != nil {
		return false, err
	}

	if status != StatusOK {
		return false, &DeviceError{
			Code:    status,
			Message: ErrorCodeToString(status),
		}
	}

	if string(payload) != "tuffpad" {
		return false, fmt.Errorf("%w: expected 'tuffpad', got '%s'", ErrInvalidResponse, string(payload))
	}

	return true, nil
}

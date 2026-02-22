// Package protocol implements the Tuffpad binary serial protocol.
//
// Protocol Format:
//
//	[SYNC:1][CMD:1][LEN:2][PAYLOAD:LEN][CRC:2]
//
//	- SYNC:    0xAA (frame start marker)
//	- CMD:     command code (1 byte)
//	- LEN:     payload length (uint16, little-endian)
//	- PAYLOAD: command-specific data
//	- CRC:     CRC16-CCITT of [CMD][LEN][PAYLOAD]
//
// CRC16-CCITT:
//	- Polynomial: 0x1021
//	- Initial value: 0xFFFF
package protocol

import (
	"errors"
	"fmt"
)

// Protocol constants
const (
	SyncByte = 0xAA

	// HeaderSize is the size of frame header [SYNC:1][CMD:1][LEN:2]
	HeaderSize = 4

	// CRCLen is the size of CRC field
	CRCLen = 2

	// MinFrameSize is the minimum valid frame size (header + CRC with no payload)
	MinFrameSize = HeaderSize + CRCLen
)

// Command codes (PC to Device)
const (
	CmdGetDeviceConfig byte = 0x01
	CmdSetDeviceConfig byte = 0x02
	CmdGetProfile      byte = 0x03
	CmdSetProfile      byte = 0x04
	CmdDeleteProfile   byte = 0x05
	CmdListProfiles    byte = 0x06
	CmdGetStorageStats byte = 0x07
	CmdPing            byte = 0x08
	CmdFactoryReset    byte = 0x09
	CmdGetVersion      byte = 0x10
	CmdDiscover        byte = 0x7F
)

// Status codes (Device to PC)
const (
	StatusOK              byte = 0x00
	StatusError           byte = 0x01
	StatusInvalidCmd      byte = 0x02
	StatusInvalidData     byte = 0x03
	StatusNotFound        byte = 0x04
	StatusNoSpace         byte = 0x05
	StatusVersionMismatch byte = 0x06
	StatusCRCError        byte = 0x07
)

// Protocol errors
var (
	ErrInvalidSync     = errors.New("invalid sync byte")
	ErrFrameTooShort   = errors.New("frame too short")
	ErrCRCMismatch     = errors.New("CRC mismatch")
	ErrInvalidResponse = errors.New("invalid response")
)

// DeviceError wraps a protocol status code with a descriptive message
type DeviceError struct {
	Code    byte
	Message string
}

func (e *DeviceError) Error() string {
	return fmt.Sprintf("device error 0x%02X: %s", e.Code, e.Message)
}

// ErrorCodeToString returns a human-readable description of a status code
func ErrorCodeToString(code byte) string {
	switch code {
	case StatusError:
		return "general error"
	case StatusInvalidCmd:
		return "invalid command"
	case StatusInvalidData:
		return "invalid data"
	case StatusNotFound:
		return "not found"
	case StatusNoSpace:
		return "insufficient space"
	case StatusVersionMismatch:
		return "version mismatch"
	case StatusCRCError:
		return "CRC error"
	default:
		return fmt.Sprintf("unknown error 0x%02X", code)
	}
}

// CRC16CCITT calculates the CRC16-CCITT checksum.
// Polynomial: 0x1021, Initial value: 0xFFFF
func CRC16CCITT(data []byte) uint16 {
	crc := uint16(0xFFFF)
	for _, b := range data {
		crc ^= uint16(b) << 8
		for i := 0; i < 8; i++ {
			if crc&0x8000 != 0 {
				crc = (crc << 1) ^ 0x1021
			} else {
				crc <<= 1
			}
			crc &= 0xFFFF
		}
	}
	return crc
}

// CheckStatus returns an error if the status code indicates failure
func CheckStatus(status byte) error {
	if status == StatusOK {
		return nil
	}
	return &DeviceError{
		Code:    status,
		Message: ErrorCodeToString(status),
	}
}

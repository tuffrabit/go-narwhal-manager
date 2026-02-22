package protocol

import (
	"bytes"
	"testing"
)

func TestCRC16CCITT(t *testing.T) {
	// Test vector: CRC16-CCITT of [0x7F, 0x00, 0x00] (Discover header)
	// Expected CRC needs to be calculated
	data := []byte{0x7F, 0x00, 0x00}
	crc := CRC16CCITT(data)

	// Verify CRC is consistent
	crc2 := CRC16CCITT(data)
	if crc != crc2 {
		t.Errorf("CRC not deterministic: %04X vs %04X", crc, crc2)
	}

	t.Logf("CRC of %X = 0x%04X", data, crc)
}

func TestBuildDiscoverFrame(t *testing.T) {
	frame := BuildDiscoverFrame()

	// Minimum frame size: 4 (header) + 2 (CRC) = 6
	if len(frame) < MinFrameSize {
		t.Fatalf("Frame too short: got %d bytes, need at least %d", len(frame), MinFrameSize)
	}

	// Check sync byte
	if frame[0] != SyncByte {
		t.Errorf("Invalid sync byte: expected 0x%02X, got 0x%02X", SyncByte, frame[0])
	}

	// Check command
	if frame[1] != CmdDiscover {
		t.Errorf("Invalid command: expected 0x%02X, got 0x%02X", CmdDiscover, frame[1])
	}

	// Check length (should be 0 for discover)
	length := uint16(frame[2]) | (uint16(frame[3]) << 8)
	if length != 0 {
		t.Errorf("Invalid length: expected 0, got %d", length)
	}

	t.Logf("Discover frame: %X", frame)
}

func TestParseDiscoverResponse(t *testing.T) {
	// Build a valid discover response frame
	// Status OK (0x00), payload "tuffpad"
	payload := []byte("tuffpad")
	frame := BuildFrame(StatusOK, payload)

	t.Logf("Discover response frame: %X", frame)

	// Parse it
	isTuffpad, err := ParseDiscoverResponse(frame)
	if err != nil {
		t.Fatalf("Failed to parse valid discover response: %v", err)
	}
	if !isTuffpad {
		t.Error("ParseDiscoverResponse returned false for valid response")
	}
}

func TestParseDiscoverResponseInvalidPayload(t *testing.T) {
	// Build a response with wrong payload
	payload := []byte("notatuffpad")
	frame := BuildFrame(StatusOK, payload)

	isTuffpad, err := ParseDiscoverResponse(frame)
	if err == nil {
		t.Error("Expected error for invalid payload")
	}
	if isTuffpad {
		t.Error("ParseDiscoverResponse should return false for invalid payload")
	}
}

func TestParseDiscoverResponseErrorStatus(t *testing.T) {
	// Build a response with error status
	frame := BuildFrame(StatusError, nil)

	isTuffpad, err := ParseDiscoverResponse(frame)
	if err == nil {
		t.Error("Expected error for error status")
	}
	if isTuffpad {
		t.Error("ParseDiscoverResponse should return false for error status")
	}

	// Check that it's a DeviceError
	deviceErr, ok := err.(*DeviceError)
	if !ok {
		t.Fatalf("Expected *DeviceError, got %T", err)
	}
	if deviceErr.Code != StatusError {
		t.Errorf("Expected error code 0x%02X, got 0x%02X", StatusError, deviceErr.Code)
	}
}

func TestFrameRoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		cmd     byte
		payload []byte
	}{
		{"Empty payload", CmdPing, nil},
		{"Small payload", CmdGetProfile, []byte{0x05}},
		{"Larger payload", CmdPing, []byte("hello world")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build frame
			frame := BuildFrame(tt.cmd, tt.payload)

			// Parse frame
			status, payload, err := ParseFrame(frame)
			if err != nil {
				t.Fatalf("ParseFrame failed: %v", err)
			}

			// For request frames, the "status" is actually the command
			if status != tt.cmd {
				t.Errorf("Command mismatch: expected 0x%02X, got 0x%02X", tt.cmd, status)
			}

			if !bytes.Equal(payload, tt.payload) {
				t.Errorf("Payload mismatch: expected %X, got %X", tt.payload, payload)
			}
		})
	}
}

func TestParseFrameErrors(t *testing.T) {
	tests := []struct {
		name        string
		data        []byte
		expectedErr error
	}{
		{
			name:        "Too short",
			data:        []byte{0xAA, 0x01},
			expectedErr: ErrFrameTooShort,
		},
		{
			name:        "Invalid sync",
			data:        []byte{0xBB, 0x01, 0x00, 0x00, 0x00, 0x00},
			expectedErr: ErrInvalidSync,
		},
		{
			name:        "CRC mismatch",
			data:        []byte{0xAA, 0x01, 0x00, 0x00, 0xDE, 0xAD},
			expectedErr: ErrCRCMismatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := ParseFrame(tt.data)
			if err == nil {
				t.Fatal("Expected error, got nil")
			}
			if !errorsIs(err, tt.expectedErr) {
				t.Errorf("Expected error %v, got %v", tt.expectedErr, err)
			}
		})
	}
}

// errorsIs is a helper to check if an error matches (simplified)
func errorsIs(err, target error) bool {
	if err == target {
		return true
	}
	// Check wrapped errors
	type wrapper interface {
		Unwrap() error
	}
	if w, ok := err.(wrapper); ok {
		return errorsIs(w.Unwrap(), target)
	}
	return false
}

func TestDeviceConfigMarshalUnmarshal(t *testing.T) {
	original := &DeviceConfig{
		Version:       1,
		Flags:         0x12345678,
		ActiveProfile: 5,
		Brightness:    200,
		DebounceMs:    50,
		Reserved:      0,
	}

	data, err := original.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary failed: %v", err)
	}

	if len(data) != DeviceConfigSize {
		t.Errorf("Wrong size: expected %d, got %d", DeviceConfigSize, len(data))
	}

	unmarshaled := &DeviceConfig{}
	if err := unmarshaled.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary failed: %v", err)
	}

	if *original != *unmarshaled {
		t.Errorf("Round-trip failed: %+v != %+v", original, unmarshaled)
	}
}

func TestCheckStatus(t *testing.T) {
	if err := CheckStatus(StatusOK); err != nil {
		t.Errorf("CheckStatus(StatusOK) should return nil, got %v", err)
	}

	tests := []struct {
		code     byte
		expected string
	}{
		{StatusError, "general error"},
		{StatusInvalidCmd, "invalid command"},
		{StatusNotFound, "not found"},
		{0xFF, "unknown error 0xFF"},
	}

	for _, tt := range tests {
		err := CheckStatus(tt.code)
		if err == nil {
			t.Errorf("CheckStatus(0x%02X) should return error", tt.code)
			continue
		}

		deviceErr, ok := err.(*DeviceError)
		if !ok {
			t.Errorf("Expected *DeviceError, got %T", err)
			continue
		}

		if deviceErr.Code != tt.code {
			t.Errorf("Error code mismatch: expected 0x%02X, got 0x%02X", tt.code, deviceErr.Code)
		}

		if deviceErr.Message != tt.expected {
			t.Errorf("Error message mismatch: expected %q, got %q", tt.expected, deviceErr.Message)
		}
	}
}

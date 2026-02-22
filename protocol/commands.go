package protocol

import (
	"encoding/binary"
	"fmt"
)

// DeviceConfig represents the device configuration (12 bytes)
// Response from GetDeviceConfig (0x01)
type DeviceConfig struct {
	Version       uint16 // Config format version
	Flags         uint32 // Device feature flags
	ActiveProfile uint8  // Currently selected profile slot
	Brightness    uint8  // LED brightness (0-255)
	DebounceMs    uint16 // Input debounce time in milliseconds
	Reserved      uint16 // Reserved bytes
}

const DeviceConfigSize = 12

// MarshalBinary encodes DeviceConfig to binary (12 bytes)
func (c *DeviceConfig) MarshalBinary() ([]byte, error) {
	buf := make([]byte, DeviceConfigSize)
	binary.LittleEndian.PutUint16(buf[0:2], c.Version)
	binary.LittleEndian.PutUint32(buf[2:6], c.Flags)
	buf[6] = c.ActiveProfile
	buf[7] = c.Brightness
	binary.LittleEndian.PutUint16(buf[8:10], c.DebounceMs)
	binary.LittleEndian.PutUint16(buf[10:12], c.Reserved)
	return buf, nil
}

// UnmarshalBinary decodes DeviceConfig from binary (12 bytes)
func (c *DeviceConfig) UnmarshalBinary(data []byte) error {
	if len(data) < DeviceConfigSize {
		return fmt.Errorf("insufficient data for DeviceConfig: got %d bytes, need %d", len(data), DeviceConfigSize)
	}
	c.Version = binary.LittleEndian.Uint16(data[0:2])
	c.Flags = binary.LittleEndian.Uint32(data[2:6])
	c.ActiveProfile = data[6]
	c.Brightness = data[7]
	c.DebounceMs = binary.LittleEndian.Uint16(data[8:10])
	c.Reserved = binary.LittleEndian.Uint16(data[10:12])
	return nil
}

// Profile represents a profile configuration (286 bytes)
type Profile struct {
	// Profile fields - exact layout depends on device implementation
	// This is a placeholder structure that can be expanded
	Data []byte
}

const ProfileSize = 286

// MarshalBinary returns the profile as 286 bytes
func (p *Profile) MarshalBinary() ([]byte, error) {
	if len(p.Data) != ProfileSize {
		return nil, fmt.Errorf("profile data must be %d bytes, got %d", ProfileSize, len(p.Data))
	}
	return p.Data, nil
}

// UnmarshalBinary decodes 286 bytes into the profile
func (p *Profile) UnmarshalBinary(data []byte) error {
	if len(data) < ProfileSize {
		return fmt.Errorf("insufficient data for Profile: got %d bytes, need %d", len(data), ProfileSize)
	}
	p.Data = make([]byte, ProfileSize)
	copy(p.Data, data[:ProfileSize])
	return nil
}

// StorageStats represents filesystem usage statistics
type StorageStats struct {
	TotalBlocks uint32
	UsedBlocks  uint32
	FreeBlocks  uint32
	BlockSize   uint32
}

const StorageStatsSize = 16

// UnmarshalBinary decodes StorageStats from binary
func (s *StorageStats) UnmarshalBinary(data []byte) error {
	if len(data) < StorageStatsSize {
		return fmt.Errorf("insufficient data for StorageStats: got %d bytes, need %d", len(data), StorageStatsSize)
	}
	s.TotalBlocks = binary.LittleEndian.Uint32(data[0:4])
	s.UsedBlocks = binary.LittleEndian.Uint32(data[4:8])
	s.FreeBlocks = binary.LittleEndian.Uint32(data[8:12])
	s.BlockSize = binary.LittleEndian.Uint32(data[12:16])
	return nil
}

// VersionInfo represents firmware and config version information
type VersionInfo struct {
	FirmwareVersion string
	ConfigVersion   uint16
}

// UnmarshalBinary decodes VersionInfo from binary (null-terminated string + uint16)
func (v *VersionInfo) UnmarshalBinary(data []byte) error {
	// Find null terminator for firmware version string
	var fwEnd int
	for fwEnd = 0; fwEnd < len(data); fwEnd++ {
		if data[fwEnd] == 0 {
			break
		}
	}
	if fwEnd >= len(data)-2 {
		return fmt.Errorf("invalid VersionInfo format")
	}
	v.FirmwareVersion = string(data[:fwEnd])
	v.ConfigVersion = binary.LittleEndian.Uint16(data[fwEnd+1:])
	return nil
}

// ProfileList represents the list of occupied profile slots
type ProfileList struct {
	Slots []uint8
}

// UnmarshalBinary decodes ProfileList from binary (array of slot numbers)
func (p *ProfileList) UnmarshalBinary(data []byte) error {
	p.Slots = make([]uint8, len(data))
	copy(p.Slots, data)
	return nil
}

// Request builders

// BuildGetDeviceConfigRequest creates a GetDeviceConfig request frame
func BuildGetDeviceConfigRequest() []byte {
	return BuildFrame(CmdGetDeviceConfig, nil)
}

// BuildSetDeviceConfigRequest creates a SetDeviceConfig request frame
func BuildSetDeviceConfigRequest(config *DeviceConfig) ([]byte, error) {
	payload, err := config.MarshalBinary()
	if err != nil {
		return nil, err
	}
	return BuildFrame(CmdSetDeviceConfig, payload), nil
}

// BuildGetProfileRequest creates a GetProfile request frame
func BuildGetProfileRequest(slot uint8) []byte {
	return BuildFrame(CmdGetProfile, []byte{slot})
}

// BuildSetProfileRequest creates a SetProfile request frame
func BuildSetProfileRequest(slot uint8, profile *Profile) ([]byte, error) {
	profileData, err := profile.MarshalBinary()
	if err != nil {
		return nil, err
	}
	// Payload: [slot:1][profile:286]
	payload := make([]byte, 1+ProfileSize)
	payload[0] = slot
	copy(payload[1:], profileData)
	return BuildFrame(CmdSetProfile, payload), nil
}

// BuildDeleteProfileRequest creates a DeleteProfile request frame
func BuildDeleteProfileRequest(slot uint8) []byte {
	return BuildFrame(CmdDeleteProfile, []byte{slot})
}

// BuildListProfilesRequest creates a ListProfiles request frame
func BuildListProfilesRequest() []byte {
	return BuildFrame(CmdListProfiles, nil)
}

// BuildGetStorageStatsRequest creates a GetStorageStats request frame
func BuildGetStorageStatsRequest() []byte {
	return BuildFrame(CmdGetStorageStats, nil)
}

// BuildPingRequest creates a Ping request frame
func BuildPingRequest(payload []byte) []byte {
	return BuildFrame(CmdPing, payload)
}

// BuildFactoryResetRequest creates a FactoryReset request frame
func BuildFactoryResetRequest() []byte {
	return BuildFrame(CmdFactoryReset, nil)
}

// BuildGetVersionRequest creates a GetVersion request frame
func BuildGetVersionRequest() []byte {
	return BuildFrame(CmdGetVersion, nil)
}

// Response parsers

// ParseDeviceConfigResponse parses a GetDeviceConfig response
func ParseDeviceConfigResponse(data []byte) (*DeviceConfig, error) {
	status, payload, err := ParseFrame(data)
	if err != nil {
		return nil, err
	}
	if err := CheckStatus(status); err != nil {
		return nil, err
	}
	config := &DeviceConfig{}
	if err := config.UnmarshalBinary(payload); err != nil {
		return nil, err
	}
	return config, nil
}

// ParseProfileResponse parses a GetProfile response
func ParseProfileResponse(data []byte) (*Profile, error) {
	status, payload, err := ParseFrame(data)
	if err != nil {
		return nil, err
	}
	if err := CheckStatus(status); err != nil {
		return nil, err
	}
	profile := &Profile{}
	if err := profile.UnmarshalBinary(payload); err != nil {
		return nil, err
	}
	return profile, nil
}

// ParseSimpleResponse parses a simple OK/Error response (SetDeviceConfig, SetProfile, DeleteProfile, FactoryReset)
func ParseSimpleResponse(data []byte) error {
	status, _, err := ParseFrame(data)
	if err != nil {
		return err
	}
	return CheckStatus(status)
}

// ParseProfileListResponse parses a ListProfiles response
func ParseProfileListResponse(data []byte) (*ProfileList, error) {
	status, payload, err := ParseFrame(data)
	if err != nil {
		return nil, err
	}
	if err := CheckStatus(status); err != nil {
		return nil, err
	}
	list := &ProfileList{}
	if err := list.UnmarshalBinary(payload); err != nil {
		return nil, err
	}
	return list, nil
}

// ParseStorageStatsResponse parses a GetStorageStats response
func ParseStorageStatsResponse(data []byte) (*StorageStats, error) {
	status, payload, err := ParseFrame(data)
	if err != nil {
		return nil, err
	}
	if err := CheckStatus(status); err != nil {
		return nil, err
	}
	stats := &StorageStats{}
	if err := stats.UnmarshalBinary(payload); err != nil {
		return nil, err
	}
	return stats, nil
}

// ParsePingResponse parses a Ping response (returns echoed payload)
func ParsePingResponse(data []byte) ([]byte, error) {
	status, payload, err := ParseFrame(data)
	if err != nil {
		return nil, err
	}
	if err := CheckStatus(status); err != nil {
		return nil, err
	}
	return payload, nil
}

// ParseVersionResponse parses a GetVersion response
func ParseVersionResponse(data []byte) (*VersionInfo, error) {
	status, payload, err := ParseFrame(data)
	if err != nil {
		return nil, err
	}
	if err := CheckStatus(status); err != nil {
		return nil, err
	}
	version := &VersionInfo{}
	if err := version.UnmarshalBinary(payload); err != nil {
		return nil, err
	}
	return version, nil
}

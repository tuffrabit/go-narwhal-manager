package protocol

import (
	"fmt"
	"time"

	"go.bug.st/serial"
)

const (
	// DefaultReadTimeout for serial port operations
	DefaultReadTimeout = time.Second

	// DefaultBaudRate for serial communication
	DefaultBaudRate = 115200

	// ReadBufferSize for reading responses
	ReadBufferSize = 512
)

// Device represents a connected Tuffpad device
type Device struct {
	port     serial.Port
	portName string
}

// Port returns the underlying serial port for direct access (e.g., for console)
func (d *Device) Port() serial.Port {
	return d.port
}

// DeviceConfig holds configuration for connecting to a device
type DeviceConfigOptions struct {
	BaudRate    int
	ReadTimeout time.Duration
}

// DefaultDeviceConfig returns default configuration options
func DefaultDeviceConfig() *DeviceConfigOptions {
	return &DeviceConfigOptions{
		BaudRate:    DefaultBaudRate,
		ReadTimeout: DefaultReadTimeout,
	}
}

// Connect opens a connection to a device on the specified port
func Connect(portName string, config *DeviceConfigOptions) (*Device, error) {
	if config == nil {
		config = DefaultDeviceConfig()
	}

	mode := &serial.Mode{
		BaudRate: config.BaudRate,
		Parity:   serial.NoParity,
		DataBits: 8,
		StopBits: serial.OneStopBit,
	}

	port, err := serial.Open(portName, mode)
	if err != nil {
		return nil, fmt.Errorf("failed to open port %s: %w", portName, err)
	}

	// Set DTR high - required for TinyGo USB CDC which drops writes if DTR is not asserted
	if err := port.SetDTR(true); err != nil {
		port.Close()
		return nil, fmt.Errorf("failed to set DTR: %w", err)
	}

	// Wait for device to be ready. The TinyGo firmware waits for DTR before
	// starting its serial read loop. We need to give it a moment to be ready
	// to receive our first request, otherwise it may miss it.
	time.Sleep(100 * time.Millisecond)

	if err := port.SetReadTimeout(config.ReadTimeout); err != nil {
		port.Close()
		return nil, fmt.Errorf("failed to set read timeout: %w", err)
	}

	return &Device{
		port:     port,
		portName: portName,
	}, nil
}

// ConnectAndDiscover opens a connection and verifies it's a Tuffpad device
func ConnectAndDiscover(portName string, config *DeviceConfigOptions) (*Device, error) {
	device, err := Connect(portName, config)
	if err != nil {
		return nil, err
	}

	isTuffpad, err := device.Discover()
	if err != nil {
		device.Close()
		return nil, fmt.Errorf("discovery failed: %w", err)
	}

	if !isTuffpad {
		device.Close()
		return nil, fmt.Errorf("device on %s is not a Tuffpad", portName)
	}

	return device, nil
}

// Close closes the serial connection
func (d *Device) Close() error {
	if d.port != nil {
		return d.port.Close()
	}
	return nil
}

// PortName returns the serial port name
func (d *Device) PortName() string {
	return d.portName
}

// sendRequest sends a request frame and reads the response
func (d *Device) sendRequest(request []byte) ([]byte, error) {
	// Clear any pending data
	d.port.ResetInputBuffer()
	d.port.ResetOutputBuffer()

	// Send request
	n, err := d.port.Write(request)
	if err != nil {
		return nil, fmt.Errorf("write failed: %w", err)
	}
	if n != len(request) {
		return nil, fmt.Errorf("short write: sent %d of %d bytes", n, len(request))
	}

	// Read response
	// We may need to read multiple times to get the full frame
	buf := make([]byte, ReadBufferSize)
	var response []byte

	for {
		n, err = d.port.Read(buf)
		if err != nil {
			return nil, fmt.Errorf("read failed: %w", err)
		}
		if n == 0 {
			// Timeout or no more data
			break
		}
		response = append(response, buf[:n]...)

		// Check if we have a complete frame
		if len(response) >= MinFrameSize {
			// Try to determine expected frame length
			if response[0] == SyncByte && len(response) >= 4 {
				payloadLen := uint16(response[2]) | (uint16(response[3]) << 8)
				expectedLen := MinFrameSize + int(payloadLen)
				if len(response) >= expectedLen {
					break
				}
			}
		}
	}

	return response, nil
}

// Discover sends a discovery request and checks for valid response.
// It retries a few times to handle race conditions during initial connection.
func (d *Device) Discover() (bool, error) {
	const maxRetries = 3
	
	for attempt := 0; attempt < maxRetries; attempt++ {
		frame := BuildDiscoverFrame()
		response, err := d.sendRequest(frame)
		if err != nil {
			// On last attempt, return the error
			if attempt == maxRetries-1 {
				return false, err
			}
			// Otherwise retry after a short delay
			time.Sleep(50 * time.Millisecond)
			continue
		}
		
		result, err := ParseDiscoverResponse(response)
		if err != nil {
			// On last attempt, return the error
			if attempt == maxRetries-1 {
				return false, err
			}
			// Otherwise retry after a short delay
			time.Sleep(50 * time.Millisecond)
			continue
		}
		
		return result, nil
	}
	
	return false, fmt.Errorf("discovery failed after %d attempts", maxRetries)
}

// GetDeviceConfig reads the current device configuration
func (d *Device) GetDeviceConfig() (*DeviceConfig, error) {
	frame := BuildGetDeviceConfigRequest()
	response, err := d.sendRequest(frame)
	if err != nil {
		return nil, err
	}
	return ParseDeviceConfigResponse(response)
}

// SetDeviceConfig writes the device configuration
func (d *Device) SetDeviceConfig(config *DeviceConfig) error {
	frame, err := BuildSetDeviceConfigRequest(config)
	if err != nil {
		return err
	}
	response, err := d.sendRequest(frame)
	if err != nil {
		return err
	}
	return ParseSimpleResponse(response)
}

// GetProfile reads a profile by slot number
func (d *Device) GetProfile(slot uint8) (*Profile, error) {
	frame := BuildGetProfileRequest(slot)
	response, err := d.sendRequest(frame)
	if err != nil {
		return nil, err
	}
	return ParseProfileResponse(response)
}

// SetProfile writes a profile to a slot
func (d *Device) SetProfile(slot uint8, profile *Profile) error {
	frame, err := BuildSetProfileRequest(slot, profile)
	if err != nil {
		return err
	}
	response, err := d.sendRequest(frame)
	if err != nil {
		return err
	}
	return ParseSimpleResponse(response)
}

// DeleteProfile removes a profile from a slot
func (d *Device) DeleteProfile(slot uint8) error {
	frame := BuildDeleteProfileRequest(slot)
	response, err := d.sendRequest(frame)
	if err != nil {
		return err
	}
	return ParseSimpleResponse(response)
}

// ListProfiles gets the list of occupied profile slots
func (d *Device) ListProfiles() (*ProfileList, error) {
	frame := BuildListProfilesRequest()
	response, err := d.sendRequest(frame)
	if err != nil {
		return nil, err
	}
	return ParseProfileListResponse(response)
}

// GetStorageStats gets filesystem usage statistics
func (d *Device) GetStorageStats() (*StorageStats, error) {
	frame := BuildGetStorageStatsRequest()
	response, err := d.sendRequest(frame)
	if err != nil {
		return nil, err
	}
	return ParseStorageStatsResponse(response)
}

// Ping sends an echo test request
func (d *Device) Ping(payload []byte) ([]byte, error) {
	frame := BuildPingRequest(payload)
	response, err := d.sendRequest(frame)
	if err != nil {
		return nil, err
	}
	return ParsePingResponse(response)
}

// FactoryReset wipes all configuration
func (d *Device) FactoryReset() error {
	frame := BuildFactoryResetRequest()
	response, err := d.sendRequest(frame)
	if err != nil {
		return err
	}
	return ParseSimpleResponse(response)
}

// GetVersion gets firmware and config version info
func (d *Device) GetVersion() (*VersionInfo, error) {
	frame := BuildGetVersionRequest()
	response, err := d.sendRequest(frame)
	if err != nil {
		return nil, err
	}
	return ParseVersionResponse(response)
}

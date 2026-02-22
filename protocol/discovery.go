package protocol

import (
	"fmt"
	"time"

	"go.bug.st/serial"
)

// DiscoveryResult represents a found Tuffpad device
type DiscoveryResult struct {
	PortName string
	Device   *Device
}

// FindTuffpads enumerates all serial ports and returns any Tuffpad devices found
func FindTuffpads(timeout time.Duration) ([]*DiscoveryResult, error) {
	ports, err := serial.GetPortsList()
	if err != nil {
		return nil, fmt.Errorf("failed to enumerate serial ports: %w", err)
	}

	var results []*DiscoveryResult
	
	for _, portName := range ports {
		device, err := tryPort(portName, timeout)
		if err != nil {
			// Log or skip ports that fail
			continue
		}
		if device != nil {
			results = append(results, &DiscoveryResult{
				PortName: portName,
				Device:   device,
			})
		}
	}

	return results, nil
}

// FindFirstTuffpad finds the first available Tuffpad device
func FindFirstTuffpad(timeout time.Duration) (*Device, string, error) {
	ports, err := serial.GetPortsList()
	if err != nil {
		return nil, "", fmt.Errorf("failed to enumerate serial ports: %w", err)
	}

	if len(ports) == 0 {
		return nil, "", fmt.Errorf("no serial ports found")
	}

	for _, portName := range ports {
		device, err := tryPort(portName, timeout)
		if err != nil {
			continue
		}
		if device != nil {
			return device, portName, nil
		}
	}

	return nil, "", fmt.Errorf("no Tuffpad device found")
}

// tryPort attempts to connect to a port and discover if it's a Tuffpad
func tryPort(portName string, timeout time.Duration) (*Device, error) {
	config := &DeviceConfigOptions{
		BaudRate:    DefaultBaudRate,
		ReadTimeout: timeout,
	}

	device, err := Connect(portName, config)
	if err != nil {
		return nil, err
	}

	// Try to discover
	isTuffpad, err := device.Discover()
	if err != nil {
		device.Close()
		return nil, err
	}

	if !isTuffpad {
		device.Close()
		return nil, nil
	}

	return device, nil
}

// IsTuffpad checks if a specific port is a Tuffpad device
func IsTuffpad(portName string, timeout time.Duration) (bool, error) {
	device, err := tryPort(portName, timeout)
	if err != nil {
		return false, err
	}
	if device == nil {
		return false, nil
	}
	// Don't close the device, return it for use
	// Caller is responsible for closing
	return true, nil
}

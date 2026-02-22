package main

import (
	"fmt"
	"time"

	"go.bug.st/serial"
	. "modernc.org/tk9.0"
	_ "modernc.org/tk9.0/themes/azure"

	"github.com/tuffrabit/go-narwhal-manager/protocol"
	"github.com/tuffrabit/go-narwhal-manager/view"
)

var (
	device     *protocol.Device
	serialPort serial.Port
)

func main() {
	ActivateTheme("azure light")
	window := App.Center()
	WmMinSize(window, 1280, 720)
	manager := &AppManager{window: window}
	manager.SwitchTo(&view.LoadingView{})

	go func() {
		handleTuffDeviceTest(manager)
	}()

	window.Wait()
}

func handleTuffDeviceTest(manager *AppManager) {
	dev, err := findTuffDevice()
	if err != nil {
		PostEvent(func() {
			manager.SwitchTo(view.NewDeviceRetryView(err, func() {
				handleTuffDeviceTest(manager)
			}))
		}, false)
	} else {
		device = dev
		serialPort = dev.Port()
		PostEvent(func() {
			manager.SwitchTo(view.NewMainView(serialPort))
		}, false)
	}
}

func findTuffDevice() (*protocol.Device, error) {
	// Enumerate serial ports
	ports, err := serial.GetPortsList()
	if err != nil {
		return nil, fmt.Errorf("main.findTuffDevice: serial port enumeration failed, error: %w", err)
	}

	if len(ports) == 0 {
		return nil, fmt.Errorf("main.findTuffDevice: no serial ports found")
	}

	timeout := time.Millisecond * 500

	for _, portName := range ports {
		fmt.Printf("Found port: %v\n", portName)
		fmt.Printf("Probing port: %v\n", portName)

		device, err := protocol.ConnectAndDiscover(portName, &protocol.DeviceConfigOptions{
			BaudRate:    115200,
			ReadTimeout: timeout,
		})
		if err != nil {
			// Log the error but continue probing other ports
			fmt.Printf("  Not a Tuffpad: %v\n", err)
			continue
		}

		fmt.Printf("  Found Tuffpad device!\n")
		return device, nil
	}

	return nil, fmt.Errorf("main.findTuffDevice: no Tuffpad devices found")
}

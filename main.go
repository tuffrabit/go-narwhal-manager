package main

import (
	"fmt"
	"time"

	"go.bug.st/serial"
	. "modernc.org/tk9.0"
	_ "modernc.org/tk9.0/themes/azure"

	"github.com/tuffrabit/go-narwhal-manager/view"
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
	if err := findTuffDevice(); err != nil {
		PostEvent(func() {
			manager.SwitchTo(view.NewDeviceRetryView(err, func() {
				handleTuffDeviceTest(manager)
			}))
		}, false)
	} else {
		PostEvent(func() {
			manager.SwitchTo(&view.TestView{})
		}, false)
	}
}

func findTuffDevice() error {
	fmt.Println("!STARTING TO LOOK FOR TUFF DEVICES!")
	ports, err := serial.GetPortsList()
	if err != nil {
		return fmt.Errorf("main.findTuffDevice: serial port enumeration failed, error: %w", err)
	}

	if len(ports) == 0 {
		return fmt.Errorf("main.findTuffDevice: no serial ports found")
	}

	mode := &serial.Mode{
		BaudRate: 115200,
		Parity:   serial.NoParity,
		DataBits: 8,
		StopBits: serial.OneStopBit,
	}

	for _, portName := range ports {
		fmt.Printf("Found port: %v\n", portName)
		fmt.Printf("Connecting port: %v\n", portName)

		port, err := serial.Open(portName, mode)
		if err != nil {
			return fmt.Errorf("main.findTuffDevice: open port %s failed, error: %w", portName, err)
		}

		port.SetReadTimeout(time.Millisecond * 1000)
		isTuffDevice, err := testPort(port, portName)
		if err != nil {
			return fmt.Errorf("main.findTuffDevice: probing port %s failed, error: %w", portName, err)
		}
		if isTuffDevice {
			return nil
		}
	}

	return fmt.Errorf("main.findTuffDevice: no tuff devices found")
}

func testPort(port serial.Port, portName string) (bool, error) {
	//areyouatuffpad?
	n, err := port.Write([]byte("areyouatuffpad?\n"))
	if err != nil {
		return false, fmt.Errorf("main.testPort: write to port %s failed, error: %w", portName, err)
	}
	fmt.Printf("Sent %v bytes\n", n)

	buff := make([]byte, 100)
	for {
		n, err := port.Read(buff)
		if err != nil {
			return false, fmt.Errorf("main.testPort: read from port %s failed, error: %w", portName, err)
		}
		if n == 0 {
			fmt.Println("\nEOF")
			break
		}
		fmt.Printf("%v", string(buff[:n]))
	}

	return false, nil
}

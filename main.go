package main

import (
	"fmt"
	"log"
	"time"

	"go.bug.st/serial"
)

func main() {
	ports, err := serial.GetPortsList()
	if err != nil {
		log.Fatal(err)
	}

	if len(ports) == 0 {
		log.Fatal("No serial ports found!")
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
			log.Fatal(err)
		}

		port.SetReadTimeout(time.Millisecond * 1000)
		testPort(port)
	}
}

func testPort(port serial.Port) {
	//areyouatuffpad?
	n, err := port.Write([]byte("areyouatuffpad?\n"))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Sent %v bytes\n", n)

	buff := make([]byte, 100)
	for {
		n, err := port.Read(buff)
		if err != nil {
			log.Fatal(err)
			break
		}
		if n == 0 {
			fmt.Println("\nEOF")
			break
		}
		fmt.Printf("%v", string(buff[:n]))
	}
}

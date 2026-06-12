package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"os"
)

// jsEvent represents a Linux joystick event structure.
type jsEvent struct {
	Time   uint32 // event timestamp in milliseconds
	Value  int16  // value (button: 0 or 1, axis: -32767 to 32767)
	Type   uint8  // event type (button or axis)
	Number uint8  // the index of the axis or button
}

const (
	jsEventButton uint8 = 0x01
	jsEventAxis   uint8 = 0x02
	jsEventInit   uint8 = 0x80 // bitmask for initial state events
)

// Mapping for Logitech F310 buttons in XInput mode
var buttonNames = map[uint8]string{
	0: "A", 1: "B", 2: "X", 3: "Y",
	4: "LB", 5: "RB", 6: "Back", 7: "Start",
	8: "Logi", 9: "L3", 10: "R3",
}

func main() {
	devicePath := "/dev/input/js0"
	if len(os.Args) > 1 {
		devicePath = os.Args[1]
	}

	// Open the raw joystick device file
	file, err := os.Open(devicePath)
	if err != nil {
		log.Fatalf("ERROR: Failed to open %s. (Try running with sudo if you get Permission Denied)\nError: %v", devicePath, err)
	}
	defer file.Close()

	fmt.Printf("--- Recording Logitech F310 Activity from %s ---\n", devicePath)
	fmt.Println("--- Press Ctrl+C to exit ---\n")

	for {
		var event jsEvent
		// Read 8 bytes from the device
		err := binary.Read(file, binary.LittleEndian, &event)
		if err != nil {
			log.Fatalf("ERROR: Failed to read event: %v", err)
		}

		// Check if this is an "Initial State" event or a "Real-time" event
		isInit := (event.Type & jsEventInit) != 0
		eventType := event.Type & (^jsEventInit)

		prefix := " EVENT "
		if isInit {
			prefix = "[INIT] "
		}

		switch eventType {
		case jsEventButton:
			name, ok := buttonNames[event.Number]
			if !ok {
				name = fmt.Sprintf("Button %d", event.Number)
			}
			action := "RELEASED"
			if event.Value == 1 {
				action = "PRESSED"
			}
			fmt.Printf("%s | Type: Button | ID: %-2d | Name: %-5s | Action: %s\n", prefix, event.Number, name, action)

		case jsEventAxis:
			fmt.Printf("%s | Type: Axis   | ID: %-2d | Value: %d\n", prefix, event.Number, event.Value)
		}
	}
}

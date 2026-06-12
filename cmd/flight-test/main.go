package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gdamore/tcell/v2"
)

type jsEvent struct {
	Time   uint32
	Value  int16
	Type   uint8
	Number uint8
}

const (
	jsEventButton uint8 = 0x01
	jsEventAxis   uint8 = 0x02
	jsEventInit   uint8 = 0x80
)

var sprites = [8][3][5]rune{
	{{' ', ' ', '▲', ' ', ' '}, {'/', '[', '*', ']', '\\'}, {' ', ' ', '║', ' ', ' '}}, // N
	{{' ', ' ', ' ', '\\', '▲'}, {' ', ' ', '*', '╪', '\\'}, {'/', '║', ' ', ' ', ' '}}, // NE
	{{' ', ' ', '═', '═', ' '}, {'╟', '▓', '*', '▓', '►'}, {' ', ' ', '═', '═', ' '}}, // E
	{{'/', '║', ' ', ' ', ' '}, {' ', ' ', '*', '╪', '/'}, {' ', ' ', ' ', '/', '▼'}}, // SE
	{{' ', ' ', '║', ' ', ' '}, {'/', '[', '*', ']', '\\'}, {' ', ' ', '▼', ' ', ' '}}, // S
	{{' ', ' ', ' ', '║', '\\'}, {'\\', '╪', '*', ' ', ' '}, {'▼', '\\', ' ', ' ', ' '}}, // SW
	{{' ', ' ', '═', '═', ' '}, {'◄', '▓', '*', '▓', '╢'}, {' ', ' ', '═', '═', ' '}}, // W
	{{'▲', '/', ' ', ' ', ' '}, {'/', '╪', '*', ' ', ' '}, {' ', ' ', ' ', '║', '/'}}, // NW
}
var rotorFrames = []rune{'|', '/', '-', '\\'}

func main() {
	devicePath := "/dev/input/js0"
	if len(os.Args) > 1 {
		devicePath = os.Args[1]
	}

	file, fileErr := os.Open(devicePath)
	s, err := tcell.NewScreen()
	if err != nil { log.Fatalf("%v", err) }
	if err := s.Init(); err != nil { log.Fatalf("%v", err) }
	defer s.Fini()
	s.HideCursor()

	w, h := s.Size()
	x, y := float64(w/2), float64(h/2)
	vx, vy := 0.0, 0.0
	dir := 0
	rotorState := 0
	frameCount := 0

	// Live input values
	axes := make(map[uint8]int16)
	buttons := make(map[uint8]bool)
	kx, ky := 0.0, 0.0

	if fileErr == nil {
		go func() {
			defer file.Close()
			for {
				var event jsEvent
				if err := binary.Read(file, binary.LittleEndian, &event); err != nil { return }
				if (event.Type & jsEventInit) == 0 {
					if event.Type == jsEventAxis { axes[event.Number] = event.Value }
					if event.Type == jsEventButton { buttons[event.Number] = (event.Value == 1) }
				}
			}
		}()
	}

	quit := make(chan struct{})
	go func() {
		for {
			ev := s.PollEvent()
			if key, ok := ev.(*tcell.EventKey); ok {
				if key.Key() == tcell.KeyEscape || key.Key() == tcell.KeyCtrlC || key.Rune() == 'q' {
					close(quit)
					return
				}
				switch key.Key() {
				case tcell.KeyUp: ky = -1; case tcell.KeyDown: ky = 1; case tcell.KeyLeft: kx = -1; case tcell.KeyRight: kx = 1
				}
				switch key.Rune() {
				case 'w': ky = -1; case 's': ky = 1; case 'a': kx = -1; case 'd': kx = 1
				}
			}
			if _, ok := ev.(*tcell.EventResize); ok { s.Sync() }
		}
	}()

	ticker := time.NewTicker(40 * time.Millisecond)
	for {
		select {
		case <-quit: return
		case <-ticker.C:
			frameCount++
			// Calculate combined input
			var tx, ty float64
			
			// Left Stick (0,1)
			tx += float64(axes[0]) / 32767.0
			ty += float64(axes[1]) / 32767.0
			// Right Stick (3,4)
			tx += float64(axes[3]) / 32767.0
			ty += float64(axes[4]) / 32767.0
			// DPAD (6,7)
			tx += float64(axes[6]) / 32767.0
			ty += float64(axes[7]) / 32767.0
			// Keyboard
			tx += kx; ty += ky
			kx, ky = 0, 0 // Clear keyboard impulse

			// Physics
			if tx > 1 { tx = 1 } else if tx < -1 { tx = -1 }
			if ty > 1 { ty = 1 } else if ty < -1 { ty = -1 }

			if tx*tx + ty*ty > 0.01 {
				vx += tx * 0.2
				vy += ty * 0.2
				// Update direction based on stick angle
				if tx > 0.3 && ty < -0.3 { dir = 1 } else if tx > 0.3 && ty > 0.3 { dir = 3 } else if tx < -0.3 && ty > 0.3 { dir = 5 } else if tx < -0.3 && ty < -0.3 { dir = 7 } else if ty < -0.3 { dir = 0 } else if tx > 0.3 { dir = 2 } else if ty > 0.3 { dir = 4 } else if tx < -0.3 { dir = 6 }
			}
			
			vx *= 0.90
			vy *= 0.90
			x += vx
			y += vy
			rotorState = (rotorState + 1) % 4
			w, h = s.Size()
			if x < 0 { x = float64(w-1) } else if x > float64(w-1) { x = 0 }
			if y < 0 { y = float64(h-1) } else if y > float64(h-1) { y = 0 }

			// Draw
			s.Clear()
			style := tcell.StyleDefault.Background(tcell.ColorReset).Foreground(tcell.ColorWhite)
			dbg := tcell.StyleDefault.Background(tcell.ColorReset).Foreground(tcell.ColorYellow)

			// Diagnostic Info
			info := fmt.Sprintf("FRAME: %d | CTRL: %v | POS: %.0f,%.1f | INPUT: %.2f, %.2f", frameCount, fileErr == nil, x, y, tx, ty)
			for i, r := range info { s.SetContent(i+2, 1, r, nil, dbg) }
			
			// Instructions
			instr := "Use STICK, DPAD, or WASD to fly. Press 'Q' to quit."
			for i, r := range instr { s.SetContent(i+2, 2, r, nil, dbg) }

			// Heli
			sprite := sprites[dir]
			for row := 0; row < 3; row++ {
				for col := 0; col < 5; col++ {
					char := sprite[row][col]
					if row == 1 && col == 2 { char = rotorFrames[rotorState] }
					s.SetContent(int(x)+col-2, int(y)+row-1, char, nil, style)
				}
			}
			s.Show()
		}
	}
}

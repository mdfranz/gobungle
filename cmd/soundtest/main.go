package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"gobungle/internal/game"
)

var sounds = []string{"warning", "laser", "missile", "explosion"}

var notes = map[string]string{
	"warning":   "C64 original: pulsed beep, gap sweeps 50→134→50ms over ~3s",
	"laser":     "C64 original: ~30-40ms burst, sharp attack (<2ms), fast exponential decay",
	"missile":   "C64 original: whoosh ~400ms, shaped broadband noise",
	"explosion": "C64 original: deep sub-bass ~30Hz, noise-heavy, long decay ~500-900ms",
}

func printMenu() {
	fmt.Println("\nSound Test Tool  (Raid on Bungeling Bay C64 reference)")
	fmt.Println("-------------------------------------------------------")
	for i, s := range sounds {
		fmt.Printf("  %d) %-12s  — %s\n", i+1, s, notes[s])
	}
	fmt.Println("  a) play all (1s gap between each)")
	fmt.Println("  r) rapid-fire laser x10")
	fmt.Println("  q) quit")
	fmt.Print("\nChoice: ")
}

func main() {
	game.InitSound()
	// Let speaker settle
	time.Sleep(100 * time.Millisecond)

	scanner := bufio.NewScanner(os.Stdin)
	for {
		printMenu()
		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())
		switch input {
		case "q", "quit":
			return
		case "a", "all":
			for _, s := range sounds {
				fmt.Printf("  Playing: %s\n", s)
				game.PlaySound(s)
				time.Sleep(1500 * time.Millisecond)
			}
		case "r":
			fmt.Println("  Rapid-fire laser x10...")
			for i := 0; i < 10; i++ {
				game.PlaySound("laser")
				time.Sleep(100 * time.Millisecond)
			}
		case "1":
			fmt.Println("  Playing: warning")
			game.PlaySound("warning")
		case "2":
			fmt.Println("  Playing: laser")
			game.PlaySound("laser")
		case "3":
			fmt.Println("  Playing: missile")
			game.PlaySound("missile")
		case "4":
			fmt.Println("  Playing: explosion")
			game.PlaySound("explosion")
		default:
			fmt.Println("  Unknown choice.")
			continue
		}
		// Let sound finish before re-showing menu
		time.Sleep(1000 * time.Millisecond)
	}
}

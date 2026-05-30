package main

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/gdamore/tcell/v2"
)

func main() {
	// Initialize logging
	logFile, err := os.OpenFile("gobungle.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open log file: %v\n", err)
		os.Exit(1)
	}
	defer logFile.Close()

	// Set up structured slog with TextHandler
	logger := slog.New(slog.NewTextHandler(logFile, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	slog.Info("Gobungle Game Started")

	// Initialize tcell screen
	s, err := tcell.NewScreen()
	if err != nil {
		slog.Error("failed to create tcell screen", "error", err)
		fmt.Fprintf(os.Stderr, "failed to create tcell screen: %v\n", err)
		os.Exit(1)
	}
	if err := s.Init(); err != nil {
		slog.Error("failed to initialize tcell screen", "error", err)
		fmt.Fprintf(os.Stderr, "failed to initialize tcell screen: %v\n", err)
		os.Exit(1)
	}
	defer s.Fini()

	// Default style matching standard terminals
	s.SetStyle(tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.ColorBlack))
	s.Clear()

	w, h := s.Size()

	// Place aircraft carrier dynamically on the screen
	carrier := Carrier{
		X:      w / 6,
		Y:      h / 4,
		Width:  26,
		Height: 6,
		Health: 100.0,
	}

	// Calculate landing pad center coordinates
	padX := carrier.X + carrier.Width/3
	padY := carrier.Y + carrier.Height/2

	// Initialize helicopter sitting on the landing pad
	heli := Helicopter{
		X:      float64(padX),
		Y:      float64(padY),
		VX:     0,
		VY:     0,
		Dir:    0, // Pointing North
		Landed: true,
		Fuel:   100.0,
		Armor:  100.0,
		MissileAmmo: 4,
	}

	// Initialize 3 target boats at different water locations (adjusted for larger size and durability)
	boats := []Boat{
		{X: 15, Y: float64(h - 10), VX: 0.05, Health: 9, MaxHealth: 9, Active: true, MissileCooldown: 200},
		{X: float64(w - 25), Y: 6, VX: -0.04, Health: 9, MaxHealth: 9, Active: true, MissileCooldown: 400},
		{X: float64(w / 2), Y: float64(h - 7), VX: 0.06, Health: 9, MaxHealth: 9, Active: true, MissileCooldown: 600},
	}

	game := &Game{
		screen:     s,
		width:      w,
		height:     h,
		quit:       make(chan struct{}),
		heli:       heli,
		carrier:    carrier,
		bullets:    make([]Bullet, 0, 16),
		missiles:   make([]Missile, 0, 2),
		boats:      boats,
		explosions: make([]Explosion, 0, 8),
		boatsSunk:  0,
	}

	// Start decoupled tickers and input loop
	go game.gameLoop()
	game.inputLoop()
}

// gameLoop runs on a strict 40ms ticker (25 FPS) for physics and rendering
func (g *Game) gameLoop() {
	ticker := time.NewTicker(40 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-g.quit:
			return
		case <-ticker.C:
			// Fetch and update screen dimensions dynamically if resized
			g.width, g.height = g.screen.Size()

			g.updatePhysics()
			g.draw()
		}
	}
}

// inputLoop blocks on tcell event polling
func (g *Game) inputLoop() {
	for {
		ev := g.screen.PollEvent()
		switch ev := ev.(type) {
		case *tcell.EventResize:
			g.screen.Sync()
			nw, nh := g.screen.Size()
			slog.Info("Screen resized", "width", nw, "height", nh)

		case *tcell.EventKey:
			// Graceful Quit
			if ev.Key() == tcell.KeyEscape || ev.Key() == tcell.KeyCtrlC {
				slog.Info("Gobungle Game Shutting Down Gracefully")
				close(g.quit)
				return
			}

			// Handle input based on flight state
			g.handleKeyPress(ev)
		}
	}
}

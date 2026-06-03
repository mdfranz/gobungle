package main

import (
	"fmt"
	"log/slog"
	"os"

	"gobungle/internal/game"

	"github.com/gdamore/tcell/v2"
)

func main() {
	logFile, err := os.OpenFile("gobungle.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open log file: %v\n", err)
		os.Exit(1)
	}
	defer logFile.Close()

	logger := slog.New(slog.NewTextHandler(logFile, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)
	slog.Info("Gobungle Game Started")

	s, err := tcell.NewScreen()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create tcell screen: %v\n", err)
		os.Exit(1)
	}
	if err := s.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize tcell screen: %v\n", err)
		os.Exit(1)
	}
	defer s.Fini()

	s.SetStyle(tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.ColorBlack))
	s.Clear()

	game.InitSound()
	g := game.New(s)
	g.Run()
}

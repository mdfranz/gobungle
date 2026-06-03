package game

import (
	"log/slog"
	"math"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
)

type Game struct {
	mu             sync.Mutex
	screen         tcell.Screen
	width          int
	height         int
	worldWidth     int
	worldHeight    int
	camX           int
	camY           int
	quit           chan struct{}
	quitConfirming bool
	gameOver       bool
	Wave           int
	heli           Helicopter
	carrier        Carrier
	bullets        []Bullet
	missiles       []Missile
	boats          []Boat
	initialBoats   []Boat
	island         Island
	factories      []Factory
	drones         []Drone
	tanks          []Tank
	staticAAs      []StaticAA
	explosions     []Explosion
	boatsSunk      int
	Ticks          int
	lockedBoat     *Boat
	lockedFactory  *Factory
	lockedTank     *Tank
	lockedStaticAA *StaticAA
}

// New initializes the game world and returns a ready-to-run Game.
// The caller owns the screen lifecycle (Init/Fini).
func New(screen tcell.Screen) *Game {
	w, h := screen.Size()

	playableHeight := h - 4
	if playableHeight < 10 {
		playableHeight = 10
	}
	worldWidth := w * 2
	if worldWidth < 80 {
		worldWidth = 80
	}
	worldHeight := playableHeight * 2

	carrier := Carrier{
		X:      worldWidth / 10,
		Y:      worldHeight / 4,
		Width:  26,
		Height: 6,
		Health: 100.0,
	}

	padX := carrier.X + carrier.Width/3
	padY := carrier.Y + carrier.Height/2

	heli := Helicopter{
		X:           float64(padX),
		Y:           float64(padY),
		Dir:         0,
		Landed:      true,
		Fuel:        100.0,
		Armor:       100.0,
		MissileAmmo: 4,
	}

	boats := []Boat{
		{X: 15, Y: float64(worldHeight - 10), VX: 0.05, Health: 9, MaxHealth: 9, Active: true, MissileCooldown: 1500},
		{X: 20, Y: 6, VX: -0.04, Health: 9, MaxHealth: 9, Active: true, MissileCooldown: 2000},
		{X: 25, Y: float64(worldHeight - 7), VX: 0.06, Health: 9, MaxHealth: 9, Active: true, MissileCooldown: 2500},
	}

	factories := []Factory{
		{X: float64(worldWidth * 2 / 3), Y: float64(worldHeight / 8), Health: 15, MaxHealth: 15, Active: true, FireCooldown: 100, DronesRemaining: 8},
		{X: float64(worldWidth - 35), Y: float64(worldHeight / 2), Health: 15, MaxHealth: 15, Active: true, FireCooldown: 150, DronesRemaining: 8},
		{X: float64(worldWidth - 15), Y: float64(worldHeight * 7 / 8), Health: 15, MaxHealth: 15, Active: true, FireCooldown: 200, DronesRemaining: 8},
	}

	drones := make([]Drone, 0, len(factories)*2+2)
	for i, f := range factories {
		drones = append(drones, Drone{X: f.X + 8.0, Y: f.Y, Active: true, Angle: 0.0, FactoryIdx: i})
		drones = append(drones, Drone{X: f.X - 8.0, Y: f.Y, Active: true, Angle: 3.14159, FactoryIdx: i})
	}
	cx := float64(carrier.X + carrier.Width/2)
	cy := float64(carrier.Y + carrier.Height/2)
	// Add 3 active drones around the Carrier (represented by FactoryIdx = -1) spaced 120 degrees apart
	drones = append(drones, Drone{X: cx + 12.0, Y: cy, Active: true, Angle: 0.0, FactoryIdx: -1})
	drones = append(drones, Drone{X: cx, Y: cy, Active: true, Angle: 2.0 * math.Pi / 3.0, FactoryIdx: -1})
	drones = append(drones, Drone{X: cx, Y: cy, Active: true, Angle: 4.0 * math.Pi / 3.0, FactoryIdx: -1})

	tanks := []Tank{
		{X: float64(worldWidth - 15), Y: float64(worldHeight * 5 / 16), VY: 0.04, Health: 4, MaxHealth: 4, Active: false, PatrolDir: 0, MinCoord: float64(worldHeight / 8), MaxCoord: float64(worldHeight / 2)},
		{X: float64(worldWidth - 15), Y: float64(worldHeight * 11 / 16), VY: -0.04, Health: 4, MaxHealth: 4, Active: false, PatrolDir: 0, MinCoord: float64(worldHeight / 2), MaxCoord: float64(worldHeight * 7 / 8)},
		{X: float64(worldWidth - 11), Y: float64(worldHeight / 2), VX: 0.06, Health: 4, MaxHealth: 4, Active: false, PatrolDir: 1, MinCoord: float64(worldWidth - 15), MaxCoord: float64(worldWidth - 7)},
	}

	// Compute initial camera offset centered on the helicopter
	camX := int(math.Round(heli.X)) - w/2
	camY := int(math.Round(heli.Y)) - playableHeight/2
	if camX < 0 {
		camX = 0
	}
	if camX > worldWidth-w {
		camX = worldWidth - w
	}
	if camY < 0 {
		camY = 0
	}
	if camY > worldHeight-playableHeight {
		camY = worldHeight - playableHeight
	}

	g := &Game{
		screen:       screen,
		width:        w,
		height:       h,
		worldWidth:   worldWidth,
		worldHeight:  worldHeight,
		camX:         camX,
		camY:         camY,
		quit:         make(chan struct{}),
		Wave:         1,
		heli:         heli,
		carrier:      carrier,
		bullets:      make([]Bullet, 0, 16),
		missiles:     make([]Missile, 0, 2),
		boats:        boats,
		initialBoats: make([]Boat, len(boats)),
		island:       Island{Active: true},
		factories:    factories,
		drones:       drones,
		tanks:        tanks,
		explosions:   make([]Explosion, 0, 8),
	}
	// Always start gunboats close to the shore (water side of coastline threshold)
	for i := range g.boats {
		by := int(math.Round(g.boats[i].Y))
		thresh := g.getCoastlineThreshold(by)
		g.boats[i].X = thresh - 8.0
		g.boats[i].PatrolMinX = g.boats[i].X - 10.0
	}

	copy(g.initialBoats, g.boats)
	g.initStaticAAs()
	return g
}

// Run starts the physics/render loop and blocks on the input loop.
func (g *Game) Run() {
	go g.gameLoop()
	g.inputLoop()
}

// gameLoop runs on a strict 40ms ticker (25 FPS) for physics and rendering.
func (g *Game) gameLoop() {
	ticker := time.NewTicker(40 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-g.quit:
			return
		case <-ticker.C:
			g.mu.Lock()
			g.width, g.height = g.screen.Size()
			if !g.quitConfirming && !g.gameOver {
				g.updatePhysics()
			}
			g.draw()
			g.mu.Unlock()
		}
	}
}

// inputLoop blocks on tcell event polling and routes events under the game lock.
func (g *Game) inputLoop() {
	for {
		ev := g.screen.PollEvent()
		switch ev := ev.(type) {
		case *tcell.EventResize:
			g.mu.Lock()
			g.screen.Sync()
			nw, nh := g.screen.Size()
			g.width, g.height = nw, nh
			slog.Info("Screen resized", "width", nw, "height", nh)
			g.mu.Unlock()

		case *tcell.EventKey:
			g.mu.Lock()
			if g.gameOver {
				slog.Info("Gobungle Game Over. Shutting Down Gracefully.")
				g.mu.Unlock()
				close(g.quit)
				return
			}

			if g.quitConfirming {
				key := ev.Key()
				ch := ev.Rune()
				if ch == 'y' || ch == 'Y' || key == tcell.KeyCtrlC {
					slog.Info("Gobungle Game Shutting Down Gracefully")
					g.mu.Unlock()
					close(g.quit)
					return
				} else if ch == 'n' || ch == 'N' || key == tcell.KeyEscape {
					g.quitConfirming = false
				}
				g.mu.Unlock()
				continue
			}

			if ev.Key() == tcell.KeyEscape || ev.Key() == tcell.KeyCtrlC {
				g.quitConfirming = true
				g.mu.Unlock()
				continue
			}
			g.handleKeyPress(ev)
			g.mu.Unlock()
		}
	}
}

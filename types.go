package main

import "github.com/gdamore/tcell/v2"

// Directions: 0 = N, 1 = NE, 2 = E, 3 = SE, 4 = S, 5 = SW, 6 = W, 7 = NW
var dirNames = [8]string{"N", "NE", "E", "SE", "S", "SW", "W", "NW"}
var dirDegrees = [8]int{0, 45, 90, 135, 180, 225, 270, 315}

// Direction unit vectors (Y vector is pre-scaled by 0.5 to adjust for terminal cell height ratio)
var dx = [8]float64{0.0, 0.707, 1.0, 0.707, 0.0, -0.707, -1.0, -0.707}
var dy = [8]float64{-0.5, -0.354, 0.0, 0.354, 0.5, 0.354, 0.0, -0.354}

// 3x3 sprites for the 8 directions.
// Center (1,1) is replaced dynamically by the spinning rotor frame character.
var sprites = [8][3][3]rune{
	// 0: North
	{
		{' ', '▲', ' '},
		{'[', '*', ']'},
		{' ', '|', ' '},
	},
	// 1: North-East
	{
		{' ', '\\', '▲'},
		{' ', '*', '\\'},
		{'/', ' ', ' '},
	},
	// 2: East
	{
		{' ', '_', ' '},
		{'=', '*', '►'},
		{' ', '¯', ' '},
	},
	// 3: South-East
	{
		{'\\', ' ', ' '},
		{' ', '*', '/'},
		{' ', '/', '▼'},
	},
	// 4: South
	{
		{' ', '|', ' '},
		{'[', '*', ']'},
		{' ', '▼', ' '},
	},
	// 5: South-West
	{
		{' ', ' ', '/'},
		{'/', '*', ' '},
		{'▼', '\\', ' '},
	},
	// 6: West
	{
		{' ', '_', ' '},
		{'◄', '*', '='},
		{' ', '¯', ' '},
	},
	// 7: North-West
	{
		{'▲', '/', ' '},
		{'/', '*', ' '},
		{' ', ' ', '\\'},
	},
}

// Rotor animation frames
var rotorFrames = []rune{'|', '/', '-', '\\'}

// Carrier deck coordinates and dimensions
type Carrier struct {
	X      int
	Y      int
	Width  int
	Height int
	Health float64
}

// Projectile fired by helicopter or enemies
type Bullet struct {
	X                float64
	Y                float64
	StartX           float64
	StartY           float64
	VX               float64
	VY               float64
	Active           bool
	IsEnemy          bool // true if fired by enemy boat
	IsCountermeasure bool // true if spawned as defensive anti-missile CIWS
}

// Guided Missile fired by player helicopter or enemy boats
type Missile struct {
	X                  float64
	Y                  float64
	StartX             float64
	StartY             float64
	VX                 float64
	VY                 float64
	Active             bool
	InterceptionRolled bool
	IsEnemy            bool // true if fired by enemy boat at the carrier
}

// Enemy target boat
type Boat struct {
	X               float64
	Y               float64
	VX              float64
	Health          int
	MaxHealth       int
	Active          bool
	FireCooldown    int // ticks until next shot
	MissileCooldown int // ticks until next guided missile launch
	SinkingTimer    int // ticks until boat completely sinks, 0 if not sinking
}

// Visual explosion particle effect
type Explosion struct {
	X   int
	Y   int
	Age int // frames elapsed
}

// Helicopter flight stats
type Helicopter struct {
	X            float64
	Y            float64
	VX           float64
	VY           float64
	Dir          int
	RotorState   int
	Landed       bool
	Fuel         float64
	Armor           float64 // 0 to 100
	FireCooldown    int
	TakeoffCooldown int
	MissileCooldown int
	MissileAmmo     int
}

type Game struct {
	screen     tcell.Screen
	width      int
	height     int
	quit       chan struct{}
	heli       Helicopter
	carrier    Carrier
	bullets    []Bullet
	missiles   []Missile
	boats      []Boat
	explosions []Explosion
	boatsSunk  int
	Ticks      int
}

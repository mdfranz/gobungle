package game

// Directions: 0 = N, 1 = NE, 2 = E, 3 = SE, 4 = S, 5 = SW, 6 = W, 7 = NW
var dirNames = [8]string{"N", "NE", "E", "SE", "S", "SW", "W", "NW"}
var dirDegrees = [8]int{0, 45, 90, 135, 180, 225, 270, 315}

// Direction unit vectors (Y vector is pre-scaled by 0.5 to adjust for terminal cell height ratio)
var dx = [8]float64{0.0, 0.707, 1.0, 0.707, 0.0, -0.707, -1.0, -0.707}
var dy = [8]float64{-0.5, -0.354, 0.0, 0.354, 0.5, 0.354, 0.0, -0.354}

// 7x5 sprites for the 8 directions representing a V-22 Osprey attack variant.
// Any '*' character is replaced dynamically by the spinning rotor frame.
var sprites = [8][5][7]rune{
	// 0: North
	{
		{' ', ' ', ' ', '▲', ' ', ' ', ' '},
		{' ', '*', '═', '#', '═', '*', ' '},
		{' ', ' ', ' ', '#', ' ', ' ', ' '},
		{' ', ' ', ' ', '+', ' ', ' ', ' '},
		{' ', ' ', ' ', ' ', ' ', ' ', ' '},
	},
	// 1: NE
	{
		{' ', ' ', ' ', ' ', '▲', ' ', ' '},
		{' ', ' ', '*', '═', '#', '═', '*'},
		{' ', ' ', ' ', '#', ' ', ' ', ' '},
		{' ', ' ', '+', ' ', ' ', ' ', ' '},
		{' ', ' ', ' ', ' ', ' ', ' ', ' '},
	},
	// 2: East
	{
		{' ', ' ', ' ', ' ', '*', ' ', ' '},
		{' ', ' ', '+', ' ', '║', ' ', ' '},
		{' ', ' ', '=', '#', '#', '►', ' '},
		{' ', ' ', '+', ' ', '║', ' ', ' '},
		{' ', ' ', ' ', ' ', '*', ' ', ' '},
	},
	// 3: SE
	{
		{' ', ' ', ' ', ' ', ' ', ' ', ' '},
		{' ', ' ', '\\',' ', ' ', ' ', ' '},
		{' ', ' ', ' ', '#', ' ', ' ', ' '},
		{' ', ' ', '*', '═', '#', '═', '*'},
		{' ', ' ', ' ', ' ', ' ', '▼', ' '},
	},
	// 4: South
	{
		{' ', ' ', ' ', ' ', ' ', ' ', ' '},
		{' ', ' ', ' ', '+', ' ', ' ', ' '},
		{' ', ' ', ' ', '#', ' ', ' ', ' '},
		{' ', '*', '═', '#', '═', '*', ' '},
		{' ', ' ', ' ', '▼', ' ', ' ', ' '},
	},
	// 5: SW
	{
		{' ', ' ', ' ', ' ', ' ', '/', ' '},
		{' ', ' ', ' ', ' ', '/', ' ', ' '},
		{' ', ' ', ' ', '#', ' ', ' ', ' '},
		{'*', '═', '#', '═', '*', ' ', ' '},
		{' ', '▼', ' ', ' ', ' ', ' ', ' '},
	},
	// 6: West
	{
		{' ', ' ', '*', ' ', ' ', ' ', ' '},
		{' ', ' ', '║', ' ', '+', ' ', ' '},
		{' ', '◄', '#', '#', '=', ' ', ' '},
		{' ', ' ', '║', ' ', '+', ' ', ' '},
		{' ', ' ', '*', ' ', ' ', ' ', ' '},
	},
	// 7: NW
	{
		{' ', '▲', ' ', ' ', ' ', ' ', ' '},
		{'*', '═', '#', '═', '*', ' ', ' '},
		{' ', ' ', ' ', '#', ' ', ' ', ' '},
		{' ', ' ', ' ', ' ', '\\', ' ', ' '},
		{' ', ' ', ' ', ' ', ' ', '\\', ' '},
	},
}

// Rotor animation frames
var rotorFrames = []rune{'|', '/', '-', '\\'}

// Carrier deck coordinates and dimensions
type Carrier struct {
	X               int
	Y               int
	Width           int
	Height          int
	Health          float64
	MissileCooldown int
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
	IsEnemy          bool
	IsCountermeasure bool
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
	IsEnemy            bool
	IsCarrier          bool
}

// Enemy target boat
type Boat struct {
	X               float64
	Y               float64
	VX              float64
	Health          int
	MaxHealth       int
	Active          bool
	FireCooldown    int
	MissileCooldown int
	SinkingTimer    int
	PatrolMinX      float64
}

// Visual explosion particle effect
type Explosion struct {
	X   int
	Y   int
	Age int
}

// Helicopter flight stats
type Helicopter struct {
	X               float64
	Y               float64
	VX              float64
	VY              float64
	Dir             int
	RotorState      int
	Landed          bool
	Fuel            float64
	Armor           float64
	FireCooldown    int
	TakeoffCooldown int
	MissileCooldown int
	MissileAmmo     int
	RespawnTimer        int
	CannonHeat          int
	CannonJammed        int
	ReturningToCarrier  bool
}

// Central island housing the enemy factory
type Island struct {
	X      int
	Y      int
	Width  int
	Height int
	Active bool
}

// Enemy military factory with anti-aircraft defenses
type Factory struct {
	X               float64
	Y               float64
	Health          int
	MaxHealth       int
	Active          bool
	FireCooldown    int
	SinkingTimer    int
	DronesRemaining int
}

// Agile air defense drones that orbit and protect the factory
type Drone struct {
	X          float64
	Y          float64
	VX         float64
	VY         float64
	Active     bool
	Angle      float64
	FactoryIdx int
}

// Mobile air defense guns (tanks) that patrol the island's road network
type Tank struct {
	X            float64
	Y            float64
	VX           float64
	VY           float64
	Health       int
	MaxHealth    int
	Active       bool
	FireCooldown int
	SinkingTimer int
	PatrolDir    int
	MinCoord     float64
	MaxCoord     float64
}

// High-speed stealth drone speedboat: invisible to radar, reaches carrier = instant game over
type StealthBoat struct {
	X      float64
	Y      float64
	VX     float64
	Active bool
}

// Static anti-aircraft gun emplacements along the coastline
type StaticAA struct {
	X            float64
	Y            float64
	Health       int
	MaxHealth    int
	Active       bool
	FireCooldown int
	SinkingTimer int
}

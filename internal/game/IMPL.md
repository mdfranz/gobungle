# Gobungle Game Implementation

## Code Organization

### Architecture Overview
The game is organized as a single-threaded event loop (25 FPS) with a game state struct and modular update/render functions. The main game loop runs in `gameLoop()` (40ms ticker) while user input is handled in a separate goroutine via `inputLoop()`, both synchronized under a mutex lock.

### Core Files

**types.go** — Type Definitions
- `Game`: Main game state struct containing all entities (helicopter, carrier, boats, factories, etc.), camera position, world dimensions, and status flags
- `Helicopter`: Player-controlled aircraft with position, velocity, armor, fuel, ammo, and state (landed/airborne/respawning)
- `Carrier`: Player's mothership with health and missile cooldown
- `Boat`: Enemy gunboat with position, velocity, health, fire cooldown, **`PatrolMinX`** (dynamic patrol boundary), missile cooldown, and sinking state
- `Factory`: Enemy fortress with position, health, fire cooldown, drone reserves, and sinking state
- `Missile`, `Bullet`: Projectiles with position, velocity, source (enemy/player), and active state
- `Drone`, `Tank`, `StaticAA`: Additional enemy units with their specific attributes
- `Island`: Static coastline geometry marker
- `Explosion`: Temporary visual effect with age counter

**game.go** — Game Initialization & Main Loop
- `New(screen)`: Initializes the game world with:
  - World dimensions (2x screen size)
  - Carrier position (upper left)
  - 3 boats positioned at coastline with high initial missile cooldowns (1500/2000/2500 ticks)
  - 3 factories spread across eastern island (worldWidth*2/3, worldWidth-35, worldWidth-15)
  - Camera centered on helicopter
  - Boat `PatrolMinX` set to `coastlineX - 10` (initial coast-hugging patrol)
- `Run()`: Starts game loop goroutine and blocks on input loop
- `gameLoop()`: Runs physics updates and rendering at 25 FPS with locked mutex
- `inputLoop()`: Handles keyboard events (arrow keys, space, q, etc.) and routes to game state

**input.go** — Player Input & Helicopter Control
- `updateHelicopter()`: Processes keyboard input for helicopter movement (8 directions), fire (space), and missile launch (M key)
- Directional input adjusts helicopter velocity with momentum
- Landing pad collision detection handles automatic refueling/rearming
- Respawn logic handles explosion sequence and re-materialization on carrier pad

**physics.go** — World State Updates
- `updatePhysics()`: Master update function called once per frame, orchestrates all entity updates
- `updateHelicopter()`: Helicopter movement, fuel consumption, takeoff delay
- `updateBoats()`: Boat movement along coastline with `PatrolMinX` boundary, gradual patrol expansion (0.02/tick), missile firing at carrier
- `updateLandForces()`: Orchestrates factory, tank, static AA, and drone updates
- `updateProjectiles()`: Moves bullets and missiles, handles out-of-bounds culling
- `updateMissileGuidance()`: Guided missile tracking toward target
- `updateCarrierDefense()`: Carrier fires back at nearby boats
- `updateCollisions()`: Collision detection for bullets, missiles, explosions (11 different collision types)
- `checkWaveCompletion()`: Detects all-assets-destroyed, increments wave counter, triggers **explosion sound**, resets boat `PatrolMinX` to `coastlineX - 18`, resets all cooldowns and speeds
- `getLockedTarget()`: Determines current locked target (boat/factory/tank/static AA) based on helicopter position and direction

**enemies.go** — Enemy AI & Behavior
- `updateBoats()`: 
  - Handles boat movement with velocity-based patrol
  - Checks boundaries against `PatrolMinX` (left) and coastline (right), reverses velocity on bounce
  - **Patrol expansion**: Each tick, decrements `PatrolMinX -= 0.02` until it reaches 6
  - AA fire: projectile launch toward helicopter within range with cooldown throttling
  - Guided missile launch at carrier with cooldown reset to 600-1000 ticks
- `updateFactories()`:
  - Factory AA fire similar to boats
  - **Wave 4+**: Ground-launched missiles at carrier with periodic launch (800 tick interval, staggered per factory)
  - Drone management and sinking/explosion sequences
- `updateTanks()`: Patrol within assigned bounds, AA fire toward helicopter
- `updateStaticAAs()`: Fire from fixed coastline positions toward helicopter

**collision.go** — Collision Detection & Damage
- 11 distinct collision types handled:
  - Helicopter vs bullets: armor damage
  - Helicopter vs missiles: armor damage + respawn sequence
  - Missiles vs boats/factories/tanks: explosive damage + sinking sequence
  - Player missiles vs enemy missiles: mutual destruction (countermeasure)
  - Various environmental collisions
- Each collision triggers appropriate damage model, sinking timer (if applicable), and explosion particle effects
- Carrier collision: incoming missiles trigger **warning sound**

**draw.go** — Rendering & HUD
- `draw()`: Master render function
  - Clears screen and draws ocean/island background with procedural texture
  - Camera culling and world-to-screen coordinate transformation
  - Renders entities in layers (background, water, island, enemies, bullets, missiles, explosions, helicopter, UI)
  - Collision debugging overlay (if enabled)
- `drawHUD()`: 
  - **Row H-3**: Flight instruments (GPS, speed, heading, altitude, fuel, missile ammo)
  - **Row H-2**: Status metrics (flight status, landing alignment, **boats remaining**, **factories remaining**, target lock, carrier health)
  - **Row H-1**: Control instructions
  - Incoming missile warning alert with flashing animation
- `drawString()`: Utility for rendering text at specific screen coordinates
- Color coding for status indicators (green=good, yellow=caution, red=critical)

**sound.go** — Audio Synthesis & Playback
- `InitSound()`: Initialize tcell speaker with 44.1kHz sample rate, fallback to silent mode if unavailable
- `PlaySound(soundType)`: 
  - Validates sound is enabled
  - Rate-limits: max once per 60ms per sound type (prevents clipping in intense combat)
  - Creates and plays synthesized audio stream
- `SynthSound`: Stream generator implementing audio synthesis
  - **Warning**: 500ms pulsed alarm (880/660Hz alternating at 8Hz with 4Hz square-wave gating)
  - **Laser**: 55ms sharp burst (320Hz swept to 60Hz with exponential decay)
  - **Missile**: 400ms whoosh (broadband noise + 200-320Hz tone swept)
  - **Explosion**: 800ms deep boom (30Hz sub-bass + noise with fast initial punch then slow rumble)

### Data Flow
1. **Input**: `inputLoop()` → keyboard events → game state mutation
2. **Physics**: `gameLoop()` calls `updatePhysics()` → all entities update their state
3. **Collision**: `updateCollisions()` → damage/destruction → particle effects + sound playback
4. **Wave Logic**: `checkWaveCompletion()` → all assets destroyed → wave++ → reset state with new difficulty
5. **Render**: `gameLoop()` calls `draw()` → screen update with current state

### Threading & Synchronization
- Main mutex `g.mu` protects all game state
- Input loop and game loop both acquire lock before state mutation
- Screen updates happen under lock to prevent tearing
- Channels used only for quit signal (non-blocking)

## Difficulty Progression

### Wave System
- Each wave resets all enemy assets (boats, factories, tanks, static AAs)
- Boats and factories respawn with harder AI and increased speed
- Wave completion triggers an explosion sound effect
- Tanks activate starting at Wave 2
- Factories launch ground missiles starting at Wave 4

### Boat Patrol Mechanics

**Initial State (Game Start)**
- 3 boats spawn near the coastline with extended missile cooldowns (1500/2000/2500 ticks = 60/80/100 seconds)
- Each boat has a `PatrolMinX` field defining their westernmost patrol boundary
- Initial `PatrolMinX = startingCoastlineX - 10` keeps boats clustered near the shore

**Progressive Threat Escalation**
- Each game tick, boats' `PatrolMinX` decreases by 0.02 units
- This allows boats to gradually expand their patrol range westward toward the carrier (~30 units per minute)
- Boats reach full patrol range (hardcoded minimum of 6) in approximately 3-4 minutes
- Provides new players breathing room while maintaining threat escalation

**Wave Reset**
- On wave completion, boats reset to `PatrolMinX = coastlineX - 18`
- Creates a wider initial patrol zone for subsequent waves (but still coast-restricted)
- Missile cooldown resets to 600-1000 ticks (24-40 seconds) for balanced progression

### Factory Positioning
- 3 factories spread dramatically across the eastern island:
  - Factory 0 (north): `worldWidth * 2/3` (middle-east area)
  - Factory 1 (center): `worldWidth - 35` (eastern area)
  - Factory 2 (south): `worldWidth - 15` (far eastern edge)
- Spread ~50+ units apart to utilize the expanded 2x world size
- Players must navigate across more terrain to complete an assault

## HUD Display

**Row H-3: Flight Instruments**
- GPS coordinates, speed (knots), heading (degrees), altitude (feet), fuel percentage
- Missile ammo represented as visual icons (▲ for loaded, · for empty)

**Row H-2: Status Metrics**
- Flight status (AIRBORNE/LANDED/REFUELING/OUT OF FUEL) with background color coding
- Landing pad alignment indicator (READY/NO)
- Active boats remaining (light cyan)
- Active factories remaining (orange)
- Current target lock status (BOAT/FACTORY/TANK/STATIC AA with drone count if factory)
- Carrier health with percentage bar

**Row H-1: Control Instructions**
- Keyboard/input reminders

## Audio System

**Sound Effects**
- **Warning**: 500ms pulsed beep (880/660Hz alternating, 4Hz gating) - incoming missile alert
- **Laser**: 55ms sharp burst (320Hz sweep, exponential decay) - cannon fire
- **Missile**: 400ms whoosh (broadband noise with 200-320Hz tone) - missile launch
- **Explosion**: 800ms deep boom (30Hz sub-bass + noise) - wave completion or impact

**Sound Initialization**
- `InitSound()` called on game startup in cmd/gobungle/main.go
- Gracefully disables audio if speaker initialization fails (silent mode fallback)
- Rate limiting prevents sound clipping: same sound type max once per 60ms

## World Layout

**Dimensions**
- `worldWidth = screenWidth * 2` (minimum 80)
- `worldHeight = (playableHeight) * 2` (screen height minus 4 for HUD)
- Camera follows helicopter with dead-zone scrolling (stays centered when possible, clamps at world edges)

**Entities**
- Carrier: placed at `(worldWidth/10, worldHeight/4)` - upper left safe zone
- Boats: 3 initial boats patrolling along coastline at various Y rows
- Factories: spread across eastern island area
- Tanks: 3 mobile units, activate Wave 2+, patrol interior routes
- Static AAs: distributed along coastline
- Drones: orbit factories and carrier for defense


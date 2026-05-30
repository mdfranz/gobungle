# Known Bugs & Issues

## Critical

### 1. Data Race: Concurrent goroutine access with no synchronization
**File:** `main.go:187` / `physics.go` / `main.go:229`

`gameLoop` runs as a goroutine and calls `updatePhysics()` which reads/writes `g.heli`, `g.bullets`, and `g.missiles`. `inputLoop` runs on the main goroutine and calls `handleKeyPress()` which also writes `g.heli.VX/VY/Dir` and appends to `g.bullets`/`g.missiles`. No mutex or channel mediates access. Under rapid key input, a concurrent append to `g.bullets` while `updatePhysics` iterates it can corrupt the slice header. Detectable with `go build -race`.

---

## High

### 2. Multi-hit bullet: dead bullet falls through to hit drones, factories, and tanks
**File:** `physics.go:969`

After `bullet.Active = false` and `break` exits the boat collision loop, execution falls through to the drone loop (line 1006), factory loop (line 1021), and tank loop (line 1040). None of those loops re-check `bullet.Active` before their collision test. A single bullet can eliminate a boat and a nearby drone in the same tick.

### 3. `drawString` uses UTF-8 byte index as screen column offset
**File:** `draw.go:768`

```go
for i, r := range str {
    g.screen.SetContent(x+i, y, r, nil, style)
}
```

`range` over a string yields `(byte_offset, rune)`. Emoji like 🚁 (4 bytes) and ⚠️ (6 bytes) cause `i` to jump by 3–5 extra columns. Characters after the emoji are shifted right, corrupting the HUD title (line 555) and incoming missile warning (line 566). Fix: use a separate rune counter instead of `i`.

### 4. Shared missile pool: enemy missiles count against player's 2-missile limit
**File:** `physics.go:1407`

`activeMissilesCount` iterates all `g.missiles` regardless of `IsEnemy`. Two concurrently active boat missiles satisfy `activeMissilesCount >= 2`, silently blocking the player from firing guided missiles — exactly when they most need to. Player and enemy missiles should use separate counts.

### 5. Missile dodge `continue` scans the next player missile instead of the next bullet
**File:** `physics.go:1222`

On a successful dodge (`rand.Float64() < MissileDodgeChance`), `continue` advances `j` (missile index) rather than `i` (bullet index). The same enemy CIWS bullet continues checking subsequent player missiles. The first missile that fails its dodge roll is destroyed. A single countermeasure bullet can scan through all active player missiles and kill one. Should be `break` (or `goto nextBullet`) to stop checking after a dodge.

---

## Medium

### 6. Boat and tank speed scaling is unbounded across wave resets
**File:** `physics.go:1273` (boats), `physics.go:1312–1316` (tanks)

Each wave completion multiplies `VX`/`VY` by `1.25` with no ceiling. Boat 2 (initial `VX=0.06`) reaches ~3.5 cells/tick after 20 waves, exceeding the screen width per tick. The single-step boundary clamp cannot correct an overshoot spanning the entire patrol range, causing boats/tanks to clip through the coastline or teleport. Add a max-speed cap (e.g. `math.Min(math.Abs(boat.VX)*1.25, 2.0)`).

### 7. Enemy missile slice grows without bound over long sessions
**File:** `physics.go:569`

Player bullets are capped at 24 entries (lines 523, 666, 840) but enemy boat missiles append without any length cap. With 3 boats firing every 600–1000 ticks, the `g.missiles` slice accumulates hundreds of dead entries over a long session. Every physics tick iterates the full slice for homing, drone interception, and collision checks — per-tick cost grows linearly. Add a cap analogous to the bullet cap, or compact the slice periodically.

---

## Low

### 8. `resetRound` hard-codes boat positions and speeds as duplicated literals
**File:** `physics.go:1606`

Reset values (`X=15/20/25`, `VX=0.05/0.04/0.06`) are copied from `main.go:77–79` and will silently diverge if changed there. The sign-preservation logic (lines 1621–1625) also reads the current `VX` sign at reset time; if a boundary bounce just flipped the sign in the same tick, the boat is reset pointing into the wall it just left, causing an immediate double-bounce. Store initial state in a config struct and restore from it.

### 9. `getLockedTarget()` called twice per frame (input path + draw path)
**File:** `draw.go:695`

`handleKeyPress` calls `getLockedTarget()` to validate missile lock (physics.go:1400). `drawHUD` calls it again to display the HUD indicator (draw.go:695). This iterates all boats, factories, and tanks twice per frame. With the data race present (finding #1), the two calls happen in different goroutines and can return inconsistent results — the displayed lock target may differ from the one the missile actually homes toward. Cache the result in the `Game` struct each tick.

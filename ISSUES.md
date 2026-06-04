# Project Issues

This file tracks both functional bugs and structural/refactoring concerns.

## 1. Functional Bugs & Defects

### Open

*(none)*

---

### Fixed (for reference)

#### ~~Data Race: Concurrent goroutine access with no synchronization~~
**Fixed:** `gameLoop` and `inputLoop` both acquire `g.mu` before touching game state.

#### ~~Multi-hit bullet: dead bullet falls through to hit drones, factories, and tanks~~
**Fixed:** `checkPlayerBulletVsTargets` uses `return` after each hit, so a bullet can only register one collision per tick.

#### ~~`drawString` uses UTF-8 byte index as screen column offset~~
**Fixed:** `drawString` uses a separate `col` counter incremented per rune, not the byte offset from `range`.

#### ~~Shared missile pool: enemy missiles count against player's 2-missile limit~~
**Fixed:** `input.go` filters `!g.missiles[i].IsEnemy && !g.missiles[i].IsCarrier` before counting active player missiles.

#### ~~Missile dodge `continue` scans the next player missile instead of the next bullet~~
**Fixed:** The dodge path in `collision.go` uses `break` to stop checking after a dodge attempt.

#### ~~`resetRound` hard-codes boat positions and speeds as duplicated literals~~
**Fixed:** `resetRound` restores boat position, health, and velocity from the `initialBoats` snapshot (`physics.go`). No magic literals remain.

#### ~~Boat and tank speed scaling is unbounded across wave resets~~
**Fixed:** `checkWaveCompletion` in `physics.go` caps boat `VX` and tank `VY`/`VX` at ±2.0 after each wave multiplier.

#### ~~Enemy missile slice grows without bound over long sessions~~
**Fixed:** All missile spawns now go through `appendMissile` in `projectiles.go`, which caps the slice at 16 entries.

#### ~~`getLockedTarget()` called twice per frame~~
**Fixed:** `updatePhysics` caches the result in `g.lockedBoat`, `g.lockedFactory`, `g.lockedTank`, `g.lockedStaticAA`; draw and input code reads from the cache.

---

## 2. Refactoring & Technical Debt

Code review findings ranked by maintenance impact. All confirmed via static analysis of the codebase.

---

### ~~2.1 Drone Spawn Pattern Duplicated (2 Sites)~~ — RESOLVED

**Fixed:** `appendDrone(Drone)` added to `projectiles.go` alongside `appendBullet`/`appendMissile`. Both inline `spawned := false` loops replaced in `enemies.go` (factory replenishment) and `physics.go` (carrier replenishment).

---

### 2.2 Sinking/Death Sequence Duplicated Across 4 Enemy Types — RESOLVED

**Already implemented** (predates this review). `tickSinking()` lives in `projectiles.go:79` and is called by all four enemy types — boats (`enemies.go:18`), factories (`:110`), tanks (`:231`), and static AAs (`:286`). The actual signature carries explosion scatter/grid/age parameters rather than an `onDone` closure:

```go
func (g *Game) tickSinking(timer *int, x, y float64, scatterX, scatterY, gridW, gridH, maxFinalAge int) bool
```

Factory-specific drone cleanup is handled by the caller in `updateFactories` after `tickSinking` returns true. No action needed.

---

### ~~2.3 HUD Row H-2 Uses Fragile Manual Offset Accumulation~~ — RESOLVED

**Fixed:** `drawHUDStat(x, y, label, value, labelStyle, valueStyle) int` added to `draw.go`. ALIGN, BOATS, FACTORIES, and LOCK entries now call it; `offset +=` arithmetic eliminated.

---

### ~~2.4 `boatsSunk` Is Dead State~~ — RESOLVED

**Fixed:** Field removed from `game.go`, both increment sites removed from `enemies.go` and `collision.go`, reset removed from `physics.go`.

---

### ~~2.5 Magic Constant `6` (World Water Left Edge)~~ — RESOLVED

**Fixed:** `const waterMinX = 6.0` added to `enemies.go`; all three bare `6` references replaced.

---

### 2.6 Boat `PatrolMinX` Initialization Duplicated Between `New()` and `checkWaveCompletion()`

**Files:** `game.go:151`, `physics.go:450`

Both functions call `getCoastlineThreshold(by)` and subtract a magic offset to set `PatrolMinX`, but with different offsets (`-10` for wave 1, `-18` for subsequent waves) and no shared logic:

```go
// game.go
g.boats[i].PatrolMinX = g.boats[i].X - 10.0

// physics.go
thresh := g.getCoastlineThreshold(by)
g.boats[i].PatrolMinX = thresh - 18.0
```

**Fix:** Extract a helper that takes the wave number and returns the appropriate initial patrol boundary, making the wave-based ramp-up explicit and centralized:
```go
func (g *Game) initialPatrolMinX(boatY float64, wave int) float64
```

---

### 2.7 Per-Enemy-Class Target Iteration Duplicated Across 4 Functions

**Files:** `input.go` (`getLockedTarget`), `projectiles.go` (`homeMissileToTarget`), `collision.go` (`checkPlayerBulletVsTargets`, `checkPlayerMissileVsTargets`)

Each of these functions walks boats → factories → tanks → staticAAs with near-identical per-candidate logic (skip if `!Active || SinkingTimer > 0`, compute distance, compare). Adding a fifth enemy class means editing all four functions, and the four copies can silently drift (e.g. a new hitbox rule applied in one but not another).

**Fix:** Introduce a `Targetable` abstraction — an interface (position, active/sinking state, hitbox half-extents, damage application) or a unified `[]target` rebuilt each tick — so the four call sites iterate one collection. **Medium effort:** touches collision, input, and projectile code together, so not a cheap win.

---

### 2.8 Global Mutex Held for the Entire `draw()` Call

**File:** `game.go` (`gameLoop`)

`gameLoop` holds `g.mu` across both `updatePhysics()` **and** `draw()`. `draw()` is read-only (verified — no `g.*` mutations) but spans ~1263 lines of rendering, so per-frame input latency in `inputLoop` is bounded by full render time every tick.

**Fix:** Snapshot the state `draw` needs under the lock, then render outside it — or accept the coupling, which is harmless at 25 FPS in a terminal. **Medium effort:** `draw` reads many slices, so a correct, race-free snapshot is non-trivial.

---

### Resolved

Completed 2026-06-03 (branch `scrolling`). All behavior-preserving; `go build`, `go vet`, and `gofmt` clean.

- **Camera-clamp logic duplicated 4×** — the identical world-bounds clamp in `updateCamera`, the respawn path, and `resetRound` collapsed into `clampCamera()`, plus `centerCameraOnPad(padX, padY)` for the two pad-recenter sites. The `New()` site was left as-is (genuinely different: pre-construction locals with a `playableHeight` floor of 10).
- **Wave-reset / round-reset duplication** — extracted `resetFactories()`, `resetDrones()`, and `resetStaticAAs(active bool)`, now shared by `checkWaveCompletion` and `resetRound`. Tank resets were deliberately **not** merged: wave reset scales speed ×1.25 while round reset restores spawn positions/velocities — genuinely different logic, not duplication.
- **Carrier-drone replenishment misplaced** — the ~50-line block moved out of `updateHelicopter()`'s landed branch into `replenishCarrierDrones()`, invoked from `updateCarrierDefense()`. Still gated to the landed state on the same 100-tick cadence; within-tick ordering unchanged.

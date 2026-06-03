# Project Issues

This file tracks both functional bugs and structural/refactoring concerns.

## 1. Functional Bugs & Defects

### Open

#### 1.1 `resetRound` hard-codes boat positions and speeds as duplicated literals
**File:** `physics.go` (`resetRound` function)

Reset values (`X=15/20/25`, `VX=0.05/0.04/0.06`) are copied from `game.go` boat initialization and will silently diverge if the initial values change. The sign-preservation logic also reads the current `VX` sign at reset time; if a boundary bounce just flipped the sign in the same tick, the boat resets pointing into the wall it just left, causing an immediate double-bounce. Restore from the `initialBoats` snapshot already stored in the `Game` struct instead.

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

### 2.1 Drone Spawn Pattern Duplicated (2 Sites)

**Files:** `enemies.go:180` (factory drone replenishment), `physics.go:322` (carrier drone replenishment in `replenishCarrierDrones`)

> **Scope reduced.** Bullet and missile spawning no longer use the inline pattern — they go through `spawnEnemyBullet`/`spawnEnemyMissile`/`spawnPlayerBullet`/`spawnPlayerMissile`/`spawnCarrierMissile`/`spawnCountermeasureBullet`, which wrap the shared `appendBullet`/`appendMissile` allocator in `projectiles.go`. Only **drone** spawning still inlines the "find inactive slot or append" loop.

```go
spawned := false
for d := 0; d < len(g.drones); d++ {
    if !g.drones[d].Active {
        g.drones[d] = Drone{...}
        spawned = true
        break
    }
}
if !spawned {
    g.drones = append(g.drones, Drone{...})
}
```

**Fix:** Add an `appendDrone(Drone)` helper alongside `appendBullet`/`appendMissile` in `projectiles.go` (or a `spawnDrone` wrapper) and call it from both sites.

---

### 2.2 Sinking/Death Sequence Duplicated Across 4 Enemy Types — RESOLVED

**Already implemented** (predates this review). `tickSinking()` lives in `projectiles.go:79` and is called by all four enemy types — boats (`enemies.go:18`), factories (`:110`), tanks (`:231`), and static AAs (`:286`). The actual signature carries explosion scatter/grid/age parameters rather than an `onDone` closure:

```go
func (g *Game) tickSinking(timer *int, x, y float64, scatterX, scatterY, gridW, gridH, maxFinalAge int) bool
```

Factory-specific drone cleanup is handled by the caller in `updateFactories` after `tickSinking` returns true. No action needed.

---

### 2.3 HUD Row H-2 Uses Fragile Manual Offset Accumulation (6 Repetitions)

**File:** `draw.go:796`

Every metric in the HUD status row manually tracks its x-position:
```go
offset += len(boatsLabel) + len(boatsValStr)
// ... then next metric ...
offset += len(factoriesLabel) + len(factoriesValStr)
// ... repeated 6 times
```

Any label string change silently shifts all subsequent metrics. Reordering or inserting a metric requires updating all downstream offset math.

**Fix:** Extract a helper:
```go
func (g *Game) drawHUDStat(x, y int, label, value string, labelStyle, valueStyle tcell.Style) int {
    g.drawString(x, y, label, labelStyle)
    g.drawString(x+len(label), y, value, valueStyle)
    return x + len(label) + len(value)
}
// Usage: offset = g.drawHUDStat(offset, hudY+2, "   |   BOATS: ", boatsValStr, hudStyle, cyanStyle)
```

---

### 2.4 `boatsSunk` Is Dead State — Tracked But Never Displayed

**File:** `game.go:37`, incremented at `enemies.go:32` and `collision.go:136`, reset at `physics.go:576`

The `boatsSunk` counter was previously shown in the HUD ("BOATS SUNK: N") but that display was removed when the HUD was updated to show live counts instead. The field and both increment sites were not cleaned up.

**Fix:** Remove the `boatsSunk` field from the `Game` struct, and remove the two increment sites and the reset in `resetRound()`. If a cumulative kill count is wanted in the future, re-add it intentionally.

---

### 2.5 Magic Constant `6` (World Water Left Edge) Should Be Named

**File:** `enemies.go:74,77`

The value `6` appears twice in patrol boundary logic with no explanation:
```go
if boat.PatrolMinX > 6 {
    boat.PatrolMinX -= 0.02
    if boat.PatrolMinX < 6 {
        boat.PatrolMinX = 6
    }
}
```

This represents the minimum X coordinate boats can reach (water left edge, just clear of the world boundary). It is separate from the carrier position and the coastline threshold, and its meaning is not obvious.

**Fix:**
```go
const waterMinX = 6.0
```

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

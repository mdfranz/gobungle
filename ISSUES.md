# Refactoring Issues

Code review findings ranked by maintenance impact. All confirmed via static analysis of the codebase.

---

## 1. Missile/Bullet Spawn Pattern Duplicated 10 Times

**Files:** `enemies.go` (7×), `physics.go` (2×), `projectiles.go` (1×)  
**First occurrence:** `enemies.go:94`

The "find inactive slot or append" pattern is repeated verbatim across every weapon-firing entity:

```go
spawned := false
for k := 0; k < len(g.missiles); k++ {
    if !g.missiles[k].Active {
        g.missiles[k] = Missile{...}
        spawned = true
        break
    }
}
if !spawned && len(g.missiles) < 16 {
    g.missiles = append(g.missiles, Missile{...})
}
```

**Sites:** boat AA bullets, boat missiles, factory AA bullets, factory missiles, tank AA bullets, static AA bullets, factory drone spawning, carrier drone replenishment, carrier defense missiles, CIWS countermeasure bullets.

**Fix:** Extract helpers:
```go
func spawnBullet(g *Game, x, y, vx, vy float64, isEnemy bool)
func spawnMissile(g *Game, x, y, vx, vy float64, isEnemy bool)
```

---

## 2. Sinking/Death Sequence Duplicated Across 4 Enemy Types (~200 Lines)

**File:** `enemies.go`  
**Lines:** boats 17–46, factories 161–199, tanks 334–361, staticAAs 426–453

All four enemy types implement the same 3-step pattern:
1. Decrement `SinkingTimer`
2. Every 3 ticks: spawn small random explosion particles
3. When timer hits 0: disable entity, log, spawn final explosion grid

Differences are only in explosion grid dimensions and factory-specific drone cleanup.

**Fix:** Extract a helper:
```go
func (g *Game) tickSinking(timer *int, x, y float64, gridW, gridH int, onDone func()) bool
```
The `onDone` closure handles entity-specific teardown (drone disable for factories).

---

## 3. HUD Row H-2 Uses Fragile Manual Offset Accumulation (6 Repetitions)

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

## 4. `boatsSunk` Is Dead State — Tracked But Never Displayed

**File:** `game.go:37`, incremented at `enemies.go:32` and `collision.go:136`, reset at `physics.go:576`

The `boatsSunk` counter was previously shown in the HUD ("BOATS SUNK: N") but that display was removed when the HUD was updated to show live counts instead. The field and both increment sites were not cleaned up.

**Fix:** Remove the `boatsSunk` field from the `Game` struct, and remove the two increment sites and the reset in `resetRound()`. If a cumulative kill count is wanted in the future, re-add it intentionally.

---

## 5. Magic Constant `6` (World Water Left Edge) Should Be Named

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

## 6. Boat `PatrolMinX` Initialization Duplicated Between `New()` and `checkWaveCompletion()`

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

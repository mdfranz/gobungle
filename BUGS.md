# Known Bugs & Issues

## Open

### 1. `resetRound` hard-codes boat positions and speeds as duplicated literals
**File:** `physics.go` (`resetRound` function)

Reset values (`X=15/20/25`, `VX=0.05/0.04/0.06`) are copied from `game.go` boat initialization and will silently diverge if the initial values change. The sign-preservation logic also reads the current `VX` sign at reset time; if a boundary bounce just flipped the sign in the same tick, the boat resets pointing into the wall it just left, causing an immediate double-bounce. Restore from the `initialBoats` snapshot already stored in the `Game` struct instead.

---

## Fixed (for reference)

### ~~Data Race: Concurrent goroutine access with no synchronization~~
**Fixed:** `gameLoop` and `inputLoop` both acquire `g.mu` before touching game state.

### ~~Multi-hit bullet: dead bullet falls through to hit drones, factories, and tanks~~
**Fixed:** `checkPlayerBulletVsTargets` uses `return` after each hit, so a bullet can only register one collision per tick.

### ~~`drawString` uses UTF-8 byte index as screen column offset~~
**Fixed:** `drawString` uses a separate `col` counter incremented per rune, not the byte offset from `range`.

### ~~Shared missile pool: enemy missiles count against player's 2-missile limit~~
**Fixed:** `input.go` filters `!g.missiles[i].IsEnemy && !g.missiles[i].IsCarrier` before counting active player missiles.

### ~~Missile dodge `continue` scans the next player missile instead of the next bullet~~
**Fixed:** The dodge path in `collision.go` uses `break` to stop checking after a dodge attempt.

### ~~Boat and tank speed scaling is unbounded across wave resets~~
**Fixed:** `checkWaveCompletion` in `physics.go` caps boat `VX` and tank `VY`/`VX` at ±2.0 after each wave multiplier.

### ~~Enemy missile slice grows without bound over long sessions~~
**Fixed:** All missile spawns now go through `appendMissile` in `projectiles.go`, which caps the slice at 16 entries.

### ~~`getLockedTarget()` called twice per frame~~
**Fixed:** `updatePhysics` caches the result in `g.lockedBoat`, `g.lockedFactory`, `g.lockedTank`, `g.lockedStaticAA`; draw and input code reads from the cache.

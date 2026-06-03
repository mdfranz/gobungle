# Gobungle Code Path Diagrams

Visual companion to [IMPL.md](./IMPL.md). Each Mermaid diagram traces an actual
control-flow path through the `internal/game` package. File/function references
are accurate as of the `scrolling` branch.

---

## 1. Threading Model & Top-Level Loop

Two goroutines share all state behind a single mutex (`g.mu`). The render/physics
loop is driven by a 40 ms ticker (25 FPS); the input loop blocks on tcell event
polling. Both acquire the lock before touching game state.

```mermaid
flowchart TB
    subgraph main["main goroutine — Run()"]
        run["Run()"] --> spawn["go gameLoop()"]
        run --> input["inputLoop() (blocks)"]
    end

    subgraph game["gameLoop goroutine (game.go)"]
        ticker["40ms ticker.C"] --> lockG["g.mu.Lock()"]
        lockG --> size["screen.Size()"]
        size --> guard{"!quitConfirming<br/>&& !gameOver?"}
        guard -- yes --> phys["updatePhysics()"]
        guard -- no --> draw["draw()"]
        phys --> draw
        draw --> unlockG["g.mu.Unlock()"]
        unlockG --> ticker
        quitG["&lt;-g.quit"] --> retG["return"]
    end

    subgraph in["inputLoop goroutine (game.go)"]
        poll["screen.PollEvent()"] --> evtype{"event type"}
        evtype -- EventResize --> resize["Sync + update size<br/>(under lock)"]
        evtype -- EventKey --> lockI["g.mu.Lock()"]
        lockI --> over{"gameOver?"}
        over -- yes --> closeq["close(g.quit) → return"]
        over -- no --> qconf{"quitConfirming?"}
        qconf -- yes --> ynkey{"y/n?"}
        ynkey -- y --> closeq
        ynkey -- n --> clearq["quitConfirming=false"]
        qconf -- no --> esc{"Esc / Ctrl-C?"}
        esc -- yes --> setq["quitConfirming=true"]
        esc -- no --> hkp["handleKeyPress(ev)"]
        resize --> poll
        clearq --> poll
        setq --> poll
        hkp --> poll
    end

    spawn -.-> ticker
    input -.-> poll
    closeq -.->|signals| quitG
```

---

## 2. Physics Update Pipeline — `updatePhysics()`

Called once per tick. The order matters: cooldowns and movement update before
collisions are resolved, and target lock is computed last so the HUD and the next
input frame see a fresh lock.

```mermaid
flowchart TD
    start["updatePhysics()<br/>(physics.go)"] --> t["Ticks++"]
    t --> heli["updateHelicopter()"]
    heli --> cam["updateCamera()"]
    cam --> cd["updateWeaponCooldowns()"]
    cd --> carrier["updateCarrierDefense()<br/>(+ replenishCarrierDrones when landed)"]
    carrier --> proj["updateProjectiles()"]
    proj --> boats["updateBoats()"]
    boats --> land["updateLandForces()"]
    land --> expl["updateExplosions()"]
    expl --> coll["checkCollisions()"]
    coll --> wave["checkWaveCompletion()"]
    wave --> lock["getLockedTarget()<br/>→ lockedBoat/Factory/Tank/StaticAA"]

    proj --> proj1["updateBullets()"]
    proj --> proj2["updateMissiles()"]

    land --> l1["updateFactories()"]
    land --> l2["updateDroneOrbits()"]
    land --> l3["updateTanks()"]
    land --> l4["updateStaticAAs()"]
```

---

## 3. Helicopter State Machine — `updateHelicopter()`

The helicopter lives in one of three states. Transitions are driven by fuel,
collisions (which set `RespawnTimer`/`Armor`), and proximity to the carrier pad.

```mermaid
stateDiagram-v2
    [*] --> Landed: New() spawns on pad

    Landed --> Airborne: takeoff key & TakeoffCooldown==0
    note right of Landed
        Refuel +0.4/tick
        Repair armor +0.5/tick
        Repair carrier +0.2/tick
        Rearm missiles → 4
        VX=VY=0
        (carrier-drone replenishment runs while
         landed, driven by updateCarrierDefense)
    end note

    Airborne --> Landed: aligned over pad & speed<0.12 & cooldown==0
    note right of Airborne
        Consume fuel -0.05/tick
        Apply velocity + drag (0.99, or 0.85 if dry)
        Clamp speed ≤ 1.2
        Bounce off world edges (×-0.4)
        Spin rotor frame
    end note

    Airborne --> Respawning: Armor≤0 (collision) OR<br/>out-of-fuel crash over ocean
    note left of Respawning
        RespawnTimer set to 40
        (or 65 if enemy missiles in flight)
        Spawn wreckage explosions every 4 ticks
    end note

    Respawning --> Landed: RespawnTimer==0<br/>re-materialize on pad,<br/>refuel/rearm, recenter camera
```

---

## 4. Player Input Routing — `handleKeyPress()`

Input is gated by helicopter state. A destroyed or respawning heli ignores all
keys; a landed heli only accepts takeoff; an airborne heli accepts steering,
thrust, cannon, and guided-missile commands.

```mermaid
flowchart TD
    key["handleKeyPress(ev)<br/>(input.go)"] --> dead{"Armor≤0 OR<br/>RespawnTimer&gt;0?"}
    dead -- yes --> ret1["return (ignore input)"]
    dead -- no --> landed{"Landed?"}

    landed -- yes --> tk{"takeoff key?<br/>(space/↑/w/l)"}
    tk -- yes & cooldown==0 --> takeoff["Landed=false<br/>VY=-0.1<br/>TakeoffCooldown=25"]
    tk -- no --> ret2["return"]

    landed -- no --> lkey{"'l' & aligned<br/>& speed&lt;0.25?"}
    lkey -- yes --> dolanding["Land: snap to pad,<br/>zero velocity"]
    lkey -- no --> fire{"space & FireCooldown==0<br/>& Fuel&gt;0?"}

    fire -- yes --> bullet["spawnPlayerBullet()<br/>PlaySound(laser)<br/>FireCooldown=4"]
    fire -- no --> msl{"f/m & MissileCooldown==0<br/>& Fuel&gt;0 & Ammo&gt;0?"}

    msl -- yes --> haslock{"any target locked?"}
    haslock -- no --> abort["abort launch (no lock)"]
    haslock -- yes --> cap{"active player<br/>missiles &lt; 2?"}
    cap -- yes --> launch["spawnPlayerMissile()<br/>Ammo--<br/>PlaySound(missile)<br/>MissileCooldown=12"]
    cap -- no --> skip["skip (max in flight)"]

    bullet --> steer["steering / thrust switch"]
    msl -- no --> steer
    steer --> sw{"key / rune"}
    sw -- "←/a" --> rotL["Dir = (Dir-1) mod 8"]
    sw -- "→/d" --> rotR["Dir = (Dir+1) mod 8"]
    sw -- "↑/w" --> thrust["VX/VY += dir·thrust"]
    sw -- "↓/s" --> brake["VX/VY *= 0.3"]
```

---

## 5. Target Lock Acquisition — `getLockedTarget()`

Each tick, scans every enemy class for the nearest active target inside the
`±45°` forward aperture (`dot ≥ 0.707`) and within `MaxLockOnRange` (100 units).
Y deltas are doubled to correct for terminal cell aspect ratio. Only one target
is locked — the last writer wins, so the global nearest survives.

```mermaid
flowchart TD
    gl["getLockedTarget()<br/>(input.go)"] --> dir["heading vector from heli.Dir<br/>(hy scaled ×2, normalized)"]
    dir --> mininit["minDist = MaxFloat64"]

    mininit --> scanB["for each Boat"]
    scanB --> scanF["for each Factory"]
    scanF --> scanT["for each Tank"]
    scanT --> scanS["for each StaticAA"]
    scanS --> result["return locked* (one non-nil)"]

    subgraph test["per-candidate test (same for all 4 classes)"]
        c1{"Active &&<br/>SinkingTimer==0?"} -- no --> nextc["skip"]
        c1 -- yes --> c2{"0 &lt; dist ≤ 100?"}
        c2 -- no --> nextc
        c2 -- yes --> c3{"dot(heading, toTarget)<br/>≥ 0.707 (±45°)<br/>&& dist &lt; minDist?"}
        c3 -- yes --> setlock["minDist = dist<br/>set this lock,<br/>clear other 3"]
        c3 -- no --> nextc
    end

    scanB -.uses.-> test
```

---

## 6. Collision Dispatch — `checkCollisions()`

Five sub-checks run in sequence. Drone interceptions run first (so a shielded
missile dies before it can hit anything), then bullets, then missiles, then the
two mutual bullet-vs-missile interception passes.

```mermaid
flowchart TD
    cc["checkCollisions()<br/>(collision.go)"] --> di["checkDroneMissileInterceptions()"]
    di --> bc["checkBulletCollisions()"]
    bc --> mc["checkMissileCollisions()"]
    mc --> pm["checkPlayerBulletsVsEnemyMissiles()"]
    pm --> em["checkEnemyBulletsVsPlayerMissiles()"]

    di --> di1{"player missile + factory drone<br/>OR enemy missile + carrier drone<br/>within 4.0?"}
    di1 -- yes --> dik["both destroyed,<br/>explosion + sound"]

    bc --> bc1{"bullet.IsEnemy?"}
    bc1 -- yes --> bce["checkEnemyBulletVsPlayer()<br/>→ heli Armor -15"]
    bc1 -- no --> bcp["checkPlayerBulletVsTargets()<br/>→ boat/drone/factory/tank/AA"]

    mc --> mc1{"missile.IsEnemy?"}
    mc1 -- yes --> mce["checkEnemyMissileVsCarrier()<br/>→ carrier Health -25"]
    mc1 -- no --> mcp["checkPlayerMissileVsTargets()<br/>→ set SinkingTimer=45"]

    pm --> pmk["manual interception:<br/>player bullet kills enemy missile"]
    em --> emk["CIWS: enemy bullet kills<br/>player missile (65% — 35% dodge)"]
```

### 6a. Player bullet damage resolution — `checkPlayerBulletVsTargets()`

Targets are tested in a fixed priority order; the first hit consumes the bullet
(`return`). Boats and factories take incremental damage; reaching 0 HP triggers
the delayed sinking sequence (factory/tank/AA) or immediate sink (boat by cannon).

```mermaid
flowchart TD
    pbt["checkPlayerBulletVsTargets(bullet)"] --> b{"hit Boat?<br/>|Δx|&lt;5.5 |Δy|&lt;1.5"}
    b -- yes --> bh["Health-- ; if ≤0 → Active=false,<br/>boatsSunk++, big explosion"] --> rb["return"]
    b -- no --> d{"hit Drone?<br/>|Δx|&lt;1.5 |Δy|&lt;1.2"}
    d -- yes --> dh["drone destroyed"] --> rd["return"]
    d -- no --> f{"hit Factory?<br/>|Δx|&lt;8.5 |Δy|&lt;2.5"}
    f -- yes --> fh["Health-- ; if ≤0 → SinkingTimer=45"] --> rf["return"]
    f -- no --> tk{"hit Tank?<br/>|Δx|&lt;2.5 |Δy|&lt;1.5"}
    tk -- yes --> th["Health-- ; if ≤0 → SinkingTimer=45"] --> rt["return"]
    tk -- no --> sa{"hit Static AA?<br/>|Δx|&lt;1.5 |Δy|&lt;1.5"}
    sa -- yes --> sah["Health-- ; if ≤0 → SinkingTimer=45"] --> rs["return"]
    sa -- no --> miss["bullet passes through"]
```

---

## 7. Projectile Lifecycle

Bullets and missiles share a pooled-slot allocator (`appendBullet`/`appendMissile`
reuse inactive slots before growing). Each tick they home (missiles only), move,
and self-cull on world-edge exit or max-range.

```mermaid
flowchart LR
    subgraph spawn["Spawn (projectiles.go)"]
        sp1["spawnPlayerBullet / spawnEnemyBullet<br/>spawnCountermeasureBullet"] --> ab["appendBullet(b, cap)"]
        sp2["spawnPlayerMissile / spawnEnemyMissile<br/>spawnCarrierMissile"] --> am["appendMissile(m)"]
        ab --> slot{"reuse inactive slot?"}
        am --> slot
        slot -- yes --> reuse["overwrite slot"]
        slot -- no --> grow["append if under cap"]
    end

    subgraph update["updateProjectiles() per tick"]
        ub["updateBullets()"] --> bmove["X+=VX, Y+=VY"]
        bmove --> bcull{"off-world OR<br/>travel &gt; range?"}
        bcull -- yes --> bdead["Active=false"]

        um["updateMissiles()"] --> mhome{"IsEnemy?"}
        mhome -- yes --> hc["homeMissileToCarrier()<br/>accel→1.1, lerp 0.92/0.08"]
        mhome -- no --> ht["homeMissileToTarget()<br/>nearest of all classes,<br/>accel→5.0, lerp 0.82/0.18"]
        hc --> mmove["X+=VX, Y+=VY"]
        ht --> mmove
        mmove --> mcull{"off-world OR<br/>travel &gt; 100?"}
        mcull -- yes --> mdead["Active=false"]
    end

    reuse -.-> ub
    grow -.-> ub
```

### 7a. Boat CIWS countermeasure (inside `homeMissileToTarget`)

When a player missile homing on a boat closes within `BoatDetectionRange` (25),
the boat rolls **once** (`InterceptionRolled`) for a 10% chance to fire a
countermeasure bullet back along the missile's bearing.

```mermaid
flowchart TD
    h["homeMissileToTarget(m)"] --> near{"target is boat &&<br/>dist &lt; 25 &&<br/>!InterceptionRolled?"}
    near -- no --> done["continue homing"]
    near -- yes --> roll["InterceptionRolled = true"]
    roll --> chance{"rand &lt; 0.10?"}
    chance -- yes --> cm["spawnCountermeasureBullet()<br/>back along bearing"]
    chance -- no --> done
```

---

## 8. Wave Progression — `checkWaveCompletion()`

A wave ends only when **all** boats, factories, tanks, and static AAs are
inactive. The reset re-arms every asset with 1.25× speed and gates harder unit
classes behind wave thresholds.

```mermaid
flowchart TD
    cw["checkWaveCompletion()<br/>(physics.go)"] --> chk["allSunk = true"]
    chk --> b{"any Boat active?"}
    b -- yes --> ret["return (wave continues)"]
    b -- no --> f{"any Factory active?"}
    f -- yes --> ret
    f -- no --> t{"any Tank active?"}
    t -- yes --> ret
    t -- no --> s{"any Static AA active?"}
    s -- yes --> ret
    s -- no --> adv["Wave++<br/>PlaySound(explosion)"]

    adv --> rb["reset boats:<br/>VX×1.25 (cap 2.0)<br/>PatrolMinX = coastline-18<br/>MissileCooldown 600-1000"]
    rb --> rf["reset factories:<br/>full HP, 8 drones"]
    rf --> rd["reset drones to orbit"]
    rd --> rt["reset tanks:<br/>Active = Wave≥2, VX/VY×1.25"]
    rt --> rs["reset static AAs:<br/>Active = Wave≥3"]
    rs --> note["Factory ground missiles<br/>unlock at Wave≥4<br/>(see updateFactories)"]
```

### 8a. Unit activation thresholds

```mermaid
timeline
    title Difficulty Gates by Wave
    Wave 1 : Boats patrol + missiles : Factories AA + drones
    Wave 2 : Tanks activate (patrol + flak)
    Wave 3 : Static AA guns activate
    Wave 4+ : Factories launch ground missiles at carrier
```

---

## 9. Carrier Destruction vs. Wave Reset

Two distinct "reset" paths exist and should not be confused:

- **`checkWaveCompletion()`** — player wins the wave; assets respawn *harder*.
- **`resetRound()`** — carrier health hits 0 mid-game; full round reset (called
  on carrier loss). Note the enemy-missile-vs-carrier path sets `gameOver = true`
  when health reaches 0, ending the session.

```mermaid
flowchart TD
    em["enemy missile hits carrier<br/>checkEnemyMissileVsCarrier()"] --> dmg["Health -= 25"]
    dmg --> zero{"Health ≤ 0?"}
    zero -- no --> cont["continue play"]
    zero -- yes --> go["gameOver = true<br/>carrier explosion burst"]
    go --> end1["inputLoop ends session<br/>on next key"]

    rr["resetRound()<br/>(carrier destruction recovery)"] -.note.-> rrn["restores carrier=100,<br/>heli on pad, clears projectiles,<br/>respawns all assets at base difficulty"]
```

---

## Cross-Reference

| Concern | Entry point | File |
| --- | --- | --- |
| Loop & threading | `Run`, `gameLoop`, `inputLoop` | `game.go` |
| Per-tick orchestration | `updatePhysics` | `physics.go` |
| Player control | `handleKeyPress`, `getLockedTarget` | `input.go` |
| Enemy AI | `updateBoats`, `updateFactories`, `updateTanks`, `updateStaticAAs` | `enemies.go` |
| Projectiles | `updateProjectiles`, `spawn*`, `homeMissile*` | `projectiles.go` |
| Damage & interception | `checkCollisions` + sub-checks | `collision.go` |
| Rendering & HUD | `draw`, `drawHUD` | `draw.go` |
| Audio | `InitSound`, `PlaySound` | `sound.go` |

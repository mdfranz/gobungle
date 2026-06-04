package game

import (
	"log/slog"
	"math"
	"math/rand"
)

const (
	waterMinX        = 6.0  // minimum X boats can reach (left edge of navigable water)
	boatMissileRange = 80.0 // minimum distance from carrier center at which boats stop advancing
)

// updateBoats moves boats and handles their AA and missile firing.
func (g *Game) updateBoats() {
	for i := 0; i < len(g.boats); i++ {
		boat := &g.boats[i]
		if !boat.Active {
			continue
		}

		if boat.SinkingTimer > 0 {
			if g.tickSinking(&boat.SinkingTimer, boat.X, boat.Y, 5, 1, 5, 1, 0) {
				g.applyBlastDamage(boat.X, boat.Y, 7.0, 20.0)
				boat.Active = false
				slog.Info("Doomed boat has fully sunk", "boat_idx", i)
				continue
			}
		}

		speedMult := 1.0
		if boat.SinkingTimer > 0 {
			speedMult = 0.25
		}
		boat.X += boat.VX * speedMult

		threshold := g.getCoastlineThreshold(int(math.Round(boat.Y)))
		maxWaterX := threshold - 7.0
		if maxWaterX > float64(g.worldWidth-7) {
			maxWaterX = float64(g.worldWidth - 7)
		}
		if boat.X < boat.PatrolMinX || boat.X > maxWaterX {
			boat.VX = -boat.VX
			boat.X += boat.VX * speedMult
			if boat.X < boat.PatrolMinX {
				boat.X = boat.PatrolMinX
			} else if boat.X > maxWaterX {
				boat.X = maxWaterX
			}
		}

		if boat.SinkingTimer > 0 {
			continue
		}

		// Advance patrol front toward carrier, stopping at missile range.
		missileStopX := float64(g.carrier.X+g.carrier.Width/2) + boatMissileRange
		if boat.PatrolMinX > missileStopX {
			boat.PatrolMinX -= 0.02
			if boat.PatrolMinX < missileStopX {
				boat.PatrolMinX = missileStopX
			}
		}

		// AA fire against the helicopter
		if g.tickAAFire(boat.X, boat.Y, &boat.FireCooldown, BoatAARange, 2.0, 60, 80) {
			slog.Info("Boat fired anti-aircraft projectile", "boat_idx", i, "x", boat.X, "y", boat.Y)
		}

		// Guided missile at carrier
		if boat.MissileCooldown > 0 {
			boat.MissileCooldown--
		} else {
			targetX := float64(g.carrier.X + g.carrier.Width/2)
			targetY := float64(g.carrier.Y + g.carrier.Height/2)
			dxVec := targetX - boat.X
			dyVec := targetY - boat.Y
			dist := math.Sqrt(dxVec*dxVec + dyVec*dyVec)
			if dist > 0 {
				speed := 0.3
				g.spawnEnemyMissile(boat.X, boat.Y, (dxVec/dist)*speed, (dyVec/dist)*speed)
				slog.Info("Boat launched guided missile at Carrier!", "boat_idx", i, "boat_x", boat.X, "boat_y", boat.Y)
				PlaySound("missile")
			}
			boat.MissileCooldown = 600 + rand.Intn(400)
		}
	}
}

// updateStealthBoats moves active stealth drone speedboats, rolls for new spawns,
// and updates the stealthNear warning flag + audio.
// Every 1500 ticks (~1 minute) there is a wave-scaled chance to queue a launch;
// the actual spawn fires after a random 1-10 second delay.
func (g *Game) updateStealthBoats() {
	const (
		minuteTicks = 1500 // 25 FPS * 60s
		ticksPerSec = minuteTicks / 60
		warnDistSq  = 71.0 * 71.0
	)

	// Roll once per minute when no boat is active and no launch is pending.
	if g.Ticks > 0 && g.Ticks%minuteTicks == 0 {
		noneActive := true
		for i := range g.stealthBoats {
			if g.stealthBoats[i].Active {
				noneActive = false
				break
			}
		}
		if noneActive && g.stealthSpawnAt == 0 {
			chance := g.Wave * 10
			if chance > 80 {
				chance = 80
			}
			if rand.Intn(100) < chance {
				delaySec := 1 + rand.Intn(10)
				g.stealthSpawnAt = g.Ticks + delaySec*ticksPerSec
			}
		}
	}

	// Fire the pending launch when its tick arrives.
	if g.stealthSpawnAt > 0 && g.Ticks >= g.stealthSpawnAt {
		g.stealthSpawnAt = 0
		g.spawnStealthBoat()
	}

	// Move boats and log any that exit the map.
	for i := range g.stealthBoats {
		sb := &g.stealthBoats[i]
		if !sb.Active {
			continue
		}
		sb.X += sb.VX
		if sb.X < 0 {
			slog.Info("Stealth drone speedboat exited map without hitting carrier", "idx", i)
			sb.Active = false
		}
	}

	// Update proximity warning flag and trigger audio from the game-logic layer.
	carrierCX := float64(g.carrier.X + g.carrier.Width/2)
	carrierCY := float64(g.carrier.Y + g.carrier.Height/2)
	g.stealthNear = false
	for i := range g.stealthBoats {
		sb := &g.stealthBoats[i]
		if !sb.Active {
			continue
		}
		dx := sb.X - carrierCX
		dy := sb.Y - carrierCY
		if dx*dx+dy*dy < warnDistSq {
			g.stealthNear = true
			break
		}
	}
	if g.stealthNear {
		if g.Ticks%20 == 0 {
			PlaySound("warning")
		}
		if g.Ticks%15 == 0 {
			PlaySound("speedboat")
		}
	}
}

func (g *Game) spawnStealthBoat() {
	carrierCY := float64(g.carrier.Y + g.carrier.Height/2)
	spawnY := carrierCY + float64(rand.Intn(7)-3)
	spawnX := g.getCoastlineThreshold(int(math.Round(spawnY))) - 3.0
	sb := StealthBoat{
		X:      spawnX,
		Y:      spawnY,
		VX:     -0.42,
		Active: true,
	}
	for i := range g.stealthBoats {
		if !g.stealthBoats[i].Active {
			g.stealthBoats[i] = sb
			slog.Warn("Stealth drone speedboat launched!", "x", spawnX, "y", spawnY, "wave", g.Wave)
			return
		}
	}
	g.stealthBoats = append(g.stealthBoats, sb)
	slog.Warn("Stealth drone speedboat launched!", "x", spawnX, "y", spawnY, "wave", g.Wave)
}

// tickAAFire decrements the cooldown and, when it expires, fires a bullet toward the
// helicopter if it is airborne and within aaRange. Returns true if a shot was fired.
func (g *Game) tickAAFire(x, y float64, cooldown *int, aaRange, speed float64, cooldownMin, cooldownRand int) bool {
	if *cooldown > 0 {
		*cooldown--
		return false
	}
	if g.heli.Landed || g.heli.Fuel <= 0 || g.heli.Armor <= 0 {
		return false
	}
	dx := g.heli.X - x
	dy := g.heli.Y - y
	dist := math.Sqrt(dx*dx + dy*dy)
	if dist <= 0 || dist >= aaRange {
		return false
	}
	g.spawnEnemyBullet(x, y, (dx/dist)*speed, (dy/dist)*speed)
	*cooldown = cooldownMin + rand.Intn(cooldownRand)
	return true
}

// updateLandForces updates factories, drone orbits, tanks, and static AA guns.
func (g *Game) updateLandForces() {
	g.updateFactories()
	g.updateDroneOrbits()
	g.updateTanks()
	g.updateStaticAAs()
}

func (g *Game) updateFactories() {
	for fIdx := range g.factories {
		fact := &g.factories[fIdx]
		if !fact.Active {
			continue
		}

		if fact.SinkingTimer > 0 {
			if g.tickSinking(&fact.SinkingTimer, fact.X, fact.Y, 3, 1, 6, 2, 4) {
				g.applyBlastDamage(fact.X, fact.Y, 9.0, 25.0)
				fact.Active = false
				slog.Info("Enemy military Factory has been completely destroyed!", "idx", fIdx)
				for d := 0; d < len(g.drones); d++ {
					if g.drones[d].Active && g.drones[d].FactoryIdx == fIdx {
						g.drones[d].Active = false
						g.explosions = append(g.explosions, Explosion{
							X:   int(math.Round(g.drones[d].X)),
							Y:   int(math.Round(g.drones[d].Y)),
							Age: 0,
						})
					}
				}
			}
		}

		// Factory AA fire
		if fact.Active && fact.SinkingTimer == 0 {
			if g.tickAAFire(fact.X, fact.Y, &fact.FireCooldown, BoatAARange, 2.0, 40, 40) {
				slog.Info("Factory fired fortress anti-aircraft projectile!", "x", fact.X, "y", fact.Y, "idx", fIdx)
			}
		}

		// Factory ground-launched missile at Carrier (Wave 4+)
		if g.Wave >= 4 && fact.Active && fact.SinkingTimer == 0 {
			if (g.Ticks+fIdx*200)%800 == 0 {
				targetX := float64(g.carrier.X + g.carrier.Width/2)
				targetY := float64(g.carrier.Y + g.carrier.Height/2)
				dxVec := targetX - fact.X
				dyVec := targetY - fact.Y
				dist := math.Sqrt(dxVec*dxVec + dyVec*dyVec)
				if dist > 0 {
					speed := 0.25
					g.spawnEnemyMissile(fact.X, fact.Y, (dxVec/dist)*speed, (dyVec/dist)*speed)
					slog.Info("Factory launched fortress ground missile at Carrier!", "factory_idx", fIdx, "fact_x", fact.X, "fact_y", fact.Y)
					PlaySound("missile")
				}
			}
		}

		// Factory drone replenishment
		if fact.Active && fact.SinkingTimer == 0 && g.Ticks%100 == 0 {
			activeCount := 0
			for d := 0; d < len(g.drones); d++ {
				if g.drones[d].Active && g.drones[d].FactoryIdx == fIdx {
					activeCount++
				}
			}

			if activeCount < 2 && fact.DronesRemaining > 0 {
				angle := 0.0
				if activeCount > 0 {
					for d := 0; d < len(g.drones); d++ {
						if g.drones[d].Active && g.drones[d].FactoryIdx == fIdx {
							angle = g.drones[d].Angle + math.Pi
							break
						}
					}
				}

				g.appendDrone(Drone{X: fact.X, Y: fact.Y, Active: true, Angle: angle, FactoryIdx: fIdx})
				fact.DronesRemaining--
				slog.Info("Factory spawned replacement defense drone!", "factory_idx", fIdx, "reserves_remaining", fact.DronesRemaining)
			}
		}
	}
}

func (g *Game) updateDroneOrbits() {
	for d := 0; d < len(g.drones); d++ {
		drone := &g.drones[d]
		if !drone.Active {
			continue
		}
		if drone.FactoryIdx >= 0 && drone.FactoryIdx < len(g.factories) {
			fact := &g.factories[drone.FactoryIdx]
			if fact.Active && fact.SinkingTimer == 0 {
				drone.Angle += 0.045
				radius := 8.0
				drone.X = fact.X + math.Cos(drone.Angle)*radius
				drone.Y = fact.Y + math.Sin(drone.Angle)*radius*0.5
			}
		} else if drone.FactoryIdx == -1 {
			drone.Angle += 0.035
			cx := float64(g.carrier.X + g.carrier.Width/2)
			cy := float64(g.carrier.Y + g.carrier.Height/2)
			radius := 12.0
			drone.X = cx + math.Cos(drone.Angle)*radius
			drone.Y = cy + math.Sin(drone.Angle)*radius*0.5
		}
	}
}

func (g *Game) updateTanks() {
	for tIdx := range g.tanks {
		tank := &g.tanks[tIdx]
		if !tank.Active {
			continue
		}

		if tank.SinkingTimer > 0 {
			if g.tickSinking(&tank.SinkingTimer, tank.X, tank.Y, 1, 1, 2, 1, 3) {
				g.applyBlastDamage(tank.X, tank.Y, 4.0, 15.0)
				tank.Active = false
				slog.Info("Patrolling Tank has fully blown up!", "tank_idx", tIdx)
				continue
			}
		}

		if tank.SinkingTimer == 0 {
			if tank.PatrolDir == 0 {
				tank.Y += tank.VY
				if tank.Y < tank.MinCoord {
					tank.Y = tank.MinCoord
					tank.VY = -tank.VY
				} else if tank.Y > tank.MaxCoord {
					tank.Y = tank.MaxCoord
					tank.VY = -tank.VY
				}
			} else {
				tank.X += tank.VX
				if tank.X < tank.MinCoord {
					tank.X = tank.MinCoord
					tank.VX = -tank.VX
				} else if tank.X > tank.MaxCoord {
					tank.X = tank.MaxCoord
					tank.VX = -tank.VX
				}
			}
		}

		if tank.Active && tank.SinkingTimer == 0 {
			if g.tickAAFire(tank.X, tank.Y, &tank.FireCooldown, 40.0, 2.2, 50, 40) {
				slog.Info("Tank fired flak projectile!", "tank_idx", tIdx, "x", tank.X, "y", tank.Y)
			}
		}
	}
}

func (g *Game) updateStaticAAs() {
	for saIdx := range g.staticAAs {
		sa := &g.staticAAs[saIdx]
		if !sa.Active {
			continue
		}

		if sa.SinkingTimer > 0 {
			if g.tickSinking(&sa.SinkingTimer, sa.X, sa.Y, 1, 1, 2, 1, 3) {
				g.applyBlastDamage(sa.X, sa.Y, 4.0, 15.0)
				sa.Active = false
				slog.Info("Static AA has fully blown up!", "idx", saIdx)
				continue
			}
		}

		if sa.Active && sa.SinkingTimer == 0 {
			if g.tickAAFire(sa.X, sa.Y, &sa.FireCooldown, 45.0, 2.2, 45, 35) {
				slog.Info("Static AA fired flak projectile!", "idx", saIdx, "x", sa.X, "y", sa.Y)
			}
		}
	}
}

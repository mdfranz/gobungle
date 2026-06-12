package game

import (
	"log/slog"
	"math"
	"math/rand"
)

const (
	MaxLockOnRange     = 100.0
	BoatDetectionRange = 25.0
	MissileDodgeChance = 0.35
	PlayerCannonRange  = 35.0
	BoatAARange        = 55.0
	MissileMaxRange    = 100.0
)

// updatePhysics is the top-level physics coordinator called once per tick.
func (g *Game) updatePhysics() {
	if g.gameOver {
		return
	}

	g.Ticks++

	// Handle carrier destruction sequence
	if g.carrierDestroying {
		g.destructionTicks--

		// Spawn secondary explosions across the deck
		if g.destructionTicks%4 == 0 {
			g.explosions = append(g.explosions, Explosion{
				X:   g.carrier.X + rand.Intn(g.carrier.Width),
				Y:   g.carrier.Y + rand.Intn(g.carrier.Height),
				Age: rand.Intn(2),
			})
			if g.destructionTicks%12 == 0 {
				PlaySound("explosion")
			}
		}

		if g.destructionTicks <= 0 {
			g.carrierDestroying = false
			g.gameOver = true
		}

		// Autopilot: fly back to watch the destruction
		g.updateHelicopter()
		g.updateExplosions()
		g.updateCamera()
		return
	}

	g.applyJoystickInput()
	g.updateHelicopter()
	g.updateCamera() // Update scroll-window camera position
	g.updateWeaponCooldowns()
	g.updateCarrierDefense()
	g.updateProjectiles()
	g.updateBoats()
	g.updateStealthBoats()
	g.updateLandForces()
	g.updateExplosions()
	g.checkCollisions()
	g.checkWaveCompletion()
	g.lockedBoat, g.lockedFactory, g.lockedTank, g.lockedStaticAA = g.getLockedTarget()
}

func (g *Game) updateHelicopter() {
	if g.heli.TakeoffCooldown > 0 {
		g.heli.TakeoffCooldown--
	}

	// Autopilot override during carrier destruction
	if g.carrierDestroying && !g.heli.Landed && g.heli.RespawnTimer == 0 {
		padX := float64(g.carrier.X + g.carrier.Width/3)
		padY := float64(g.carrier.Y + g.carrier.Height/2)

		dx := padX - g.heli.X
		dy := padY - g.heli.Y
		dist := math.Sqrt(dx*dx + dy*dy)

		if dist > 2.0 {
			// Calculate desired direction
			angle := math.Atan2(dy, dx)
			// Map angle to 8-way Dir (0-7: East, SE, South, SW, West, NW, North, NE)
			// Atan2 returns -PI to PI. 0 is East.
			deg := angle * (180.0 / math.Pi)
			if deg < 0 {
				deg += 360
			}
			g.heli.Dir = int(math.Round(deg/45.0)) % 8

			// Move faster than normal to ensure we get there
			speed := 1.2
			g.heli.VX = math.Cos(angle) * speed
			g.heli.VY = math.Sin(angle) * speed
		} else {
			g.heli.VX = 0
			g.heli.VY = 0
		}
	}

	if g.heli.RotationCooldown > 0 {
		g.heli.RotationCooldown--
	}
	if g.heli.RespawnTimer > 0 {
		g.heli.RespawnTimer--
		if g.heli.RespawnTimer%4 == 0 {
			// Spawn random small secondary explosions around the wreckage
			g.explosions = append(g.explosions, Explosion{
				X:   int(math.Round(g.heli.X)) + rand.Intn(5) - 2,
				Y:   int(math.Round(g.heli.Y)) + rand.Intn(3) - 1,
				Age: rand.Intn(3),
			})
		}
		if g.heli.RespawnTimer == 0 {
			// Complete respawn onto the carrier pad
			padX := g.carrier.X + g.carrier.Width/3
			padY := g.carrier.Y + g.carrier.Height/2
			g.heli.X = float64(padX)
			g.heli.Y = float64(padY)
			g.heli.VX = 0
			g.heli.VY = 0
			g.heli.Fuel = 100.0
			g.heli.Armor = 100.0
			g.heli.MissileAmmo = 4
			g.heli.Landed = true
			g.heli.ReturningToCarrier = false
			g.heli.TakeoffCooldown = 25

			// Re-center camera over carrier pad
			g.centerCameraOnPad(padX, padY)
		}
		return
	}
	if g.heli.Landed {
		// Refuel slowly on carrier landing pad
		if g.heli.Fuel < 100.0 {
			g.heli.Fuel += 0.4
			if g.heli.Fuel >= 100.0 {
				g.heli.Fuel = 100.0
				slog.Info("Refueling completed", "fuel", g.heli.Fuel)
			}
		}
		// Repair armor slowly on carrier landing pad
		if g.heli.Armor < 100.0 {
			g.heli.Armor += 0.5
			if g.heli.Armor >= 100.0 {
				g.heli.Armor = 100.0
				slog.Info("Repairs completed", "armor", g.heli.Armor)
			}
		}
		// Repair carrier health slowly on landing pad
		if g.carrier.Health < 100.0 {
			g.carrier.Health += 0.2
			if g.carrier.Health >= 100.0 {
				g.carrier.Health = 100.0
				slog.Info("Carrier fully repaired", "health", g.carrier.Health)
			}
		}
		// Rearm missiles to full capacity (4) on landing pad
		if g.heli.MissileAmmo < 4 {
			g.heli.MissileAmmo = 4
			slog.Info("Missiles fully rearmed", "ammo", g.heli.MissileAmmo)
		}

		g.heli.VX = 0
		g.heli.VY = 0
	} else {
		// Consume fuel while airborne
		if g.heli.Fuel > 0 {
			g.heli.Fuel -= 0.05
			if g.heli.Fuel <= 0 {
				g.heli.Fuel = 0
				slog.Warn("Engine failure: Out of fuel")
			}
		}

		drag := 0.99
		if g.heli.Fuel <= 0 {
			drag = 0.85
		}

		g.heli.X += g.heli.VX
		g.heli.Y += g.heli.VY
		g.heli.VX *= drag
		g.heli.VY *= drag

		speed := math.Sqrt(g.heli.VX*g.heli.VX + g.heli.VY*g.heli.VY)
		maxSpeed := 1.2
		if speed > maxSpeed {
			ratio := maxSpeed / speed
			g.heli.VX *= ratio
			g.heli.VY *= ratio
		}

		// Crash if out of fuel and gliding to a halt over the ocean
		if g.heli.Fuel <= 0 {
			hx := int(math.Round(g.heli.X))
			hy := int(math.Round(g.heli.Y))
			onCarrier := hx >= g.carrier.X && hx < g.carrier.X+g.carrier.Width &&
				hy >= g.carrier.Y && hy < g.carrier.Y+g.carrier.Height

			if !onCarrier && speed < 0.02 {
				slog.Warn("Helicopter crashed into the ocean: Out of fuel", "x", g.heli.X, "y", g.heli.Y)

				for ddx := -2; ddx <= 2; ddx++ {
					for ddy := -1; ddy <= 1; ddy++ {
						g.explosions = append(g.explosions, Explosion{X: hx + ddx, Y: hy + ddy, Age: 0})
					}
				}

				g.heli.Armor = 0
				hasIncoming := false
				for j := 0; j < len(g.missiles); j++ {
					if g.missiles[j].Active && g.missiles[j].IsEnemy {
						hasIncoming = true
						break
					}
				}
				g.killHeli(hasIncoming)
			}
		}

		// Autopilot: fly back to carrier after wave clear
		if g.heli.ReturningToCarrier {
			padX := float64(g.carrier.X + g.carrier.Width/3)
			padY := float64(g.carrier.Y + g.carrier.Height/2)
			toDX := padX - g.heli.X
			toDY := padY - g.heli.Y
			dist := math.Sqrt(toDX*toDX + toDY*toDY)
			if dist > 0.5 {
				bestDir := 0
				bestDot := -math.MaxFloat64
				for d := 0; d < 8; d++ {
					dot := (toDX*dx[d] + toDY*dy[d]*2.0) / (dist + 0.001)
					if dot > bestDot {
						bestDot = dot
						bestDir = d
					}
				}
				g.heli.Dir = bestDir
				thrust := 0.10
				if dist < 5.0 {
					thrust = 0.04
				}
				g.heli.VX += dx[g.heli.Dir] * thrust
				g.heli.VY += dy[g.heli.Dir] * thrust
			}
		}

		mapHeight := float64(g.worldHeight)

		if g.heli.X < 1.0 {
			g.heli.X = 1.0
			g.heli.VX = -g.heli.VX * 0.4
		}
		if g.heli.X > float64(g.worldWidth-2) {
			g.heli.X = float64(g.worldWidth - 2)
			g.heli.VX = -g.heli.VX * 0.4
		}
		if g.heli.Y < 1.0 {
			g.heli.Y = 1.0
			g.heli.VY = -g.heli.VY * 0.4
		}
		if g.heli.Y > mapHeight-2.0 {
			g.heli.Y = mapHeight - 2.0
			g.heli.VY = -g.heli.VY * 0.4
		}

		// Automatic landing when hovering slowly over the landing pad
		padX := g.carrier.X + g.carrier.Width/3
		padY := g.carrier.Y + g.carrier.Height/2
		aligned := int(math.Round(g.heli.X)) >= padX-1 && int(math.Round(g.heli.X)) <= padX+1 &&
			int(math.Round(g.heli.Y)) >= padY-1 && int(math.Round(g.heli.Y)) <= padY+1

		speed = math.Sqrt(g.heli.VX*g.heli.VX + g.heli.VY*g.heli.VY)
		if aligned && speed < 0.12 && g.heli.TakeoffCooldown == 0 {
			g.heli.Landed = true
			g.heli.ReturningToCarrier = false
			g.heli.X = float64(padX)
			g.heli.Y = float64(padY)
			g.heli.VX = 0
			g.heli.VY = 0
			g.heli.TakeoffCooldown = 25
			slog.Info("Auto-landed on carrier pad", "x", g.heli.X, "y", g.heli.Y)
		} else {
			g.heli.RotorState = (g.heli.RotorState + 1) % len(rotorFrames)
		}
	}
}

func (g *Game) updateCamera() {
	hx := int(math.Round(g.heli.X))
	hy := int(math.Round(g.heli.Y))

	// Margin values (20% of screen size)
	marginW := int(float64(g.width) * 0.30)
	marginH := int(float64(g.height-6) * 0.30)

	// Keep margins reasonable
	if marginW < 5 {
		marginW = 5
	}
	if marginH < 3 {
		marginH = 3
	}

	// 1. Check horizontal boundaries (camera dead zone)
	if hx-g.camX < marginW {
		g.camX = hx - marginW
	} else if hx-g.camX > g.width-marginW {
		g.camX = hx - (g.width - marginW)
	}

	// 2. Check vertical boundaries (camera dead zone)
	if hy-g.camY < marginH {
		g.camY = hy - marginH
	} else if hy-g.camY > (g.height-6)-marginH {
		g.camY = hy - ((g.height - 6) - marginH)
	}

	// 3. Clamp camera to world boundaries
	g.clampCamera()
}

// clampCamera constrains the camera offset to the world bounds, accounting for
// the 4-row HUD reserved at the bottom of the screen.
func (g *Game) clampCamera() {
	if g.camX < 0 {
		g.camX = 0
	}
	if g.worldWidth > g.width {
		if g.camX > g.worldWidth-g.width {
			g.camX = g.worldWidth - g.width
		}
	} else {
		g.camX = 0
	}

	if g.camY < 0 {
		g.camY = 0
	}
	if g.worldHeight > g.height-6 {
		if g.camY > g.worldHeight-(g.height-6) {
			g.camY = g.worldHeight - (g.height - 6)
		}
	} else {
		g.camY = 0
	}
}

// centerCameraOnPad centers the camera over the carrier landing pad and clamps
// it to the world bounds.
func (g *Game) centerCameraOnPad(padX, padY int) {
	g.camX = padX - g.width/2
	g.camY = padY - (g.height-6)/2
	g.clampCamera()
}

func (g *Game) updateWeaponCooldowns() {
	if g.heli.FireCooldown > 0 {
		g.heli.FireCooldown--
	}
	if g.heli.MissileCooldown > 0 {
		g.heli.MissileCooldown--
	}
	if g.heli.CannonHeat > 0 {
		g.heli.CannonHeat--
	}
	if g.heli.CannonJammed > 0 {
		g.heli.CannonJammed--
	}
}

// replenishCarrierDrones slowly rebuilds the carrier's defensive drone screen
// (one drone every 100 ticks) while the helicopter is parked on the pad.
func (g *Game) replenishCarrierDrones() {
	if !g.heli.Landed || g.Ticks%100 != 0 {
		return
	}

	carrierDronesCount := 0
	for d := 0; d < len(g.drones); d++ {
		if g.drones[d].Active && g.drones[d].FactoryIdx == -1 {
			carrierDronesCount++
		}
	}
	if carrierDronesCount >= 3 {
		return
	}

	angle := 0.0
	if carrierDronesCount == 1 {
		for d := 0; d < len(g.drones); d++ {
			if g.drones[d].Active && g.drones[d].FactoryIdx == -1 {
				angle = g.drones[d].Angle + 2.0*math.Pi/3.0
				break
			}
		}
	} else if carrierDronesCount == 2 {
		var angles []float64
		for d := 0; d < len(g.drones); d++ {
			if g.drones[d].Active && g.drones[d].FactoryIdx == -1 {
				angles = append(angles, g.drones[d].Angle)
			}
		}
		if len(angles) == 2 {
			mid := (angles[0] + angles[1]) / 2.0
			if math.Abs(angles[0]-angles[1]) > math.Pi {
				angle = mid
			} else {
				angle = mid + math.Pi
			}
		}
	}
	cx := float64(g.carrier.X + g.carrier.Width/2)
	cy := float64(g.carrier.Y + g.carrier.Height/2)

	g.appendDrone(Drone{X: cx, Y: cy, Active: true, Angle: angle, FactoryIdx: -1})
	slog.Info("Carrier repaired/spawned defensive carrier drone!")
}

func (g *Game) updateCarrierDefense() {
	g.replenishCarrierDrones()

	if g.carrier.Health <= 0 {
		return
	}
	if g.carrier.MissileCooldown > 0 {
		g.carrier.MissileCooldown--
		return
	}

	cx := float64(g.carrier.X + g.carrier.Width/2)
	cy := float64(g.carrier.Y + g.carrier.Height/2)
	var targetBoat *Boat
	minDist := 45.0

	for i := 0; i < len(g.boats); i++ {
		boat := &g.boats[i]
		if !boat.Active || boat.SinkingTimer > 0 {
			continue
		}
		dxVec := boat.X - cx
		dyVec := boat.Y - cy
		dist := math.Sqrt(dxVec*dxVec + dyVec*dyVec)
		if dist < minDist {
			minDist = dist
			targetBoat = boat
		}
	}

	if targetBoat == nil {
		return
	}

	dxVec := targetBoat.X - cx
	dyVec := targetBoat.Y - cy
	dist := math.Sqrt(dxVec*dxVec + dyVec*dyVec)
	initialSpeed := 0.5
	mvx := (dxVec / dist) * initialSpeed
	mvy := (dyVec / dist) * initialSpeed

	g.spawnCarrierMissile(cx, cy, mvx, mvy)
	slog.Info("Carrier launched defensive SSM at enemy boat!", "boat_x", targetBoat.X, "dist", minDist)
	g.carrier.MissileCooldown = 300 + rand.Intn(150)
}

func (g *Game) updateExplosions() {
	active := make([]Explosion, 0, len(g.explosions))
	for i := range g.explosions {
		g.explosions[i].Age++
		if g.explosions[i].Age < 15 {
			active = append(active, g.explosions[i])
		}
	}
	g.explosions = active
}

func (g *Game) checkWaveCompletion() {
	allSunk := true
	for i := range g.boats {
		if g.boats[i].Active {
			allSunk = false
			break
		}
	}
	if allSunk {
		for fIdx := range g.factories {
			if g.factories[fIdx].Active {
				allSunk = false
				break
			}
		}
	}
	if allSunk {
		for tIdx := range g.tanks {
			if g.tanks[tIdx].Active {
				allSunk = false
				break
			}
		}
	}
	if allSunk {
		for saIdx := range g.staticAAs {
			if g.staticAAs[saIdx].Active {
				allSunk = false
				break
			}
		}
	}

	if !allSunk {
		return
	}

	g.Wave++
	g.Lives = 5
	PlaySound("explosion")
	slog.Info("All enemy assets destroyed! Advancing to next wave", "wave", g.Wave, "speed_multiplier", 1.25)

	if !g.heli.Landed && g.heli.Armor > 0 && g.heli.RespawnTimer == 0 {
		g.heli.ReturningToCarrier = true
		slog.Info("Wave cleared - Osprey returning to carrier")
	}

	for i := range g.boats {
		g.boats[i].Active = true
		g.boats[i].Health = g.boats[i].MaxHealth
		g.boats[i].SinkingTimer = 0
		g.boats[i].X = g.initialBoats[i].X
		g.boats[i].Y = g.initialBoats[i].Y
		by := int(math.Round(g.boats[i].Y))
		thresh := g.getCoastlineThreshold(by)
		g.boats[i].PatrolMinX = thresh - 18.0
		newSpeed := g.boats[i].VX * 1.25
		if math.Abs(newSpeed) > 2.0 {
			if newSpeed < 0 {
				g.boats[i].VX = -2.0
			} else {
				g.boats[i].VX = 2.0
			}
		} else {
			g.boats[i].VX = newSpeed
		}
		g.boats[i].MissileCooldown = 600 + rand.Intn(400)
	}

	g.resetFactories()
	g.resetDrones()

	for tIdx := range g.tanks {
		tank := &g.tanks[tIdx]
		tank.Active = g.Wave >= 2
		tank.Health = tank.MaxHealth
		tank.SinkingTimer = 0
		tank.FireCooldown = 60 + rand.Intn(100)
		if tank.PatrolDir == 0 {
			newSpeed := tank.VY * 1.25
			if math.Abs(newSpeed) > 2.0 {
				if newSpeed < 0 {
					tank.VY = -2.0
				} else {
					tank.VY = 2.0
				}
			} else {
				tank.VY = newSpeed
			}
		} else {
			newSpeed := tank.VX * 1.25
			if math.Abs(newSpeed) > 2.0 {
				if newSpeed < 0 {
					tank.VX = -2.0
				} else {
					tank.VX = 2.0
				}
			} else {
				tank.VX = newSpeed
			}
		}
	}

	g.resetStaticAAs(g.Wave >= 3)
}

// resetFactories restores every factory to full health with a fresh fire
// cooldown and replenished drone reserves. Shared by wave and round resets.
func (g *Game) resetFactories() {
	for fIdx := range g.factories {
		fact := &g.factories[fIdx]
		fact.Active = true
		fact.Health = fact.MaxHealth
		fact.SinkingTimer = 0
		fact.FireCooldown = 100 + rand.Intn(100)
		fact.DronesRemaining = 8
	}
}

// resetDrones reactivates all drones and snaps them back to their orbit anchor
// (their owning factory, or the carrier for FactoryIdx == -1). Shared by wave
// and round resets; assumes factories have already been reset.
func (g *Game) resetDrones() {
	for d := range g.drones {
		g.drones[d].Active = true
		if d%2 == 0 {
			g.drones[d].Angle = 0.0
		} else {
			g.drones[d].Angle = 3.14159
		}
		if g.drones[d].FactoryIdx >= 0 {
			fact := &g.factories[g.drones[d].FactoryIdx]
			g.drones[d].X = fact.X
			g.drones[d].Y = fact.Y
		} else if g.drones[d].FactoryIdx == -1 {
			g.drones[d].X = float64(g.carrier.X + g.carrier.Width/2)
			g.drones[d].Y = float64(g.carrier.Y + g.carrier.Height/2)
		}
	}
}

// resetStaticAAs restores every static AA gun to full health with a fresh fire
// cooldown. The active flag is caller-controlled: wave resets gate it behind
// Wave >= 3, while round resets reactivate unconditionally.
func (g *Game) resetStaticAAs(active bool) {
	for saIdx := range g.staticAAs {
		sa := &g.staticAAs[saIdx]
		sa.Active = active
		sa.Health = sa.MaxHealth
		sa.SinkingTimer = 0
		sa.FireCooldown = 45 + rand.Intn(100)
	}
}

func (g *Game) resetRound() {
	slog.Info("Resetting round due to carrier destruction")

	g.carrier.Health = 100.0
	g.Lives = 5

	padX := g.carrier.X + g.carrier.Width/3
	padY := g.carrier.Y + g.carrier.Height/2
	g.heli.X = float64(padX)
	g.heli.Y = float64(padY)
	g.heli.VX = 0
	g.heli.VY = 0
	g.heli.Dir = 0
	g.heli.Landed = true
	g.heli.Fuel = 100.0
	g.heli.Armor = 100.0
	g.heli.MissileAmmo = 4
	g.heli.TakeoffCooldown = 25

	// Reset camera to center around the carrier pad
	g.centerCameraOnPad(padX, padY)

	g.bullets = g.bullets[:0]
	g.missiles = g.missiles[:0]


	for i := range g.boats {
		boat := &g.boats[i]
		initial := g.initialBoats[i]
		boat.Active = true
		boat.Health = initial.Health
		boat.MaxHealth = initial.MaxHealth
		boat.SinkingTimer = 0
		boat.FireCooldown = 60 + rand.Intn(80)
		boat.MissileCooldown = 200 + i*200
		boat.X = initial.X
		boat.Y = initial.Y
		boat.VX = initial.VX
	}

	g.resetFactories()
	g.resetDrones()

	for tIdx := range g.tanks {
		tank := &g.tanks[tIdx]
		tank.Active = true
		tank.Health = tank.MaxHealth
		tank.SinkingTimer = 0
		tank.FireCooldown = 60 + rand.Intn(100)
		if tIdx == 0 {
			tank.X = float64(g.worldWidth - 15)
			tank.Y = float64(g.worldHeight * 5 / 16)
			tank.VY = 0.04
			tank.VX = 0
			tank.PatrolDir = 0
			tank.MinCoord = float64(g.worldHeight / 8)
			tank.MaxCoord = float64(g.worldHeight / 2)
		} else if tIdx == 1 {
			tank.X = float64(g.worldWidth - 15)
			tank.Y = float64(g.worldHeight * 11 / 16)
			tank.VY = -0.04
			tank.VX = 0
			tank.PatrolDir = 0
			tank.MinCoord = float64(g.worldHeight / 2)
			tank.MaxCoord = float64(g.worldHeight * 7 / 8)
		} else if tIdx == 2 {
			tank.X = float64(g.worldWidth - 11)
			tank.Y = float64(g.worldHeight / 2)
			tank.VX = 0.06
			tank.VY = 0
			tank.PatrolDir = 1
			tank.MinCoord = float64(g.worldWidth - 15)
			tank.MaxCoord = float64(g.worldWidth - 7)
		}
	}

	g.resetStaticAAs(true)

	g.stealthSpawnAt = 0
	for i := range g.stealthBoats {
		g.stealthBoats[i].Active = false
	}
}

func (g *Game) initStaticAAs() {
	h := g.worldHeight
	if h <= 0 {
		h = 1
	}

	g.staticAAs = make([]StaticAA, 6)
	for i := 0; i < 6; i++ {
		y := int(float64(h) * (float64(i) + 0.5) / 6.0)
		if y < 2 {
			y = 2
		}
		if y > h-3 {
			y = h - 3
		}

		threshold := g.getCoastlineThreshold(y)
		x := threshold + 6.0

		for g.isRoad(int(math.Round(x)), y) {
			x += 3.0
		}

		g.staticAAs[i] = StaticAA{
			X:            x,
			Y:            float64(y),
			Health:       4,
			MaxHealth:    4,
			Active:       g.Wave >= 3,
			FireCooldown: 30 + rand.Intn(100),
		}
		slog.Info("Initialized static AA gun", "idx", i, "x", x, "y", y)
	}
}

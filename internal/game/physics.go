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
	g.Ticks++
	g.updateHelicopter()
	g.updateWeaponCooldowns()
	g.updateCarrierDefense()
	g.updateProjectiles()
	g.updateBoats()
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

		// Slowly replenish carrier defense drones on landing pad (rebuild 1 drone every 100 ticks)
		if g.Ticks%100 == 0 {
			carrierDronesCount := 0
			for d := 0; d < len(g.drones); d++ {
				if g.drones[d].Active && g.drones[d].FactoryIdx == -1 {
					carrierDronesCount++
				}
			}
			if carrierDronesCount < 2 {
				angle := 0.0
				if carrierDronesCount > 0 {
					for d := 0; d < len(g.drones); d++ {
						if g.drones[d].Active && g.drones[d].FactoryIdx == -1 {
							angle = g.drones[d].Angle + math.Pi
							break
						}
					}
				}
				cx := float64(g.carrier.X + g.carrier.Width/2)
				cy := float64(g.carrier.Y + g.carrier.Height/2)

				spawned := false
				for d := 0; d < len(g.drones); d++ {
					if !g.drones[d].Active {
						g.drones[d] = Drone{X: cx, Y: cy, Active: true, Angle: angle, FactoryIdx: -1}
						spawned = true
						break
					}
				}
				if !spawned {
					g.drones = append(g.drones, Drone{X: cx, Y: cy, Active: true, Angle: angle, FactoryIdx: -1})
				}
				slog.Info("Carrier repaired/spawned defensive carrier drone!")
			}
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
				g.heli.TakeoffCooldown = 25
			}
		}

		mapHeight := float64(g.height - 4)

		if g.heli.X < 1.0 {
			g.heli.X = 1.0
			g.heli.VX = -g.heli.VX * 0.4
		}
		if g.heli.X > float64(g.width-2) {
			g.heli.X = float64(g.width - 2)
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

func (g *Game) updateWeaponCooldowns() {
	if g.heli.FireCooldown > 0 {
		g.heli.FireCooldown--
	}
	if g.heli.MissileCooldown > 0 {
		g.heli.MissileCooldown--
	}
}

func (g *Game) updateCarrierDefense() {
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

	spawned := false
	for i := 0; i < len(g.missiles); i++ {
		if !g.missiles[i].Active {
			g.missiles[i] = Missile{X: cx, Y: cy, StartX: cx, StartY: cy, VX: mvx, VY: mvy, Active: true, IsEnemy: false, IsCarrier: true}
			spawned = true
			break
		}
	}
	if !spawned && len(g.missiles) < 16 {
		g.missiles = append(g.missiles, Missile{X: cx, Y: cy, StartX: cx, StartY: cy, VX: mvx, VY: mvy, Active: true, IsEnemy: false, IsCarrier: true})
	}

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

	slog.Info("All enemy assets destroyed! Resetting with progressive difficulty increase", "speed_multiplier", 1.25)

	for i := range g.boats {
		g.boats[i].Active = true
		g.boats[i].Health = g.boats[i].MaxHealth
		g.boats[i].SinkingTimer = 0
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
		g.boats[i].MissileCooldown = 300 + rand.Intn(300)
	}

	for fIdx := range g.factories {
		fact := &g.factories[fIdx]
		fact.Active = true
		fact.Health = fact.MaxHealth
		fact.SinkingTimer = 0
		fact.FireCooldown = 100 + rand.Intn(100)
		fact.DronesRemaining = 8
	}

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

	for tIdx := range g.tanks {
		tank := &g.tanks[tIdx]
		tank.Active = true
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

	for saIdx := range g.staticAAs {
		sa := &g.staticAAs[saIdx]
		sa.Active = true
		sa.Health = sa.MaxHealth
		sa.SinkingTimer = 0
		sa.FireCooldown = 45 + rand.Intn(100)
	}
}

func (g *Game) resetRound() {
	slog.Info("Resetting round due to carrier destruction")

	g.carrier.Health = 100.0

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

	g.bullets = g.bullets[:0]
	g.missiles = g.missiles[:0]

	g.boatsSunk = 0
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

	for fIdx := range g.factories {
		fact := &g.factories[fIdx]
		fact.Active = true
		fact.Health = fact.MaxHealth
		fact.SinkingTimer = 0
		fact.FireCooldown = 100 + rand.Intn(100)
		fact.DronesRemaining = 8
	}

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

	for tIdx := range g.tanks {
		tank := &g.tanks[tIdx]
		tank.Active = true
		tank.Health = tank.MaxHealth
		tank.SinkingTimer = 0
		tank.FireCooldown = 60 + rand.Intn(100)
		if tIdx == 0 {
			tank.X = float64(g.width - 15)
			tank.Y = float64(g.height * 5 / 16)
			tank.VY = 0.04
			tank.VX = 0
			tank.PatrolDir = 0
			tank.MinCoord = float64(g.height / 8)
			tank.MaxCoord = float64(g.height / 2)
		} else if tIdx == 1 {
			tank.X = float64(g.width - 15)
			tank.Y = float64(g.height * 11 / 16)
			tank.VY = -0.04
			tank.VX = 0
			tank.PatrolDir = 0
			tank.MinCoord = float64(g.height / 2)
			tank.MaxCoord = float64(g.height * 7 / 8)
		} else if tIdx == 2 {
			tank.X = float64(g.width - 11)
			tank.Y = float64(g.height / 2)
			tank.VX = 0.06
			tank.VY = 0
			tank.PatrolDir = 1
			tank.MinCoord = float64(g.width - 15)
			tank.MaxCoord = float64(g.width - 7)
		}
	}

	for saIdx := range g.staticAAs {
		sa := &g.staticAAs[saIdx]
		sa.Active = true
		sa.Health = sa.MaxHealth
		sa.SinkingTimer = 0
		sa.FireCooldown = 45 + rand.Intn(100)
	}
}

func (g *Game) initStaticAAs() {
	h := g.height - 4
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
			Health:       5,
			MaxHealth:    5,
			Active:       true,
			FireCooldown: 30 + rand.Intn(100),
		}
		slog.Info("Initialized static AA gun", "idx", i, "x", x, "y", y)
	}
}

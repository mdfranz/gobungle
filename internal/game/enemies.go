package game

import (
	"log/slog"
	"math"
	"math/rand"
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
				g.boatsSunk++
				slog.Info("Doomed boat has fully sunk", "boat_idx", i, "total_sunk", g.boatsSunk)
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

		// Gradually expand patrol range from coast toward carrier
		if boat.PatrolMinX > 6 {
			boat.PatrolMinX -= 0.02
			if boat.PatrolMinX < 6 {
				boat.PatrolMinX = 6
			}
		}

		// AA fire against the helicopter
		if boat.FireCooldown > 0 {
			boat.FireCooldown--
		} else if !g.heli.Landed && g.heli.Fuel > 0 && g.heli.Armor > 0 {
			dxVec := g.heli.X - boat.X
			dyVec := g.heli.Y - boat.Y
			dist := math.Sqrt(dxVec*dxVec + dyVec*dyVec)
			if dist > 0 && dist < BoatAARange {
				speed := 2.0
				g.spawnEnemyBullet(boat.X, boat.Y, (dxVec/dist)*speed, (dyVec/dist)*speed)
				slog.Info("Boat fired anti-aircraft projectile", "boat_idx", i, "x", boat.X, "y", boat.Y)
				boat.FireCooldown = 60 + rand.Intn(80)
			}
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
			if fact.FireCooldown > 0 {
				fact.FireCooldown--
			} else if !g.heli.Landed && g.heli.Fuel > 0 && g.heli.Armor > 0 {
				dxVec := g.heli.X - fact.X
				dyVec := g.heli.Y - fact.Y
				dist := math.Sqrt(dxVec*dxVec + dyVec*dyVec)
				if dist > 0 && dist < BoatAARange {
					speed := 2.0
					g.spawnEnemyBullet(fact.X, fact.Y, (dxVec/dist)*speed, (dyVec/dist)*speed)
					slog.Info("Factory fired fortress anti-aircraft projectile!", "x", fact.X, "y", fact.Y, "idx", fIdx)
					fact.FireCooldown = 40 + rand.Intn(40)
				}
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

				spawned := false
				for d := 0; d < len(g.drones); d++ {
					if !g.drones[d].Active {
						g.drones[d] = Drone{X: fact.X, Y: fact.Y, Active: true, Angle: angle, FactoryIdx: fIdx}
						spawned = true
						break
					}
				}
				if !spawned {
					g.drones = append(g.drones, Drone{X: fact.X, Y: fact.Y, Active: true, Angle: angle, FactoryIdx: fIdx})
				}
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
			if tank.FireCooldown > 0 {
				tank.FireCooldown--
			} else if !g.heli.Landed && g.heli.Fuel > 0 && g.heli.Armor > 0 {
				dxVec := g.heli.X - tank.X
				dyVec := g.heli.Y - tank.Y
				dist := math.Sqrt(dxVec*dxVec + dyVec*dyVec)
				if dist > 0 && dist < 40.0 {
					speed := 2.2
					g.spawnEnemyBullet(tank.X, tank.Y, (dxVec/dist)*speed, (dyVec/dist)*speed)
					slog.Info("Tank fired flak projectile!", "tank_idx", tIdx, "x", tank.X, "y", tank.Y)
					tank.FireCooldown = 50 + rand.Intn(40)
				}
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
			if sa.FireCooldown > 0 {
				sa.FireCooldown--
			} else if !g.heli.Landed && g.heli.Fuel > 0 && g.heli.Armor > 0 {
				dxVec := g.heli.X - sa.X
				dyVec := g.heli.Y - sa.Y
				dist := math.Sqrt(dxVec*dxVec + dyVec*dyVec)
				if dist > 0 && dist < 45.0 {
					speed := 2.2
					g.spawnEnemyBullet(sa.X, sa.Y, (dxVec/dist)*speed, (dyVec/dist)*speed)
					slog.Info("Static AA fired flak projectile!", "idx", saIdx, "x", sa.X, "y", sa.Y)
					sa.FireCooldown = 45 + rand.Intn(35)
				}
			}
		}
	}
}

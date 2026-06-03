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
			boat.SinkingTimer--

			if boat.SinkingTimer%3 == 0 {
				offsetX := float64(rand.Intn(11) - 5)
				offsetY := float64(rand.Intn(3) - 1)
				g.explosions = append(g.explosions, Explosion{
					X:   int(math.Round(boat.X + offsetX)),
					Y:   int(math.Round(boat.Y + offsetY)),
					Age: 0,
				})
			}

			if boat.SinkingTimer == 0 {
				boat.Active = false
				g.boatsSunk++
				slog.Info("Doomed boat has fully sunk", "boat_idx", i, "total_sunk", g.boatsSunk)

				for ddx := -5; ddx <= 5; ddx++ {
					for ddy := -1; ddy <= 1; ddy++ {
						g.explosions = append(g.explosions, Explosion{
							X:   int(math.Round(boat.X)) + ddx,
							Y:   int(math.Round(boat.Y)) + ddy,
							Age: 0,
						})
					}
				}
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
				bulletSpeed := 2.0
				bvx := (dxVec / dist) * bulletSpeed
				bvy := (dyVec / dist) * bulletSpeed

				spawned := false
				for k := 0; k < len(g.bullets); k++ {
					if !g.bullets[k].Active {
						g.bullets[k] = Bullet{X: boat.X, Y: boat.Y, StartX: boat.X, StartY: boat.Y, VX: bvx, VY: bvy, Active: true, IsEnemy: true}
						spawned = true
						break
					}
				}
				if !spawned && len(g.bullets) < 24 {
					g.bullets = append(g.bullets, Bullet{X: boat.X, Y: boat.Y, StartX: boat.X, StartY: boat.Y, VX: bvx, VY: bvy, Active: true, IsEnemy: true})
				}

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
				initialSpeed := 0.3
				mvx := (dxVec / dist) * initialSpeed
				mvy := (dyVec / dist) * initialSpeed

				spawned := false
				for k := 0; k < len(g.missiles); k++ {
					if !g.missiles[k].Active {
						g.missiles[k] = Missile{X: boat.X, Y: boat.Y, StartX: boat.X, StartY: boat.Y, VX: mvx, VY: mvy, Active: true, InterceptionRolled: false, IsEnemy: true}
						spawned = true
						break
					}
				}
				if !spawned && len(g.missiles) < 16 {
					g.missiles = append(g.missiles, Missile{X: boat.X, Y: boat.Y, StartX: boat.X, StartY: boat.Y, VX: mvx, VY: mvy, Active: true, InterceptionRolled: false, IsEnemy: true})
				}
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
			fact.SinkingTimer--

			if fact.SinkingTimer%3 == 0 {
				offsetX := float64(rand.Intn(7) - 3)
				offsetY := float64(rand.Intn(3) - 1)
				g.explosions = append(g.explosions, Explosion{
					X:   int(math.Round(fact.X + offsetX)),
					Y:   int(math.Round(fact.Y + offsetY)),
					Age: 0,
				})
			}

			if fact.SinkingTimer == 0 {
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

				for ddx := -6; ddx <= 6; ddx++ {
					for ddy := -2; ddy <= 2; ddy++ {
						g.explosions = append(g.explosions, Explosion{
							X:   int(math.Round(fact.X)) + ddx,
							Y:   int(math.Round(fact.Y)) + ddy,
							Age: rand.Intn(4),
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
					bulletSpeed := 2.0
					bvx := (dxVec / dist) * bulletSpeed
					bvy := (dyVec / dist) * bulletSpeed

					spawned := false
					for k := 0; k < len(g.bullets); k++ {
						if !g.bullets[k].Active {
							g.bullets[k] = Bullet{X: fact.X, Y: fact.Y, StartX: fact.X, StartY: fact.Y, VX: bvx, VY: bvy, Active: true, IsEnemy: true}
							spawned = true
							break
						}
					}
					if !spawned && len(g.bullets) < 24 {
						g.bullets = append(g.bullets, Bullet{X: fact.X, Y: fact.Y, StartX: fact.X, StartY: fact.Y, VX: bvx, VY: bvy, Active: true, IsEnemy: true})
					}

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
					initialSpeed := 0.25
					mvx := (dxVec / dist) * initialSpeed
					mvy := (dyVec / dist) * initialSpeed

					spawned := false
					for k := 0; k < len(g.missiles); k++ {
						if !g.missiles[k].Active {
							g.missiles[k] = Missile{X: fact.X, Y: fact.Y, StartX: fact.X, StartY: fact.Y, VX: mvx, VY: mvy, Active: true, InterceptionRolled: false, IsEnemy: true}
							spawned = true
							break
						}
					}
					if !spawned && len(g.missiles) < 16 {
						g.missiles = append(g.missiles, Missile{X: fact.X, Y: fact.Y, StartX: fact.X, StartY: fact.Y, VX: mvx, VY: mvy, Active: true, InterceptionRolled: false, IsEnemy: true})
					}
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
			tank.SinkingTimer--

			if tank.SinkingTimer%3 == 0 {
				offsetX := float64(rand.Intn(3) - 1)
				offsetY := float64(rand.Intn(3) - 1)
				g.explosions = append(g.explosions, Explosion{
					X:   int(math.Round(tank.X + offsetX)),
					Y:   int(math.Round(tank.Y + offsetY)),
					Age: 0,
				})
			}

			if tank.SinkingTimer == 0 {
				tank.Active = false
				slog.Info("Patrolling Tank has fully blown up!", "tank_idx", tIdx)

				for ddx := -2; ddx <= 2; ddx++ {
					for ddy := -1; ddy <= 1; ddy++ {
						g.explosions = append(g.explosions, Explosion{
							X:   int(math.Round(tank.X)) + ddx,
							Y:   int(math.Round(tank.Y)) + ddy,
							Age: rand.Intn(3),
						})
					}
				}
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
					bulletSpeed := 2.2
					bvx := (dxVec / dist) * bulletSpeed
					bvy := (dyVec / dist) * bulletSpeed

					spawned := false
					for k := 0; k < len(g.bullets); k++ {
						if !g.bullets[k].Active {
							g.bullets[k] = Bullet{X: tank.X, Y: tank.Y, StartX: tank.X, StartY: tank.Y, VX: bvx, VY: bvy, Active: true, IsEnemy: true}
							spawned = true
							break
						}
					}
					if !spawned && len(g.bullets) < 24 {
						g.bullets = append(g.bullets, Bullet{X: tank.X, Y: tank.Y, StartX: tank.X, StartY: tank.Y, VX: bvx, VY: bvy, Active: true, IsEnemy: true})
					}

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
			sa.SinkingTimer--

			if sa.SinkingTimer%3 == 0 {
				offsetX := float64(rand.Intn(3) - 1)
				offsetY := float64(rand.Intn(3) - 1)
				g.explosions = append(g.explosions, Explosion{
					X:   int(math.Round(sa.X + offsetX)),
					Y:   int(math.Round(sa.Y + offsetY)),
					Age: 0,
				})
			}

			if sa.SinkingTimer == 0 {
				sa.Active = false
				slog.Info("Static AA has fully blown up!", "idx", saIdx)

				for ddx := -2; ddx <= 2; ddx++ {
					for ddy := -1; ddy <= 1; ddy++ {
						g.explosions = append(g.explosions, Explosion{
							X:   int(math.Round(sa.X)) + ddx,
							Y:   int(math.Round(sa.Y)) + ddy,
							Age: rand.Intn(3),
						})
					}
				}
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
					bulletSpeed := 2.2
					bvx := (dxVec / dist) * bulletSpeed
					bvy := (dyVec / dist) * bulletSpeed

					spawned := false
					for k := 0; k < len(g.bullets); k++ {
						if !g.bullets[k].Active {
							g.bullets[k] = Bullet{X: sa.X, Y: sa.Y, StartX: sa.X, StartY: sa.Y, VX: bvx, VY: bvy, Active: true, IsEnemy: true}
							spawned = true
							break
						}
					}
					if !spawned && len(g.bullets) < 24 {
						g.bullets = append(g.bullets, Bullet{X: sa.X, Y: sa.Y, StartX: sa.X, StartY: sa.Y, VX: bvx, VY: bvy, Active: true, IsEnemy: true})
					}

					slog.Info("Static AA fired flak projectile!", "idx", saIdx, "x", sa.X, "y", sa.Y)
					sa.FireCooldown = 45 + rand.Intn(35)
				}
			}
		}
	}
}

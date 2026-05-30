package game

import (
	"log/slog"
	"math"
	"math/rand"
)

// checkCollisions handles all projectile-vs-entity and drone-vs-missile collision detection.
func (g *Game) checkCollisions() {
	g.checkDroneMissileInterceptions()
	g.checkBulletCollisions()
	g.checkMissileCollisions()
	g.checkPlayerBulletsVsEnemyMissiles()
	g.checkEnemyBulletsVsPlayerMissiles()
}

// checkDroneMissileInterceptions checks whether orbiting drones intercept guided missiles.
func (g *Game) checkDroneMissileInterceptions() {
	for i := 0; i < len(g.missiles); i++ {
		m := &g.missiles[i]
		if !m.Active {
			continue
		}
		for d := 0; d < len(g.drones); d++ {
			drone := &g.drones[d]
			if !drone.Active {
				continue
			}

			isFactoryInterception := !m.IsEnemy && drone.FactoryIdx >= 0
			isCarrierInterception := m.IsEnemy && drone.FactoryIdx == -1

			if !isFactoryInterception && !isCarrierInterception {
				continue
			}

			ddx := drone.X - m.X
			ddy := drone.Y - m.Y
			if math.Sqrt(ddx*ddx+ddy*ddy) < 4.0 {
				m.Active = false
				drone.Active = false

				if isFactoryInterception {
					slog.Info("Drone shield interception: Air defense drone neutralized player guided missile!", "missile_idx", i, "drone_idx", d)
				} else {
					slog.Info("Carrier drone shield interception: Carrier defense drone neutralized enemy guided missile!", "missile_idx", i, "drone_idx", d)
				}

				midX := int(math.Round((m.X + drone.X) / 2))
				midY := int(math.Round((m.Y + drone.Y) / 2))
				for ox := -1; ox <= 1; ox++ {
					for oy := -1; oy <= 1; oy++ {
						g.explosions = append(g.explosions, Explosion{X: midX + ox, Y: midY + oy, Age: rand.Intn(4)})
					}
				}
				break
			}
		}
	}
}

func (g *Game) checkBulletCollisions() {
	for i := 0; i < len(g.bullets); i++ {
		bullet := &g.bullets[i]
		if !bullet.Active {
			continue
		}

		if bullet.IsEnemy {
			g.checkEnemyBulletVsPlayer(bullet)
		} else {
			g.checkPlayerBulletVsTargets(bullet)
		}
	}
}

func (g *Game) checkEnemyBulletVsPlayer(bullet *Bullet) {
	if g.heli.Landed || g.heli.Armor <= 0 {
		return
	}
	if math.Abs(bullet.X-g.heli.X) < 2.5 && math.Abs(bullet.Y-g.heli.Y) < 1.5 {
		bullet.Active = false
		g.heli.Armor -= 15.0
		slog.Info("Enemy projectile hit Player", "damage", 15.0, "remaining_armor", g.heli.Armor)
		g.explosions = append(g.explosions, Explosion{X: int(math.Round(bullet.X)), Y: int(math.Round(bullet.Y)), Age: 0})

		if g.heli.Armor <= 0 {
			g.heli.Armor = 0
			slog.Warn("Helicopter destroyed", "x", g.heli.X, "y", g.heli.Y)

			hx := int(math.Round(g.heli.X))
			hy := int(math.Round(g.heli.Y))
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
		}
	}
}

func (g *Game) checkPlayerBulletVsTargets(bullet *Bullet) {
	// vs Boats
	for j := 0; j < len(g.boats); j++ {
		boat := &g.boats[j]
		if !boat.Active {
			continue
		}
		if math.Abs(bullet.X-boat.X) < 5.5 && math.Abs(bullet.Y-boat.Y) < 1.5 {
			bullet.Active = false
			if boat.SinkingTimer == 0 {
				boat.Health--
				slog.Info("Player bullet hit Boat", "boat_idx", j, "health", boat.Health, "max_health", boat.MaxHealth)
				g.explosions = append(g.explosions, Explosion{X: int(math.Round(bullet.X)), Y: int(math.Round(bullet.Y)), Age: 0})

				if boat.Health <= 0 {
					boat.Active = false
					g.boatsSunk++
					slog.Info("Boat sunk by cannon round", "boat_idx", j, "total_sunk", g.boatsSunk)
					for ddx := -5; ddx <= 5; ddx++ {
						for ddy := -1; ddy <= 1; ddy++ {
							g.explosions = append(g.explosions, Explosion{X: int(math.Round(boat.X)) + ddx, Y: int(math.Round(boat.Y)) + ddy, Age: 0})
						}
					}
				}
			} else {
				slog.Info("Player bullet hit already-sinking Boat", "boat_idx", j)
				g.explosions = append(g.explosions, Explosion{X: int(math.Round(bullet.X)), Y: int(math.Round(bullet.Y)), Age: 0})
			}
			return
		}
	}

	// vs Drones
	for d := 0; d < len(g.drones); d++ {
		drone := &g.drones[d]
		if !drone.Active {
			continue
		}
		if math.Abs(bullet.X-drone.X) < 1.5 && math.Abs(bullet.Y-drone.Y) < 1.2 {
			bullet.Active = false
			drone.Active = false
			slog.Info("Player shot down an Air Defense Drone!", "drone_idx", d)
			g.explosions = append(g.explosions, Explosion{X: int(math.Round(drone.X)), Y: int(math.Round(drone.Y)), Age: 0})
			return
		}
	}

	// vs Factories
	for fIdx := range g.factories {
		fact := &g.factories[fIdx]
		if fact.Active && fact.SinkingTimer == 0 {
			if math.Abs(bullet.X-fact.X) < 8.5 && math.Abs(bullet.Y-fact.Y) < 2.5 {
				bullet.Active = false
				fact.Health--
				slog.Info("Player bullet hit Factory", "idx", fIdx, "health", fact.Health, "max_health", fact.MaxHealth)
				g.explosions = append(g.explosions, Explosion{X: int(math.Round(bullet.X)), Y: int(math.Round(bullet.Y)), Age: 0})

				if fact.Health <= 0 {
					fact.SinkingTimer = 45
					slog.Info("Factory destroyed by cannon!", "idx", fIdx, "health", fact.Health)
				}
				return
			}
		}
	}

	// vs Tanks
	for tIdx := range g.tanks {
		tank := &g.tanks[tIdx]
		if tank.Active && tank.SinkingTimer == 0 {
			if math.Abs(bullet.X-tank.X) < 2.5 && math.Abs(bullet.Y-tank.Y) < 1.5 {
				bullet.Active = false
				tank.Health--
				slog.Info("Player bullet hit Tank", "tank_idx", tIdx, "health", tank.Health, "max_health", tank.MaxHealth)
				g.explosions = append(g.explosions, Explosion{X: int(math.Round(bullet.X)), Y: int(math.Round(bullet.Y)), Age: 0})

				if tank.Health <= 0 {
					tank.SinkingTimer = 45
					slog.Info("Tank destroyed by player cannon!", "tank_idx", tIdx)
				}
				return
			}
		}
	}

	// vs Static AA
	for saIdx := range g.staticAAs {
		sa := &g.staticAAs[saIdx]
		if sa.Active && sa.SinkingTimer == 0 {
			if math.Abs(bullet.X-sa.X) < 1.5 && math.Abs(bullet.Y-sa.Y) < 1.5 {
				bullet.Active = false
				sa.Health--
				slog.Info("Player bullet hit Static AA", "idx", saIdx, "health", sa.Health, "max_health", sa.MaxHealth)
				g.explosions = append(g.explosions, Explosion{X: int(math.Round(bullet.X)), Y: int(math.Round(bullet.Y)), Age: 0})

				if sa.Health <= 0 {
					sa.SinkingTimer = 45
					slog.Info("Static AA destroyed by player cannon!", "idx", saIdx)
				}
				return
			}
		}
	}
}

func (g *Game) checkMissileCollisions() {
	for i := 0; i < len(g.missiles); i++ {
		m := &g.missiles[i]
		if !m.Active {
			continue
		}

		if m.IsEnemy {
			g.checkEnemyMissileVsCarrier(m)
		} else {
			g.checkPlayerMissileVsTargets(m)
		}
	}
}

func (g *Game) checkEnemyMissileVsCarrier(m *Missile) {
	mx := int(math.Round(m.X))
	my := int(math.Round(m.Y))

	if mx < g.carrier.X || mx >= g.carrier.X+g.carrier.Width || my < g.carrier.Y || my >= g.carrier.Y+g.carrier.Height {
		return
	}

	m.Active = false
	g.carrier.Health -= 25.0
	slog.Warn("Enemy guided missile hit the Carrier!", "damage", 25.0, "remaining_health", g.carrier.Health)

	for ddx := -2; ddx <= 2; ddx++ {
		for ddy := -1; ddy <= 1; ddy++ {
			g.explosions = append(g.explosions, Explosion{X: mx + ddx, Y: my + ddy, Age: rand.Intn(4)})
		}
	}

	if g.carrier.Health <= 0 {
		g.carrier.Health = 0
		slog.Error("CRITICAL FAILURE: Aircraft Carrier Destroyed!")

		for ddx := -4; ddx <= 4; ddx++ {
			for ddy := -2; ddy <= 2; ddy++ {
				g.explosions = append(g.explosions, Explosion{
					X:   g.carrier.X + g.carrier.Width/2 + ddx,
					Y:   g.carrier.Y + g.carrier.Height/2 + ddy,
					Age: rand.Intn(5),
				})
			}
		}

		g.resetRound()
	}
}

func (g *Game) checkPlayerMissileVsTargets(m *Missile) {
	// vs Boats
	for j := 0; j < len(g.boats); j++ {
		boat := &g.boats[j]
		if !boat.Active {
			continue
		}
		if math.Abs(m.X-boat.X) < 5.5 && math.Abs(m.Y-boat.Y) < 1.5 {
			m.Active = false
			if boat.SinkingTimer == 0 {
				boat.SinkingTimer = 45
				boat.Health = 0
				slog.Info("Player guided missile hit Boat - delayed sinking initiated!", "boat_idx", j)
			} else {
				slog.Info("Player guided missile hit already-sinking Boat", "boat_idx", j)
			}
			g.explosions = append(g.explosions, Explosion{X: int(math.Round(m.X)), Y: int(math.Round(m.Y)), Age: 0})
			return
		}
	}

	// vs Factories
	for fIdx := range g.factories {
		fact := &g.factories[fIdx]
		if fact.Active && fact.SinkingTimer == 0 {
			if math.Abs(m.X-fact.X) < 8.5 && math.Abs(m.Y-fact.Y) < 2.5 {
				m.Active = false
				fact.SinkingTimer = 45
				fact.Health = 0
				slog.Info("Player guided missile hit Factory (CRITICAL HIT!)", "idx", fIdx)
				g.explosions = append(g.explosions, Explosion{X: int(math.Round(m.X)), Y: int(math.Round(m.Y)), Age: 0})
				return
			}
		}
	}

	// vs Tanks
	for tIdx := range g.tanks {
		tank := &g.tanks[tIdx]
		if tank.Active && tank.SinkingTimer == 0 {
			if math.Abs(m.X-tank.X) < 2.5 && math.Abs(m.Y-tank.Y) < 1.5 {
				m.Active = false
				tank.SinkingTimer = 45
				tank.Health = 0
				slog.Info("Player guided missile hit Tank (CRITICAL HIT!)", "tank_idx", tIdx)
				g.explosions = append(g.explosions, Explosion{X: int(math.Round(m.X)), Y: int(math.Round(m.Y)), Age: 0})
				return
			}
		}
	}

	// vs Static AA
	for saIdx := range g.staticAAs {
		sa := &g.staticAAs[saIdx]
		if sa.Active && sa.SinkingTimer == 0 {
			if math.Abs(m.X-sa.X) < 1.5 && math.Abs(m.Y-sa.Y) < 1.5 {
				m.Active = false
				sa.SinkingTimer = 45
				sa.Health = 0
				slog.Info("Player guided missile hit Static AA (CRITICAL HIT!)", "idx", saIdx)
				g.explosions = append(g.explosions, Explosion{X: int(math.Round(m.X)), Y: int(math.Round(m.Y)), Age: 0})
				return
			}
		}
	}
}

// checkPlayerBulletsVsEnemyMissiles handles manual missile interception by the player.
func (g *Game) checkPlayerBulletsVsEnemyMissiles() {
	for i := 0; i < len(g.bullets); i++ {
		bullet := &g.bullets[i]
		if !bullet.Active || bullet.IsEnemy {
			continue
		}
		for j := 0; j < len(g.missiles); j++ {
			m := &g.missiles[j]
			if !m.Active || !m.IsEnemy {
				continue
			}
			if math.Abs(bullet.X-m.X) < 1.5 && math.Abs(bullet.Y-m.Y) < 1.5 {
				bullet.Active = false
				m.Active = false
				g.explosions = append(g.explosions, Explosion{X: int(math.Round(m.X)), Y: int(math.Round(m.Y)), Age: 0})
				slog.Info("Player manual interception: Enemy missile shot down by aerial cannon!", "missile_idx", j, "bullet_idx", i)
				break
			}
		}
	}
}

// checkEnemyBulletsVsPlayerMissiles handles boat CIWS defense interception.
func (g *Game) checkEnemyBulletsVsPlayerMissiles() {
	for i := 0; i < len(g.bullets); i++ {
		bullet := &g.bullets[i]
		if !bullet.Active || !bullet.IsEnemy {
			continue
		}
		for j := 0; j < len(g.missiles); j++ {
			m := &g.missiles[j]
			if !m.Active || m.IsEnemy {
				continue
			}
			if math.Abs(bullet.X-m.X) < 1.5 && math.Abs(bullet.Y-m.Y) < 1.5 {
				if rand.Float64() < MissileDodgeChance {
					slog.Info("Missile successfully dodged enemy anti-aircraft projectile!", "missile_idx", j, "bullet_idx", i, "dodge_chance", MissileDodgeChance)
					break
				}
				bullet.Active = false
				m.Active = false
				g.explosions = append(g.explosions, Explosion{X: int(math.Round(m.X)), Y: int(math.Round(m.Y)), Age: 0})
				slog.Info("CIWS Interception Successful: Guided missile shot down by boat anti-air fire!", "missile_idx", j, "bullet_idx", i)
				break
			}
		}
	}
}

package main

import (
	"log/slog"
	"math"
	"math/rand"

	"github.com/gdamore/tcell/v2"
)

const (
	MaxLockOnRange     = 100.0 // Maximum distance helicopter radar can lock onto a target
	BoatDetectionRange = 25.0  // Distance at which incoming missile is visible/detectable by boat CIWS
	MissileDodgeChance = 0.35  // 35% probability of a missile dodging any incoming enemy bullet
	PlayerCannonRange  = 35.0  // Shorter range for player aerial cannon bullets
	BoatAARange        = 55.0  // Longer range for enemy boat standard AA flak
	MissileMaxRange    = 100.0 // Longest range for guided missiles
)

// updatePhysics updates positions, applies drag, checks bounds, and runs world elements
func (g *Game) updatePhysics() {
	g.Ticks++

	// 1. Helicopter-specific physics updates
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
		g.heli.VX = 0
		g.heli.VY = 0
	} else {
		// Consume fuel while airborne
		if g.heli.Fuel > 0 {
			g.heli.Fuel -= 0.05 // Lasts about 80 seconds at 25 FPS
			if g.heli.Fuel <= 0 {
				g.heli.Fuel = 0
				slog.Warn("Engine failure: Out of fuel")
			}
		}

		// Extremely low drag so helicopter slides and maintains momentum smoothly!
		drag := 0.99
		if g.heli.Fuel <= 0 {
			drag = 0.85 // Heavy drag if out of fuel (engine dies)
		}

		// Move helicopter
		g.heli.X += g.heli.VX
		g.heli.Y += g.heli.VY

		// Apply drag/friction
		g.heli.VX *= drag
		g.heli.VY *= drag

		// Cap maximum speed to keep flight controllable on a single screen (boosted to 1.2 for faster gameplay)
		speed := math.Sqrt(g.heli.VX*g.heli.VX + g.heli.VY*g.heli.VY)
		maxSpeed := 1.2 // cells per tick limit
		if speed > maxSpeed {
			ratio := maxSpeed / speed
			g.heli.VX *= ratio
			g.heli.VY *= ratio
		}

		// Handle crash if out of fuel and gliding to a halt over the ocean
		if g.heli.Fuel <= 0 {
			hx := int(math.Round(g.heli.X))
			hy := int(math.Round(g.heli.Y))
			onCarrier := hx >= g.carrier.X && hx < g.carrier.X+g.carrier.Width &&
				hy >= g.carrier.Y && hy < g.carrier.Y+g.carrier.Height

			if !onCarrier && speed < 0.02 {
				slog.Warn("Helicopter crashed into the ocean: Out of fuel", "x", g.heli.X, "y", g.heli.Y)

				// Spectacular crash: spawn major explosion circle
				for dx := -2; dx <= 2; dx++ {
					for dy := -1; dy <= 1; dy++ {
						g.explosions = append(g.explosions, Explosion{
							X:   hx + dx,
							Y:   hy + dy,
							Age: 0,
						})
					}
				}

				// Respawn player sitting on the aircraft carrier deck landing pad
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

		// Check boundaries (keep helicopter inside map region, leaving 4 lines for HUD)
		mapHeight := float64(g.height - 4)

		// Left boundary
		if g.heli.X < 1.0 {
			g.heli.X = 1.0
			g.heli.VX = -g.heli.VX * 0.4 // Soft bounce
		}
		// Right boundary
		if g.heli.X > float64(g.width-2) {
			g.heli.X = float64(g.width - 2)
			g.heli.VX = -g.heli.VX * 0.4
		}
		// Top boundary
		if g.heli.Y < 1.0 {
			g.heli.Y = 1.0
			g.heli.VY = -g.heli.VY * 0.4
		}
		// Bottom boundary (just above the HUD splitter)
		if g.heli.Y > mapHeight-2.0 {
			g.heli.Y = mapHeight - 2.0
			g.heli.VY = -g.heli.VY * 0.4
		}

		// Automatic landing when hovering slowly over the landing pad
		padX := g.carrier.X + g.carrier.Width/3
		padY := g.carrier.Y + g.carrier.Height/2
		aligned := int(math.Round(g.heli.X)) >= padX-1 && int(math.Round(g.heli.X)) <= padX+1 &&
			int(math.Round(g.heli.Y)) >= padY-1 && int(math.Round(g.heli.Y)) <= padY+1

		if aligned && speed < 0.12 && g.heli.TakeoffCooldown == 0 {
			g.heli.Landed = true
			g.heli.X = float64(padX)
			g.heli.Y = float64(padY)
			g.heli.VX = 0
			g.heli.VY = 0
			g.heli.TakeoffCooldown = 25 // 1 second debounce
			slog.Info("Auto-landed on carrier pad", "x", g.heli.X, "y", g.heli.Y)
		} else {
			// Animate rotor blades only when airborne or turning engine
			g.heli.RotorState = (g.heli.RotorState + 1) % len(rotorFrames)
		}
	}

	// 2. World Elements physics (Always active regardless of landing state)

	// Update weapon fire cooldown
	if g.heli.FireCooldown > 0 {
		g.heli.FireCooldown--
	}
	if g.heli.MissileCooldown > 0 {
		g.heli.MissileCooldown--
	}

	// Move active bullets and enforce weapon ranges
	for i := 0; i < len(g.bullets); i++ {
		b := &g.bullets[i]
		if !b.Active {
			continue
		}
		b.X += b.VX
		b.Y += b.VY

		// Deactivate if out of map bounds
		if b.X < 0 || b.X >= float64(g.width) ||
			b.Y < 0 || b.Y >= float64(g.height-4) {
			b.Active = false
			continue
		}

		// Enforce maximum travel ranges (cannons are shorter range, boat AA is longer range)
		dxVec := b.X - b.StartX
		dyVec := b.Y - b.StartY
		travelDist := math.Sqrt(dxVec*dxVec + dyVec*dyVec)

		if b.IsEnemy {
			if travelDist > BoatAARange {
				b.Active = false
			}
		} else {
			if travelDist > PlayerCannonRange {
				b.Active = false
			}
		}
	}

	// Move active guided missiles and apply homing/steering logic
	for i := 0; i < len(g.missiles); i++ {
		m := &g.missiles[i]
		if !m.Active {
			continue
		}

		if m.IsEnemy {
			// Target is the aircraft carrier's center
			targetX := float64(g.carrier.X + g.carrier.Width/2)
			targetY := float64(g.carrier.Y + g.carrier.Height/2)
			dxVec := targetX - m.X
			dyVec := targetY - m.Y
			dist := math.Sqrt(dxVec*dxVec + dyVec*dyVec)

			if dist > 0 {
				tx := dxVec / dist
				ty := dyVec / dist

				// Calculate current speed of the missile
				currentSpeed := math.Sqrt(m.VX*m.VX + m.VY*m.VY)

				// Accelerate the enemy missile speed towards 1.1 (increase speed by 0.03 per tick)
				newSpeed := currentSpeed + 0.03
				if newSpeed > 1.1 {
					newSpeed = 1.1
				}

				// Proportional homing: blend current velocity direction with vector to target
				m.VX = m.VX*0.92 + tx*newSpeed*0.08
				m.VY = m.VY*0.92 + ty*newSpeed*0.08
			}
		} else {
			// Find nearest active enemy boat target
			var targetBoat *Boat
			minDist := math.MaxFloat64
			for j := 0; j < len(g.boats); j++ {
				boat := &g.boats[j]
				if !boat.Active {
					continue
				}
				dxVec := boat.X - m.X
				dyVec := boat.Y - m.Y
				dist := math.Sqrt(dxVec*dxVec + dyVec*dyVec)
				if dist < minDist {
					minDist = dist
					targetBoat = boat
				}
			}

			if targetBoat != nil {
				// Compute normalized target unit vector (tx, ty)
				dxVec := targetBoat.X - m.X
				dyVec := targetBoat.Y - m.Y
				if minDist > 0 {
					tx := dxVec / minDist
					ty := dyVec / minDist

					// Calculate current speed of the missile
					currentSpeed := math.Sqrt(m.VX*m.VX + m.VY*m.VY)

					// Accelerate the missile speed towards 5.0 (increase speed by 0.20 per tick)
					newSpeed := currentSpeed + 0.20
					if newSpeed > 5.0 {
						newSpeed = 5.0
					}

					// Proportional homing: blend current velocity direction with vector to target at accelerated speed
					m.VX = m.VX*0.82 + tx*newSpeed*0.18
					m.VY = m.VY*0.82 + ty*newSpeed*0.18
				}

				// Boat CIWS defense: 10% chance to intercept incoming missiles within BoatDetectionRange
				if minDist < BoatDetectionRange && !m.InterceptionRolled && targetBoat.SinkingTimer == 0 {
					m.InterceptionRolled = true
					if rand.Float64() < 0.10 {
						bulletSpeed := 3.5 // Hyper-velocity anti-missile projectile (faster than standard AA)
						bvx := -(dxVec / minDist) * bulletSpeed
						bvy := -(dyVec / minDist) * bulletSpeed

						// Spawn the defensive enemy bullet directed at the missile
						spawned := false
						for k := 0; k < len(g.bullets); k++ {
							if !g.bullets[k].Active {
								g.bullets[k] = Bullet{X: targetBoat.X, Y: targetBoat.Y, StartX: targetBoat.X, StartY: targetBoat.Y, VX: bvx, VY: bvy, Active: true, IsEnemy: true, IsCountermeasure: true}
								spawned = true
								break
							}
						}
						if !spawned && len(g.bullets) < 24 {
							g.bullets = append(g.bullets, Bullet{X: targetBoat.X, Y: targetBoat.Y, StartX: targetBoat.X, StartY: targetBoat.Y, VX: bvx, VY: bvy, Active: true, IsEnemy: true, IsCountermeasure: true})
						}
						slog.Info("CIWS engaged: Boat launched defensive anti-missile countermeasure!", "boat_x", targetBoat.X, "missile_x", m.X)
					}
				}
			}
		}

		// Update position
		m.X += m.VX
		m.Y += m.VY

		// Deactivate if out of map bounds
		if m.X < 0 || m.X >= float64(g.width) || m.Y < 0 || m.Y >= float64(g.height-4) {
			m.Active = false
			continue
		}

		// Enforce maximum travel range for guided missiles
		dxVec := m.X - m.StartX
		dyVec := m.Y - m.StartY
		travelDist := math.Sqrt(dxVec*dxVec + dyVec*dyVec)
		if travelDist > MissileMaxRange {
			m.Active = false
		}
	}

	// Move active boats & shoot back at airborne player
	for i := 0; i < len(g.boats); i++ {
		boat := &g.boats[i]
		if !boat.Active {
			continue
		}

		// Handle delayed sinking
		if boat.SinkingTimer > 0 {
			boat.SinkingTimer--

			// Periodically spawn visual fire/smoke explosion effects on the burning ship!
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

				// Spawn massive hull breakage explosion cluster
				for dx := -5; dx <= 5; dx++ {
					for dy := -1; dy <= 1; dy++ {
						g.explosions = append(g.explosions, Explosion{
							X:   int(math.Round(boat.X)) + dx,
							Y:   int(math.Round(boat.Y)) + dy,
							Age: 0,
						})
					}
				}
				continue
			}
		}

		// Move boat (slow down if sinking)
		speedMult := 1.0
		if boat.SinkingTimer > 0 {
			speedMult = 0.25
		}
		boat.X += boat.VX * speedMult

		// Bounce boats off screen margins (adjusted for 11-cell wide boat size)
		if boat.X < 6 || boat.X > float64(g.width-7) {
			boat.VX = -boat.VX
			boat.X += boat.VX * speedMult
		}

		// Update boat fire cooldown (burning/sinking boats cannot fire back)
		if boat.SinkingTimer > 0 {
			continue
		}

		if boat.FireCooldown > 0 {
			boat.FireCooldown--
		} else if !g.heli.Landed && g.heli.Fuel > 0 && g.heli.Armor > 0 {
			// Only fire if helicopter is airborne and healthy
			dxVec := g.heli.X - boat.X
			dyVec := g.heli.Y - boat.Y
			dist := math.Sqrt(dxVec*dxVec + dyVec*dyVec)

			// Range check (boats shoot at player if within BoatAARange)
			if dist > 0 && dist < BoatAARange {
				bulletSpeed := 2.0 // Fast, matched projectile speed
				bvx := (dxVec / dist) * bulletSpeed
				bvy := (dyVec / dist) * bulletSpeed

				// Try to reuse inactive bullet
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

				// Reset fire cooldown (60 to 140 ticks)
				boat.FireCooldown = 60 + rand.Intn(80)
			}
		}

		// Handle boat guided missile firing (targets the aircraft carrier)
		if boat.MissileCooldown > 0 {
			boat.MissileCooldown--
		} else {
			// Find direction vector to the aircraft carrier center
			targetX := float64(g.carrier.X + g.carrier.Width/2)
			targetY := float64(g.carrier.Y + g.carrier.Height/2)
			dxVec := targetX - boat.X
			dyVec := targetY - boat.Y
			dist := math.Sqrt(dxVec*dxVec + dyVec*dyVec)

			if dist > 0 {
				initialSpeed := 0.3
				mvx := (dxVec / dist) * initialSpeed
				mvy := (dyVec / dist) * initialSpeed

				// Try to find an inactive missile slot to reuse
				spawned := false
				for k := 0; k < len(g.missiles); k++ {
					if !g.missiles[k].Active {
						g.missiles[k] = Missile{
							X:                  boat.X,
							Y:                  boat.Y,
							StartX:             boat.X,
							StartY:             boat.Y,
							VX:                 mvx,
							VY:                 mvy,
							Active:             true,
							InterceptionRolled: false,
							IsEnemy:            true,
						}
						spawned = true
						break
					}
				}
				if !spawned {
					g.missiles = append(g.missiles, Missile{
						X:                  boat.X,
						Y:                  boat.Y,
						StartX:             boat.X,
						StartY:             boat.Y,
						VX:                 mvx,
						VY:                 mvy,
						Active:             true,
						InterceptionRolled: false,
						IsEnemy:            true,
					})
				}
				slog.Info("Boat launched guided missile at Carrier!", "boat_idx", i, "boat_x", boat.X, "boat_y", boat.Y)
			}

			// Reset guided missile cooldown: 600 to 1000 ticks (approx 24-40 seconds)
			boat.MissileCooldown = 600 + rand.Intn(400)
		}
	}

	// Age active explosions
	activeExplosions := make([]Explosion, 0, len(g.explosions))
	for i := 0; i < len(g.explosions); i++ {
		g.explosions[i].Age++
		if g.explosions[i].Age < 15 {
			activeExplosions = append(activeExplosions, g.explosions[i])
		}
	}
	g.explosions = activeExplosions

	// Collision Detection: Bullets vs Boats / Player
	for i := 0; i < len(g.bullets); i++ {
		bullet := &g.bullets[i]
		if !bullet.Active {
			continue
		}

		if bullet.IsEnemy {
			// Enemy Bullet vs Player Helicopter
			if !g.heli.Landed && g.heli.Armor > 0 {
				// 3x3 collision hitbox around helicopter center (X, Y)
				if math.Abs(bullet.X-g.heli.X) < 1.5 && math.Abs(bullet.Y-g.heli.Y) < 1.5 {
					bullet.Active = false
					g.heli.Armor -= 15.0 // take 15 points of damage
					slog.Info("Enemy projectile hit Player", "damage", 15.0, "remaining_armor", g.heli.Armor)

					// Spawn small hit spark
					g.explosions = append(g.explosions, Explosion{X: int(math.Round(bullet.X)), Y: int(math.Round(bullet.Y)), Age: 0})

					if g.heli.Armor <= 0 {
						g.heli.Armor = 0
						slog.Warn("Helicopter destroyed", "x", g.heli.X, "y", g.heli.Y)
						// Spectacular crash: spawn major explosion circle
						hx := int(math.Round(g.heli.X))
						hy := int(math.Round(g.heli.Y))
						for dx := -2; dx <= 2; dx++ {
							for dy := -1; dy <= 1; dy++ {
								g.explosions = append(g.explosions, Explosion{
									X:   hx + dx,
									Y:   hy + dy,
									Age: 0,
								})
							}
						}

						// Respawn player sitting on the aircraft carrier deck
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
		} else {
			// Player Bullet vs Enemy Boats
			for j := 0; j < len(g.boats); j++ {
				boat := &g.boats[j]
				if !boat.Active {
					continue
				}

				// Collision window: boat is 11 cells wide (X-5 to X+5) and 3 rows high (Y-1 to Y+1)
				if math.Abs(bullet.X-boat.X) < 5.5 && math.Abs(bullet.Y-boat.Y) < 1.5 {
					bullet.Active = false

					// Bullet only deals damage if boat is not already sinking
					if boat.SinkingTimer == 0 {
						boat.Health--
						slog.Info("Player bullet hit Boat", "boat_idx", j, "health", boat.Health, "max_health", boat.MaxHealth)

						// Spawn tiny flash spark
						g.explosions = append(g.explosions, Explosion{X: int(math.Round(bullet.X)), Y: int(math.Round(bullet.Y)), Age: 0})

						if boat.Health <= 0 {
							boat.Active = false
							g.boatsSunk++
							slog.Info("Boat sunk by cannon round", "boat_idx", j, "total_sunk", g.boatsSunk)

							// Spawn massive hull breakage explosion cluster (adjusted for large boat size)
							for dx := -5; dx <= 5; dx++ {
								for dy := -1; dy <= 1; dy++ {
									g.explosions = append(g.explosions, Explosion{
										X:   int(math.Round(boat.X)) + dx,
										Y:   int(math.Round(boat.Y)) + dy,
										Age: 0,
									})
								}
							}
						}
					} else {
						slog.Info("Player bullet hit already-sinking Boat", "boat_idx", j)
						// Spawn tiny flash spark anyway
						g.explosions = append(g.explosions, Explosion{X: int(math.Round(bullet.X)), Y: int(math.Round(bullet.Y)), Age: 0})
					}
					break
				}
			}
		}
	}

	// Collision Detection: Missiles vs Boats / Carrier
	for i := 0; i < len(g.missiles); i++ {
		m := &g.missiles[i]
		if !m.Active {
			continue
		}

		if m.IsEnemy {
			// Enemy missile checking against Carrier deck bounds
			mx := int(math.Round(m.X))
			my := int(math.Round(m.Y))

			if mx >= g.carrier.X && mx < g.carrier.X+g.carrier.Width &&
				my >= g.carrier.Y && my < g.carrier.Y+g.carrier.Height {
				
				m.Active = false
				g.carrier.Health -= 25.0
				slog.Warn("Enemy guided missile hit the Carrier!", "damage", 25.0, "remaining_health", g.carrier.Health)

				// Spawn spectacular major explosion cluster on the carrier deck
				for dx := -2; dx <= 2; dx++ {
					for dy := -1; dy <= 1; dy++ {
						g.explosions = append(g.explosions, Explosion{
							X:   mx + dx,
							Y:   my + dy,
							Age: rand.Intn(4),
						})
					}
				}

				if g.carrier.Health <= 0 {
					g.carrier.Health = 0
					slog.Error("CRITICAL FAILURE: Aircraft Carrier Destroyed!")
					
					// Massive final explosion cluster on deck
					for dx := -4; dx <= 4; dx++ {
						for dy := -2; dy <= 2; dy++ {
							g.explosions = append(g.explosions, Explosion{
								X:   g.carrier.X + g.carrier.Width/2 + dx,
								Y:   g.carrier.Y + g.carrier.Height/2 + dy,
								Age: rand.Intn(5),
							})
						}
					}

					// Reset the entire round
					g.resetRound()
				}
			}
		} else {
			// Player missile against enemy boats
			for j := 0; j < len(g.boats); j++ {
				boat := &g.boats[j]
				if !boat.Active {
					continue
				}

				// Collision window: boat is 11 cells wide (X-5 to X+5) and 3 rows high (Y-1 to Y+1)
				if math.Abs(m.X-boat.X) < 5.5 && math.Abs(m.Y-boat.Y) < 1.5 {
					m.Active = false

					// Initiate delayed sinking sequence if not already sinking
					if boat.SinkingTimer == 0 {
						boat.SinkingTimer = 45 // 1.8 second burning delay
						boat.Health = 0
						slog.Info("Player guided missile hit Boat - delayed sinking initiated!", "boat_idx", j)
					} else {
						slog.Info("Player guided missile hit already-sinking Boat", "boat_idx", j)
					}

					// Spawn minor explosion at missile impact point
					g.explosions = append(g.explosions, Explosion{X: int(math.Round(m.X)), Y: int(math.Round(m.Y)), Age: 0})
					break
				}
			}
		}
	}

	// Collision Detection: Player Bullets vs Enemy Missiles (Player Manual Interception)
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

			// Check interception collision: if they are extremely close (distance < 1.5)
			if math.Abs(bullet.X-m.X) < 1.5 && math.Abs(bullet.Y-m.Y) < 1.5 {
				bullet.Active = false
				m.Active = false

				// Spawn interception explosion (mid-air spark)
				g.explosions = append(g.explosions, Explosion{X: int(math.Round(m.X)), Y: int(math.Round(m.Y)), Age: 0})
				slog.Info("Player manual interception: Enemy missile shot down by aerial cannon!", "missile_idx", j, "bullet_idx", i)
				break
			}
		}
	}

	// Collision Detection: Enemy Bullets vs Player Missiles (Boat CIWS Defense Interception)
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

			// Check interception collision: if they are extremely close (distance < 1.5)
			if math.Abs(bullet.X-m.X) < 1.5 && math.Abs(bullet.Y-m.Y) < 1.5 {
				// Roll for missile dodge probability
				if rand.Float64() < MissileDodgeChance {
					slog.Info("Missile successfully dodged enemy anti-aircraft projectile!", "missile_idx", j, "bullet_idx", i, "dodge_chance", MissileDodgeChance)
					continue // Guided missile successfully evades this bullet; both remain active!
				}

				bullet.Active = false
				m.Active = false

				// Spawn interception explosion (mid-air spark)
				g.explosions = append(g.explosions, Explosion{X: int(math.Round(m.X)), Y: int(math.Round(m.Y)), Age: 0})
				slog.Info("CIWS Interception Successful: Guided missile shot down by boat anti-air fire!", "missile_idx", j, "bullet_idx", i)
				break
			}
		}
	}


	// Progressive Respawn: If all boats are sunk, respawn them with progressive speed scaling!
	allSunk := true
	for i := 0; i < len(g.boats); i++ {
		if g.boats[i].Active {
			allSunk = false
			break
		}
	}
	if allSunk {
		slog.Info("All enemy boats sunk! Resetting with progressive speed increase", "speed_multiplier", 1.25)
		for i := 0; i < len(g.boats); i++ {
			g.boats[i].Active = true
			g.boats[i].Health = g.boats[i].MaxHealth
			g.boats[i].VX *= 1.25 // Scale up sailing speeds!
			g.boats[i].MissileCooldown = 300 + rand.Intn(300) // Reset cooldown
		}
	}
}

// handleKeyPress updates steering, thrust, weapons, and landing commands
func (g *Game) handleKeyPress(ev *tcell.EventKey) {
	key := ev.Key()
	ch := ev.Rune()

	// Pad alignment parameters
	padX := g.carrier.X + g.carrier.Width/3
	padY := g.carrier.Y + g.carrier.Height/2
	aligned := int(math.Round(g.heli.X)) >= padX-1 && int(math.Round(g.heli.X)) <= padX+1 &&
		int(math.Round(g.heli.Y)) >= padY-1 && int(math.Round(g.heli.Y)) <= padY+1

	// 1. Landing state actions
	if g.heli.Landed {
		// Take off on L, Space, up arrow, or W
		if ch == ' ' || key == tcell.KeyUp || ch == 'w' || ch == 'W' || ch == 'l' || ch == 'L' {
			if g.heli.TakeoffCooldown == 0 {
				g.heli.Landed = false
				g.heli.VY = -0.1 // Kickoff speed pointing upwards
				g.heli.TakeoffCooldown = 25 // 1 second debounce to prevent immediate landing
				slog.Info("Takeoff initiated", "x", g.heli.X, "y", g.heli.Y)
			}
		}
		return
	}

	// 2. Airborne state actions (thrust boosted to 0.18 for powerful acceleration)
	thrust := 0.18
	if g.heli.Fuel <= 0 {
		thrust = 0.0 // No engine power
	}

	// Try to Land on L/l key if aligned with the deck pad and moving slowly
	if (ch == 'l' || ch == 'L') && g.heli.TakeoffCooldown == 0 {
		speed := math.Sqrt(g.heli.VX*g.heli.VX + g.heli.VY*g.heli.VY)
		if aligned && speed < 0.25 {
			g.heli.Landed = true
			g.heli.X = float64(padX)
			g.heli.Y = float64(padY)
			g.heli.VX = 0
			g.heli.VY = 0
			g.heli.TakeoffCooldown = 25 // 1 second debounce
			slog.Info("Landed on carrier pad", "x", g.heli.X, "y", g.heli.Y)
			return
		}
	}

	// Fire Aerial Cannon on Spacebar
	if ch == ' ' && g.heli.FireCooldown == 0 && g.heli.Fuel > 0 {
		bulletSpeed := 2.0

		// Offset bullet spawn slightly forward based on direction to emit from the cockpit nose!
		bx := g.heli.X + dx[g.heli.Dir]*1.5
		by := g.heli.Y + dy[g.heli.Dir]*1.5

		bvx := dx[g.heli.Dir] * bulletSpeed
		bvy := dy[g.heli.Dir] * bulletSpeed

		// Try to reuse an inactive bullet first to minimize allocations
		spawned := false
		for i := 0; i < len(g.bullets); i++ {
			if !g.bullets[i].Active {
				g.bullets[i] = Bullet{X: bx, Y: by, StartX: bx, StartY: by, VX: bvx, VY: bvy, Active: true}
				spawned = true
				break
			}
		}
		// If none inactive and capacity permits, append
		if !spawned && len(g.bullets) < 16 {
			g.bullets = append(g.bullets, Bullet{X: bx, Y: by, StartX: bx, StartY: by, VX: bvx, VY: bvy, Active: true})
		}

		slog.Info("Aerial cannon fired", "dir", g.heli.Dir, "degrees", dirDegrees[g.heli.Dir])

		g.heli.FireCooldown = 4 // Set firing speed limits (6 shots per second)
	}

	// Fire Guided Missile on 'F', 'f', 'M', or 'm' key
	if (ch == 'f' || ch == 'F' || ch == 'm' || ch == 'M') && g.heli.MissileCooldown == 0 && g.heli.Fuel > 0 && g.heli.MissileAmmo > 0 {
		// Force lock-on check: target must be within +/- 45 degree forward aperture of view
		lockedBoat := g.getLockedBoat()
		if lockedBoat == nil {
			slog.Warn("Missile launch aborted: No target boat locked within +/- 45 degree forward aperture!")
			return
		}

		// Count active missiles to ensure maximum of 2 on screen at any time
		activeMissilesCount := 0
		for i := 0; i < len(g.missiles); i++ {
			if g.missiles[i].Active {
				activeMissilesCount++
			}
		}

		if activeMissilesCount < 2 {
			initialSpeed := 0.5
			mx := g.heli.X + dx[g.heli.Dir]*1.5
			my := g.heli.Y + dy[g.heli.Dir]*1.5

			mvx := dx[g.heli.Dir] * initialSpeed
			mvy := dy[g.heli.Dir] * initialSpeed

			// Try to reuse an inactive missile first
			spawned := false
			for i := 0; i < len(g.missiles); i++ {
				if !g.missiles[i].Active {
					g.missiles[i] = Missile{X: mx, Y: my, StartX: mx, StartY: my, VX: mvx, VY: mvy, Active: true}
					spawned = true
					break
				}
			}
			if !spawned && len(g.missiles) < 2 {
				g.missiles = append(g.missiles, Missile{X: mx, Y: my, StartX: mx, StartY: my, VX: mvx, VY: mvy, Active: true})
			}

			g.heli.MissileAmmo--
			slog.Info("Guided missile fired", "dir", g.heli.Dir, "degrees", dirDegrees[g.heli.Dir], "ammo_remaining", g.heli.MissileAmmo)

			g.heli.MissileCooldown = 12 // 0.48 second firing debounce
		}
	}

	switch key {
	case tcell.KeyLeft:
		g.heli.Dir = (g.heli.Dir - 1 + 8) % 8
	case tcell.KeyRight:
		g.heli.Dir = (g.heli.Dir + 1) % 8
	case tcell.KeyUp:
		g.heli.VX += dx[g.heli.Dir] * thrust
		g.heli.VY += dy[g.heli.Dir] * thrust
	case tcell.KeyDown:
		// Strong, responsive air brakes
		g.heli.VX *= 0.3
		g.heli.VY *= 0.3
	}

	// Fallback to WASD/wasd keys
	switch ch {
	case 'a', 'A':
		g.heli.Dir = (g.heli.Dir - 1 + 8) % 8
	case 'd', 'D':
		g.heli.Dir = (g.heli.Dir + 1) % 8
	case 'w', 'W':
		g.heli.VX += dx[g.heli.Dir] * thrust
		g.heli.VY += dy[g.heli.Dir] * thrust
	case 's', 'S':
		// Strong, responsive air brakes
		g.heli.VX *= 0.3
		g.heli.VY *= 0.3
	}
}

// getLockedBoat returns the nearest active healthy boat within the +/- 45 degree field of view of the helicopter
func (g *Game) getLockedBoat() *Boat {
	var lockedBoat *Boat
	minDist := math.MaxFloat64

	for i := 0; i < len(g.boats); i++ {
		boat := &g.boats[i]
		if !boat.Active || boat.SinkingTimer > 0 {
			continue
		}

		// Calculate vector from helicopter to boat
		dxVec := boat.X - g.heli.X
		dyVec := (boat.Y - g.heli.Y) * 2.0 // Adjust for terminal aspect ratio

		dist := math.Sqrt(dxVec*dxVec + dyVec*dyVec)
		if dist == 0 || dist > MaxLockOnRange {
			continue
		}

		// Helicopter heading unit vector in standard screen cells:
		// (hx, hy) where North is (0, -1) and East is (1, 0)
		hx := dx[g.heli.Dir]
		hy := dy[g.heli.Dir] * 2.0

		// Normalize heading vector
		hLen := math.Sqrt(hx*hx + hy*hy)
		if hLen > 0 {
			hx /= hLen
			hy /= hLen
		}

		// Normalized vector to boat:
		bx := dxVec / dist
		by := dyVec / dist

		// Dot product of heading vector and direction-to-boat vector
		dot := hx*bx + hy*by

		// Cosine of 45 degrees is 0.707.
		// If dot product > 0.707 (meaning the angle between vectors is less than 45 degrees),
		// then the boat is within the +/- 45 degree aperture of view!
		if dot >= 0.707 {
			if dist < minDist {
				minDist = dist
				lockedBoat = boat
			}
		}
	}

	return lockedBoat
}

// resetRound resets the entire game state when the carrier is destroyed
func (g *Game) resetRound() {
	slog.Info("Resetting round due to carrier destruction")

	// 1. Reset carrier health
	g.carrier.Health = 100.0

	// 2. Reset helicopter sitting on landing pad
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

	// 3. Clear projectiles
	g.bullets = g.bullets[:0]
	g.missiles = g.missiles[:0]

	// 4. Reset boats
	g.boatsSunk = 0
	for i := 0; i < len(g.boats); i++ {
		boat := &g.boats[i]
		boat.Active = true
		boat.Health = boat.MaxHealth
		boat.SinkingTimer = 0
		boat.FireCooldown = 60 + rand.Intn(80)
		boat.MissileCooldown = 200 + i*200
		
		// Reset velocity to initial absolute speed, preserving direction
		initialSpeed := 0.05
		if i == 1 {
			initialSpeed = 0.04
		} else if i == 2 {
			initialSpeed = 0.06
		}
		if boat.VX < 0 {
			boat.VX = -initialSpeed
		} else {
			boat.VX = initialSpeed
		}
	}
}


package main

import (
	"log/slog"
	"math"
	"math/rand"

	"github.com/gdamore/tcell/v2"
)

// updatePhysics updates positions, applies drag, checks bounds, and runs world elements
func (g *Game) updatePhysics() {
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

		// Cap maximum speed to keep flight controllable on a single screen
		speed := math.Sqrt(g.heli.VX*g.heli.VX + g.heli.VY*g.heli.VY)
		maxSpeed := 0.5 // cells per tick limit
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

	// Move active bullets
	for i := 0; i < len(g.bullets); i++ {
		if !g.bullets[i].Active {
			continue
		}
		g.bullets[i].X += g.bullets[i].VX
		g.bullets[i].Y += g.bullets[i].VY

		// Deactivate if out of map bounds
		if g.bullets[i].X < 0 || g.bullets[i].X >= float64(g.width) ||
			g.bullets[i].Y < 0 || g.bullets[i].Y >= float64(g.height-4) {
			g.bullets[i].Active = false
		}
	}

	// Move active boats & shoot back at airborne player
	for i := 0; i < len(g.boats); i++ {
		boat := &g.boats[i]
		if !boat.Active {
			continue
		}
		boat.X += boat.VX

		// Bounce boats off screen margins (adjusted for 11-cell wide boat size)
		if boat.X < 6 || boat.X > float64(g.width-7) {
			boat.VX = -boat.VX
			boat.X += boat.VX
		}

		// Update boat fire cooldown
		if boat.FireCooldown > 0 {
			boat.FireCooldown--
		} else if !g.heli.Landed && g.heli.Fuel > 0 && g.heli.Armor > 0 {
			// Only fire if helicopter is airborne and healthy
			dxVec := g.heli.X - boat.X
			dyVec := g.heli.Y - boat.Y
			dist := math.Sqrt(dxVec*dxVec + dyVec*dyVec)

			// Range check (boats shoot at player if within 45 cells)
			if dist > 0 && dist < 45 {
				bulletSpeed := 0.22 // Dodgeable, slow projectile
				bvx := (dxVec / dist) * bulletSpeed
				bvy := (dyVec / dist) * bulletSpeed

				// Try to reuse inactive bullet
				spawned := false
				for k := 0; k < len(g.bullets); k++ {
					if !g.bullets[k].Active {
						g.bullets[k] = Bullet{X: boat.X, Y: boat.Y, VX: bvx, VY: bvy, Active: true, IsEnemy: true}
						spawned = true
						break
					}
				}
				if !spawned && len(g.bullets) < 24 {
					g.bullets = append(g.bullets, Bullet{X: boat.X, Y: boat.Y, VX: bvx, VY: bvy, Active: true, IsEnemy: true})
				}

				slog.Info("Boat fired anti-aircraft projectile", "boat_idx", i, "x", boat.X, "y", boat.Y)

				// Reset fire cooldown (60 to 140 ticks)
				boat.FireCooldown = 60 + rand.Intn(80)
			}
		}
	}

	// Age active explosions
	activeExplosions := make([]Explosion, 0, len(g.explosions))
	for i := 0; i < len(g.explosions); i++ {
		g.explosions[i].Age++
		if g.explosions[i].Age < 10 {
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
					boat.Health--
					slog.Info("Player bullet hit Boat", "boat_idx", j, "health", boat.Health, "max_health", boat.MaxHealth)

					// Spawn tiny flash spark
					g.explosions = append(g.explosions, Explosion{X: int(math.Round(bullet.X)), Y: int(math.Round(bullet.Y)), Age: 0})

					if boat.Health <= 0 {
						boat.Active = false
						g.boatsSunk++
						slog.Info("Boat sunk", "boat_idx", j, "total_sunk", g.boatsSunk)

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
					break
				}
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

	// 2. Airborne state actions
	thrust := 0.08
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
		bulletSpeed := 1.0

		// Offset bullet spawn slightly forward based on direction to emit from the cockpit nose!
		bx := g.heli.X + dx[g.heli.Dir]*1.5
		by := g.heli.Y + dy[g.heli.Dir]*1.5

		bvx := dx[g.heli.Dir] * bulletSpeed
		bvy := dy[g.heli.Dir] * bulletSpeed

		// Try to reuse an inactive bullet first to minimize allocations
		spawned := false
		for i := 0; i < len(g.bullets); i++ {
			if !g.bullets[i].Active {
				g.bullets[i] = Bullet{X: bx, Y: by, VX: bvx, VY: bvy, Active: true}
				spawned = true
				break
			}
		}
		// If none inactive and capacity permits, append
		if !spawned && len(g.bullets) < 16 {
			g.bullets = append(g.bullets, Bullet{X: bx, Y: by, VX: bvx, VY: bvy, Active: true})
		}

		slog.Info("Aerial cannon fired", "dir", g.heli.Dir, "degrees", dirDegrees[g.heli.Dir])

		g.heli.FireCooldown = 4 // Set firing speed limits (6 shots per second)
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

package game

import (
	"log/slog"
	"math"

	"github.com/gdamore/tcell/v2"
)

// handleKeyPress updates steering, thrust, weapons, and landing commands.
func (g *Game) handleKeyPress(ev *tcell.EventKey) {
	key := ev.Key()
	ch := ev.Rune()

	if g.heli.Armor <= 0 || g.heli.RespawnTimer > 0 || g.heli.ReturningToCarrier || g.carrierDestroying {
		return
	}

	padX := g.carrier.X + g.carrier.Width/3
	padY := g.carrier.Y + g.carrier.Height/2
	aligned := int(math.Round(g.heli.X)) >= padX-1 && int(math.Round(g.heli.X)) <= padX+1 &&
		int(math.Round(g.heli.Y)) >= padY-1 && int(math.Round(g.heli.Y)) <= padY+1

	if g.heli.Landed {
		if ch == ' ' || key == tcell.KeyUp || ch == 'w' || ch == 'W' || ch == 'l' || ch == 'L' {
			if g.heli.TakeoffCooldown == 0 {
				g.heli.Landed = false
				g.heli.VY = -0.1
				g.heli.TakeoffCooldown = 25
				slog.Info("Takeoff initiated", "x", g.heli.X, "y", g.heli.Y)
			}
		}
		return
	}

	thrust := 0.18
	if g.heli.Fuel <= 0 {
		thrust = 0.0
	}

	if (ch == 'l' || ch == 'L') && g.heli.TakeoffCooldown == 0 {
		speed := math.Sqrt(g.heli.VX*g.heli.VX + g.heli.VY*g.heli.VY)
		if aligned && speed < 0.25 {
			g.heli.Landed = true
			g.heli.X = float64(padX)
			g.heli.Y = float64(padY)
			g.heli.VX = 0
			g.heli.VY = 0
			g.heli.TakeoffCooldown = 25
			slog.Info("Landed on carrier pad", "x", g.heli.X, "y", g.heli.Y)
			return
		}
	}

	if ch == ' ' && g.heli.FireCooldown == 0 && g.heli.CannonJammed == 0 && g.heli.Fuel > 0 {
		bulletSpeed := 2.0
		bx := g.heli.X + dx[g.heli.Dir]*1.5
		by := g.heli.Y + dy[g.heli.Dir]*1.5
		bvx := dx[g.heli.Dir] * bulletSpeed
		bvy := dy[g.heli.Dir] * bulletSpeed

		g.spawnPlayerBullet(bx, by, bvx, bvy)
		slog.Info("Aerial cannon fired", "dir", g.heli.Dir, "degrees", dirDegrees[g.heli.Dir])
		PlaySound("laser")
		g.heli.FireCooldown = 4

		const maxHeat = 20
		g.heli.CannonHeat += 5
		if g.heli.CannonHeat >= maxHeat {
			g.heli.CannonHeat = 0
			g.heli.CannonJammed = 60 // 2.4 seconds at 25 FPS
			g.heli.Armor -= 5.0
			if g.heli.Armor < 0 {
				g.heli.Armor = 0
			}
			slog.Warn("Cannon overheated! Barrel jammed.", "jam_ticks", 60, "armor_remaining", g.heli.Armor)
		}
	}

	if (ch == 'f' || ch == 'F' || ch == 'm' || ch == 'M') && g.heli.MissileCooldown == 0 && g.heli.Fuel > 0 && g.heli.MissileAmmo > 0 {
		lockedBoat, lockedFactory, lockedTank, lockedStaticAA := g.lockedBoat, g.lockedFactory, g.lockedTank, g.lockedStaticAA
		if lockedBoat == nil && lockedFactory == nil && lockedTank == nil && lockedStaticAA == nil {
			slog.Warn("Missile launch aborted: No target locked within +/- 45 degree forward aperture!")
			return
		}

		activeMissilesCount := 0
		for i := 0; i < len(g.missiles); i++ {
			if g.missiles[i].Active && !g.missiles[i].IsEnemy && !g.missiles[i].IsCarrier {
				activeMissilesCount++
			}
		}

		if activeMissilesCount < 2 {
			initialSpeed := 0.5
			mx := g.heli.X + dx[g.heli.Dir]*1.5
			my := g.heli.Y + dy[g.heli.Dir]*1.5
			mvx := dx[g.heli.Dir] * initialSpeed
			mvy := dy[g.heli.Dir] * initialSpeed

			g.spawnPlayerMissile(mx, my, mvx, mvy)
			g.heli.MissileAmmo--
			slog.Info("Guided missile fired", "dir", g.heli.Dir, "degrees", dirDegrees[g.heli.Dir], "ammo_remaining", g.heli.MissileAmmo)
			PlaySound("missile")
			g.heli.MissileCooldown = 12
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
		g.heli.VX *= 0.3
		g.heli.VY *= 0.3
	}

	switch ch {
	case 'a', 'A':
		g.heli.Dir = (g.heli.Dir - 1 + 8) % 8
	case 'd', 'D':
		g.heli.Dir = (g.heli.Dir + 1) % 8
	case 'w', 'W':
		g.heli.VX += dx[g.heli.Dir] * thrust
		g.heli.VY += dy[g.heli.Dir] * thrust
	case 's', 'S':
		g.heli.VX *= 0.3
		g.heli.VY *= 0.3
	}
}

// applyJoystickInput processes joystick axes and buttons each physics tick
func (g *Game) applyJoystickInput() {
	if g.heli.Armor <= 0 || g.heli.RespawnTimer > 0 {
		return
	}

	// Combine input from left stick (0,1) and dpad (6,7)
	// Right stick (3,4) is reserved for acceleration
	var tx, ty float64

	tx += float64(g.joystickAxes[0]) / 32767.0
	ty += float64(g.joystickAxes[1]) / 32767.0
	tx += float64(g.joystickAxes[6]) / 32767.0
	ty += float64(g.joystickAxes[7]) / 32767.0

	if tx > 1 {
		tx = 1
	} else if tx < -1 {
		tx = -1
	}
	if ty > 1 {
		ty = 1
	} else if ty < -1 {
		ty = -1
	}

	// Axis 4 is right stick Y for acceleration (pulling back = forward, pushing forward = brake)
	rightStickY := float64(g.joystickAxes[4]) / 32767.0

	// Axis 5 is guns trigger (fire when > 0.5 threshold)
	gunsTrigger := float64(g.joystickAxes[5]) / 32767.0

	padX := g.carrier.X + g.carrier.Width/3
	padY := g.carrier.Y + g.carrier.Height/2
	aligned := int(math.Round(g.heli.X)) >= padX-1 && int(math.Round(g.heli.X)) <= padX+1 &&
		int(math.Round(g.heli.Y)) >= padY-1 && int(math.Round(g.heli.Y)) <= padY+1

	// Handle landing/takeoff (button 0 = A button)
	if g.joystickButtons[0] {
		if g.heli.Landed {
			if g.heli.TakeoffCooldown == 0 {
				g.heli.Landed = false
				g.heli.VY = -0.1
				g.heli.TakeoffCooldown = 25
				slog.Info("Takeoff initiated", "x", g.heli.X, "y", g.heli.Y)
			}
		} else if g.heli.TakeoffCooldown == 0 {
			speed := math.Sqrt(g.heli.VX*g.heli.VX + g.heli.VY*g.heli.VY)
			if aligned && speed < 0.25 {
				g.heli.Landed = true
				g.heli.X = float64(padX)
				g.heli.Y = float64(padY)
				g.heli.VX = 0
				g.heli.VY = 0
				g.heli.TakeoffCooldown = 25
				slog.Info("Landed on carrier pad", "x", g.heli.X, "y", g.heli.Y)
			}
		}
		return
	}

	if g.heli.Landed {
		return
	}

	// Only apply movement thrust when flying
	thrust := 0.18
	if g.heli.Fuel <= 0 {
		thrust = 0.0
	}

	if tx*tx+ty*ty > 0.01 {
		// Apply thrust in the direction of stick input
		g.heli.VX += tx * thrust * 0.2
		g.heli.VY += ty * thrust * 0.2
	}

	// Continuous rotation: left = counterclockwise, right = clockwise
	if g.heli.RotationCooldown == 0 {
		if tx < -0.3 {
			g.heli.Dir = (g.heli.Dir - 1 + 8) % 8
			g.heli.RotationCooldown = 4
		} else if tx > 0.3 {
			g.heli.Dir = (g.heli.Dir + 1) % 8
			g.heli.RotationCooldown = 4
		}
	}

	// Right stick Y: pull back = accelerate forward, push forward = brake
	if rightStickY < -0.3 {
		// Pulling back: accelerate forward
		g.heli.VX += dx[g.heli.Dir] * thrust
		g.heli.VY += dy[g.heli.Dir] * thrust
	} else if rightStickY > 0.3 {
		// Pushing forward: brake
		g.heli.VX *= 0.3
		g.heli.VY *= 0.3
	}

	// Cannon fire: button 2 (X button), button 6 (LB), or guns trigger (axis 5)
	if (g.joystickButtons[2] || g.joystickButtons[6] || gunsTrigger > 0.5) && g.heli.FireCooldown == 0 && g.heli.Fuel > 0 {
		bulletSpeed := 2.0
		bx := g.heli.X + dx[g.heli.Dir]*1.5
		by := g.heli.Y + dy[g.heli.Dir]*1.5
		bvx := dx[g.heli.Dir] * bulletSpeed
		bvy := dy[g.heli.Dir] * bulletSpeed

		g.spawnPlayerBullet(bx, by, bvx, bvy)
		slog.Info("Aerial cannon fired (joystick)", "dir", g.heli.Dir, "degrees", dirDegrees[g.heli.Dir])
		PlaySound("laser")
		g.heli.FireCooldown = 4
	}

	// Missile: button 1 (B button), button 3 (Y button), or button 7 (RB)
	if (g.joystickButtons[1] || g.joystickButtons[3] || g.joystickButtons[7]) && g.heli.MissileCooldown == 0 && g.heli.Fuel > 0 && g.heli.MissileAmmo > 0 {
		lockedBoat, lockedFactory, lockedTank, lockedStaticAA := g.lockedBoat, g.lockedFactory, g.lockedTank, g.lockedStaticAA
		if lockedBoat == nil && lockedFactory == nil && lockedTank == nil && lockedStaticAA == nil {
			slog.Warn("Missile launch aborted: No target locked within +/- 45 degree forward aperture!")
			return
		}

		activeMissilesCount := 0
		for i := 0; i < len(g.missiles); i++ {
			if g.missiles[i].Active && !g.missiles[i].IsEnemy && !g.missiles[i].IsCarrier {
				activeMissilesCount++
			}
		}

		if activeMissilesCount < 2 {
			initialSpeed := 0.5
			mx := g.heli.X + dx[g.heli.Dir]*1.5
			my := g.heli.Y + dy[g.heli.Dir]*1.5
			mvx := dx[g.heli.Dir] * initialSpeed
			mvy := dy[g.heli.Dir] * initialSpeed

			g.spawnPlayerMissile(mx, my, mvx, mvy)
			g.heli.MissileAmmo--
			slog.Info("Guided missile fired (joystick)", "dir", g.heli.Dir, "degrees", dirDegrees[g.heli.Dir], "ammo_remaining", g.heli.MissileAmmo)
			PlaySound("missile")
			g.heli.MissileCooldown = 12
		}
	}
}

// getLockedTarget returns the nearest active healthy target within the +/- 45 degree field of view.
func (g *Game) getLockedTarget() (*Boat, *Factory, *Tank, *StaticAA) {
	var lockedBoat *Boat
	var lockedFactory *Factory
	var lockedTank *Tank
	var lockedStaticAA *StaticAA
	minDist := math.MaxFloat64

	hx := dx[g.heli.Dir]
	hy := dy[g.heli.Dir] * 2.0
	hLen := math.Sqrt(hx*hx + hy*hy)
	if hLen > 0 {
		hx /= hLen
		hy /= hLen
	}

	for i := range g.boats {
		boat := &g.boats[i]
		if !boat.Active || boat.SinkingTimer > 0 {
			continue
		}
		dxVec := boat.X - g.heli.X
		dyVec := (boat.Y - g.heli.Y) * 2.0
		dist := math.Sqrt(dxVec*dxVec + dyVec*dyVec)
		if dist == 0 || dist > MaxLockOnRange {
			continue
		}
		bx := dxVec / dist
		by := dyVec / dist
		if hx*bx+hy*by >= 0.707 && dist < minDist {
			minDist = dist
			lockedBoat = boat
			lockedFactory = nil
			lockedTank = nil
			lockedStaticAA = nil
		}
	}

	for fIdx := range g.factories {
		fact := &g.factories[fIdx]
		if !fact.Active || fact.SinkingTimer > 0 {
			continue
		}
		dxVec := fact.X - g.heli.X
		dyVec := (fact.Y - g.heli.Y) * 2.0
		dist := math.Sqrt(dxVec*dxVec + dyVec*dyVec)
		if dist <= 0 || dist > MaxLockOnRange {
			continue
		}
		bx := dxVec / dist
		by := dyVec / dist
		if hx*bx+hy*by >= 0.707 && dist < minDist {
			minDist = dist
			lockedBoat = nil
			lockedFactory = fact
			lockedTank = nil
			lockedStaticAA = nil
		}
	}

	for tIdx := range g.tanks {
		tank := &g.tanks[tIdx]
		if !tank.Active || tank.SinkingTimer > 0 {
			continue
		}
		dxVec := tank.X - g.heli.X
		dyVec := (tank.Y - g.heli.Y) * 2.0
		dist := math.Sqrt(dxVec*dxVec + dyVec*dyVec)
		if dist <= 0 || dist > MaxLockOnRange {
			continue
		}
		bx := dxVec / dist
		by := dyVec / dist
		if hx*bx+hy*by >= 0.707 && dist < minDist {
			minDist = dist
			lockedBoat = nil
			lockedFactory = nil
			lockedTank = tank
			lockedStaticAA = nil
		}
	}

	for saIdx := range g.staticAAs {
		sa := &g.staticAAs[saIdx]
		if !sa.Active || sa.SinkingTimer > 0 {
			continue
		}
		dxVec := sa.X - g.heli.X
		dyVec := (sa.Y - g.heli.Y) * 2.0
		dist := math.Sqrt(dxVec*dxVec + dyVec*dyVec)
		if dist <= 0 || dist > MaxLockOnRange {
			continue
		}
		bx := dxVec / dist
		by := dyVec / dist
		if hx*bx+hy*by >= 0.707 && dist < minDist {
			minDist = dist
			lockedBoat = nil
			lockedFactory = nil
			lockedTank = nil
			lockedStaticAA = sa
		}
	}

	return lockedBoat, lockedFactory, lockedTank, lockedStaticAA
}

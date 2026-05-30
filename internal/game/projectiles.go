package game

import (
	"log/slog"
	"math"
	"math/rand"
)

// updateProjectiles moves bullets and guided missiles, applying range limits and homing logic.
func (g *Game) updateProjectiles() {
	g.updateBullets()
	g.updateMissiles()
}

func (g *Game) updateBullets() {
	for i := 0; i < len(g.bullets); i++ {
		b := &g.bullets[i]
		if !b.Active {
			continue
		}
		b.X += b.VX
		b.Y += b.VY

		if b.X < 0 || b.X >= float64(g.width) || b.Y < 0 || b.Y >= float64(g.height-4) {
			b.Active = false
			continue
		}

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
}

func (g *Game) updateMissiles() {
	for i := 0; i < len(g.missiles); i++ {
		m := &g.missiles[i]
		if !m.Active {
			continue
		}

		if m.IsEnemy {
			g.homeMissileToCarrier(m)
		} else {
			g.homeMissileToTarget(m)
		}

		m.X += m.VX
		m.Y += m.VY

		if m.X < 0 || m.X >= float64(g.width) || m.Y < 0 || m.Y >= float64(g.height-4) {
			m.Active = false
			continue
		}

		dxVec := m.X - m.StartX
		dyVec := m.Y - m.StartY
		if math.Sqrt(dxVec*dxVec+dyVec*dyVec) > MissileMaxRange {
			m.Active = false
		}
	}
}

func (g *Game) homeMissileToCarrier(m *Missile) {
	targetX := float64(g.carrier.X + g.carrier.Width/2)
	targetY := float64(g.carrier.Y + g.carrier.Height/2)
	dxVec := targetX - m.X
	dyVec := targetY - m.Y
	dist := math.Sqrt(dxVec*dxVec + dyVec*dyVec)
	if dist <= 0 {
		return
	}

	tx := dxVec / dist
	ty := dyVec / dist

	currentSpeed := math.Sqrt(m.VX*m.VX + m.VY*m.VY)
	newSpeed := currentSpeed + 0.03
	if newSpeed > 1.1 {
		newSpeed = 1.1
	}

	m.VX = m.VX*0.92 + tx*newSpeed*0.08
	m.VY = m.VY*0.92 + ty*newSpeed*0.08
}

func (g *Game) homeMissileToTarget(m *Missile) {
	var targetX, targetY float64
	var hasTarget bool
	var isBoat bool
	var targetBoat *Boat
	minDist := math.MaxFloat64

	for j := range g.boats {
		boat := &g.boats[j]
		if !boat.Active || boat.SinkingTimer > 0 {
			continue
		}
		dxVec := boat.X - m.X
		dyVec := boat.Y - m.Y
		dist := math.Sqrt(dxVec*dxVec + dyVec*dyVec)
		if dist < minDist {
			minDist = dist
			targetX = boat.X
			targetY = boat.Y
			targetBoat = boat
			isBoat = true
			hasTarget = true
		}
	}

	for fIdx := range g.factories {
		fact := &g.factories[fIdx]
		if !fact.Active || fact.SinkingTimer > 0 {
			continue
		}
		dxVec := fact.X - m.X
		dyVec := fact.Y - m.Y
		dist := math.Sqrt(dxVec*dxVec + dyVec*dyVec)
		if dist < minDist {
			minDist = dist
			targetX = fact.X
			targetY = fact.Y
			targetBoat = nil
			isBoat = false
			hasTarget = true
		}
	}

	for tIdx := range g.tanks {
		tank := &g.tanks[tIdx]
		if !tank.Active || tank.SinkingTimer > 0 {
			continue
		}
		dxVec := tank.X - m.X
		dyVec := tank.Y - m.Y
		dist := math.Sqrt(dxVec*dxVec + dyVec*dyVec)
		if dist < minDist {
			minDist = dist
			targetX = tank.X
			targetY = tank.Y
			targetBoat = nil
			isBoat = false
			hasTarget = true
		}
	}

	for saIdx := range g.staticAAs {
		sa := &g.staticAAs[saIdx]
		if !sa.Active || sa.SinkingTimer > 0 {
			continue
		}
		dxVec := sa.X - m.X
		dyVec := sa.Y - m.Y
		dist := math.Sqrt(dxVec*dxVec + dyVec*dyVec)
		if dist < minDist {
			minDist = dist
			targetX = sa.X
			targetY = sa.Y
			targetBoat = nil
			isBoat = false
			hasTarget = true
		}
	}

	if !hasTarget {
		return
	}

	dxVec := targetX - m.X
	dyVec := targetY - m.Y
	if minDist > 0 {
		tx := dxVec / minDist
		ty := dyVec / minDist

		currentSpeed := math.Sqrt(m.VX*m.VX + m.VY*m.VY)
		newSpeed := currentSpeed + 0.20
		if newSpeed > 5.0 {
			newSpeed = 5.0
		}

		m.VX = m.VX*0.82 + tx*newSpeed*0.18
		m.VY = m.VY*0.82 + ty*newSpeed*0.18
	}

	// Boat CIWS: 10% chance to intercept incoming missiles within BoatDetectionRange
	if isBoat && targetBoat != nil && minDist < BoatDetectionRange && !m.InterceptionRolled {
		m.InterceptionRolled = true
		if rand.Float64() < 0.10 {
			bulletSpeed := 3.5
			bvx := -(dxVec / minDist) * bulletSpeed
			bvy := -(dyVec / minDist) * bulletSpeed

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

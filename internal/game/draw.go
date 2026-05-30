package game

import (
	"fmt"
	"math"

	"github.com/gdamore/tcell/v2"
)

func (g *Game) getCoastlineThreshold(y int) float64 {
	h := float64(g.worldHeight)
	if h <= 0 {
		h = 1
	}
	// Organic wiggle using combined trigonometric waves
	wiggle := math.Sin(float64(y)*0.7)*2.0 + math.Cos(float64(y)*0.3)*1.0
	
	// Sine-curve bay coast: wide water bay in the center (y ≈ h/2), wrapping around north and south
	return float64(g.worldWidth)/3.0 + math.Sin(float64(y)/h*math.Pi)*(float64(g.worldWidth)/2.2) + wiggle
}

func (g *Game) getCoastlineStyle(x, y int) (bool, bool) {
	if !g.island.Active {
		return false, false
	}
	threshold := g.getCoastlineThreshold(y)
	if float64(x) >= threshold {
		// Sand shore is the first 3 cells of the coastline landmass
		isSand := float64(x) < threshold+3.0
		return true, isSand
	}
	return false, false
}

func (g *Game) isRoad(x, y int) bool {
	h := g.worldHeight
	w := g.worldWidth

	// Vertical road center: x == w - 15. Width: 3 cells (x in [w-16, w-14]) from y = h/8 to h*7/8.
	if x >= w-16 && x <= w-14 && y >= h/8 && y <= h*7/8 {
		return true
	}

	// Horizontal road center: y == h/2. Width: 3 cells (y in [h/2-1, h/2+1]) from x = w-15 to w-7.
	if y >= h/2-1 && y <= h/2+1 && x >= w-15 && x <= w-7 {
		return true
	}

	return false
}

// getMapStyle returns background colors dynamically for transparency rendering
func (g *Game) getMapStyle(x, y int) tcell.Style {
	navyBlue := tcell.ColorNames["navy"]
	lightBlue := tcell.ColorNames["dodgerblue"]

	// Check if the coordinate falls on the aircraft carrier deck
	if x >= g.carrier.X && x < g.carrier.X+g.carrier.Width &&
		y >= g.carrier.Y && y < g.carrier.Y+g.carrier.Height {

		cy := y - g.carrier.Y
		cx := x - g.carrier.X

		// Create ship-tapered edges
		leftTaper := (cy == 0 && cx < 4) || (cy == g.carrier.Height-1 && cx < 4)
		rightTaper := (cy == 0 && cx >= g.carrier.Width-4) || (cy == g.carrier.Height-1 && cx >= g.carrier.Width-4)

		if !leftTaper && !rightTaper {
			return tcell.StyleDefault.Background(tcell.ColorNames["grey"]).Foreground(tcell.ColorNames["white"])
		}
	}

	// Check if the coordinate falls on the bay coastline
	if isLand, isSand := g.getCoastlineStyle(x, y); isLand {
		if !isSand && g.isRoad(x, y) {
			// Asphalt/Road background
			return tcell.StyleDefault.Background(tcell.ColorNames["dimgray"])
		}
		if isSand {
			// Sand shore style
			return tcell.StyleDefault.Background(tcell.ColorNames["olive"]).Foreground(tcell.ColorNames["khaki"])
		} else {
			// Grassy interior style
			return tcell.StyleDefault.Background(tcell.ColorNames["darkgreen"]).Foreground(tcell.ColorNames["limegreen"])
		}
	}

	// Ocean styling with pseudo-random wave patterns (static coordinates map to waves)
	isWave := (x*9+y*13)%23 == 0
	if isWave {
		return tcell.StyleDefault.Background(navyBlue).Foreground(lightBlue)
	}

	return tcell.StyleDefault.Background(navyBlue).Foreground(navyBlue)
}

func (g *Game) getCameraOffset() (int, int) {
	return g.camX, g.camY
}

// draw handles screen rendering
func (g *Game) draw() {
	camX, camY := g.getCameraOffset()
	// 1. Draw Ocean Background & Aircraft Carrier
	for sy := 0; sy < g.height-4; sy++ {
		for sx := 0; sx < g.width; sx++ {
			vx := sx + camX
			vy := sy + camY
			style := g.getMapStyle(vx, vy)
			r := ' '

			// Check if the coordinate falls on the aircraft carrier deck
			if vx >= g.carrier.X && vx < g.carrier.X+g.carrier.Width &&
				vy >= g.carrier.Y && vy < g.carrier.Y+g.carrier.Height {

				cy := vy - g.carrier.Y
				cx := vx - g.carrier.X

				leftTaper := (cy == 0 && cx < 4) || (cy == g.carrier.Height-1 && cx < 4)
				rightTaper := (cy == 0 && cx >= g.carrier.Width-4) || (cy == g.carrier.Height-1 && cx >= g.carrier.Width-4)

				if !leftTaper && !rightTaper {
					// Mid-deck runway stripes
					if cy == g.carrier.Height/2 && cx > 3 && cx < g.carrier.Width-3 && cx%3 != 0 {
						r = '-'
						style = style.Foreground(tcell.ColorNames["yellow"])
					}

					// Draw landing circle & H pad
					padX := g.carrier.Width / 3
					padY := g.carrier.Height / 2
					if cx >= padX-2 && cx <= padX+2 && cy >= padY-1 && cy <= padY+1 {
						style = style.Foreground(tcell.ColorNames["yellow"])
						if cx == padX && cy == padY {
							r = 'H'
						} else if cx == padX-2 || cx == padX+2 {
							r = '|'
						} else if cy == padY-1 {
							r = '¯'
						} else if cy == padY+1 {
							r = '_'
						}
					}
				}
			} else if isLand, isSand := g.getCoastlineStyle(vx, vy); isLand {
				if !isSand && g.isRoad(vx, vy) {
					// Asphalt/Road styling
					hVal := g.worldHeight
					wVal := g.worldWidth
					isVerticalCenter := (vx == wVal - 15)
					isHorizontalCenter := (vy == hVal / 2)
					
					if (isVerticalCenter && vy >= hVal/6 && vy <= hVal*5/6 && vy%2 == 0) ||
					   (isHorizontalCenter && vx >= wVal-15 && vx <= wVal-8 && vx%2 == 0) {
						if isVerticalCenter {
							r = '|'
						} else {
							r = '-'
						}
						style = tcell.StyleDefault.Background(tcell.ColorNames["dimgray"]).Foreground(tcell.ColorNames["yellow"])
					} else {
						// Subtle concrete grain dots
						hash := (vx*13 + vy*17) % 6
						if hash == 0 {
							r = '.'
							style = tcell.StyleDefault.Background(tcell.ColorNames["dimgray"]).Foreground(tcell.ColorNames["lightgrey"])
						} else {
							r = ' '
							style = tcell.StyleDefault.Background(tcell.ColorNames["dimgray"]).Foreground(tcell.ColorNames["dimgray"])
						}
					}
				} else if isSand {
					// Sand shore ripple/pebbles
					hash := (vx*17 + vy*13) % 4
					if hash == 0 {
						r = '.'
					} else {
						r = ' '
					}
				} else {
					// Grass interior runes
					hash := (vx*7 + vy*11) % 5
					if hash == 0 {
						r = ','
					} else if hash == 1 {
						r = '`'
					} else {
						r = ' '
					}
				}
			} else {
				// Sea waves
				isWave := (vx*9 + vy*13) % 23 == 0
				if isWave {
					r = '~'
				}
			}

			g.screen.SetContent(sx, sy, r, nil, style)
		}
	}

	// 1.2 Draw Billowing Smoke & Flame from Carrier depending on Health (Aesthetics Enhancement)
	if g.carrier.Health < 100.0 {
		damagePct := 100.0 - g.carrier.Health

		// Define up to 12 smoke sources distributed across the carrier deck
		sources := []struct{ dx, dy int }{
			{4, 2},   // Left side
			{18, 1},  // Right deck
			{10, 3},  // Mid deck
			{22, 4},  // Far right deck
			{7, 1},   // Near landing pad
			{14, 4},  // Mid-low deck
			{2, 3},   // Far left deck bottom
			{20, 3},  // Right mid-lower deck
			{12, 1},  // Mid top deck
			{16, 2},  // Mid-right deck
			{5, 4},   // Left lower deck
			{24, 2},  // Far right mid deck
		}

		// Determine active smoke columns based on damage level (more granular)
		numColumns := int(damagePct / 8.0)
		if numColumns < 1 && damagePct > 0 {
			numColumns = 1
		}
		if numColumns > len(sources) {
			numColumns = len(sources)
		}

		// Height of columns grows with damage
		maxHeight := 5 + int(damagePct*0.12)

		// Smoke billowing speed increases as ship is more damaged
		speedDiv := 3
		if damagePct >= 40 {
			speedDiv = 2
		}
		if damagePct >= 75 {
			speedDiv = 1
		}

		// Calculate fire base height based on damage
		fireHeight := 0
		if damagePct >= 20 {
			fireHeight = int(damagePct / 25.0)
		}

		for col := 0; col < numColumns; col++ {
			src := sources[col]
			sx := g.carrier.X + src.dx
			sy := g.carrier.Y + src.dy

			// Add a time-based sinusoidal oscillation to each column's height for organic waving
			colOscillation := int(math.Sin(float64(g.Ticks)/6.0+float64(col)) * 1.5)
			colHeight := maxHeight + colOscillation
			if colHeight < 3 {
				colHeight = 3
			}

			// Render the vertical column of smoke rising and drifting
			for h := 0; h < colHeight; h++ {
				// Organic horizontal curling/wiggle + wind drift to the right (East)
				wiggle := int(math.Sin(float64(g.Ticks)/5.0+float64(h)) * 0.6)
				smX := sx + h/2 + wiggle
				smY := sy - h

				ssmX := smX - camX
				ssmY := smY - camY

				// Ensure within map boundaries and above HUD
				if ssmX < 0 || ssmX >= g.width || ssmY < 0 || ssmY >= g.height-4 {
					continue
				}

				// Billow phase mapping
				phase := (g.Ticks/speedDiv - h) % 4
				if phase < 0 {
					phase += 4
				}

				// Density of particles increases with damage (wider phase thresholds)
				drawParticle := false
				if damagePct < 30 {
					drawParticle = (phase == 0)
				} else if damagePct < 70 {
					drawParticle = (phase == 0 || phase == 1)
				} else {
					drawParticle = (phase == 0 || phase == 1 || phase == 2)
				}

				if drawParticle {
					var r rune
					var fg tcell.Color
					bgStyle := g.getMapStyle(smX, smY)

					if h < fireHeight {
						// Flickering fire base: alternate colors/characters
						flicker := (g.Ticks + h + col) % 3
						if flicker == 0 {
							r = '▲'
							fg = tcell.ColorRed
						} else if flicker == 1 {
							r = '☼'
							fg = tcell.ColorOrange
						} else {
							r = '▲'
							fg = tcell.ColorYellow
						}
					} else if h < fireHeight+3 {
						// Extra thick hot smoke near base
						r = '█'
						fg = tcell.ColorDarkGray
					} else if h < fireHeight+7 {
						// Billowing medium smoke
						r = '▒'
						fg = tcell.ColorDarkGray
					} else {
						// Dissipated thin ash smoke
						r = '░'
						fg = tcell.ColorGray
					}

					// Blend smoke particle dynamically onto background style
					g.screen.SetContent(ssmX, ssmY, r, nil, bgStyle.Foreground(fg))
				}
			}
		}
	}

	// 1.5 Draw World Targets, Projectiles, and Particle Effects
	// A. Draw Boats (scaled 2-3x larger: 11 cells wide, 3 rows high)
	for i := 0; i < len(g.boats); i++ {
		boat := &g.boats[i]
		if !boat.Active {
			continue
		}
		bx := int(math.Round(boat.X))
		by := int(math.Round(boat.Y))

		boatColor := tcell.ColorSilver
		flagColor := tcell.ColorRed

		if boat.VX < 0 {
			// West-bound (moving left)
			// Row Y-1: Superstructure/Masts
			g.drawCell(bx-1, by-1, '_', boatColor)
			g.drawCell(bx, by-1, '╨', boatColor)
			g.drawCell(bx+1, by-1, '_', boatColor)

			// Row Y: Deck / Cabin Structure
			g.drawCell(bx-3, by, '/', boatColor)
			g.drawCell(bx-2, by, '█', boatColor)
			g.drawCell(bx-1, by, '█', flagColor) // Red flag
			g.drawCell(bx, by, '█', boatColor)
			g.drawCell(bx+1, by, '█', boatColor)
			g.drawCell(bx+2, by, '\\', boatColor)

			// Row Y+1: Main Hull & Bow/Stern
			g.drawCell(bx-5, by+1, '◄', boatColor)
			g.drawCell(bx-4, by+1, '█', boatColor)
			g.drawCell(bx-3, by+1, '█', boatColor)
			g.drawCell(bx-2, by+1, '█', boatColor)
			g.drawCell(bx-1, by+1, '█', boatColor)
			g.drawCell(bx, by+1, '█', boatColor)
			g.drawCell(bx+1, by+1, '█', boatColor)
			g.drawCell(bx+2, by+1, '█', boatColor)
			g.drawCell(bx+3, by+1, '█', boatColor)
			g.drawCell(bx+4, by+1, '█', boatColor)
			g.drawCell(bx+5, by+1, '═', boatColor)
		} else {
			// East-bound (moving right)
			// Row Y-1: Superstructure/Masts
			g.drawCell(bx-1, by-1, '_', boatColor)
			g.drawCell(bx, by-1, '╨', boatColor)
			g.drawCell(bx+1, by-1, '_', boatColor)

			// Row Y: Deck / Cabin Structure
			g.drawCell(bx-2, by, '/', boatColor)
			g.drawCell(bx-1, by, '█', boatColor)
			g.drawCell(bx, by, '█', flagColor) // Red flag
			g.drawCell(bx+1, by, '█', boatColor)
			g.drawCell(bx+2, by, '█', boatColor)
			g.drawCell(bx+3, by, '\\', boatColor)

			// Row Y+1: Main Hull & Bow/Stern
			g.drawCell(bx-5, by+1, '═', boatColor)
			g.drawCell(bx-4, by+1, '█', boatColor)
			g.drawCell(bx-3, by+1, '█', boatColor)
			g.drawCell(bx-2, by+1, '█', boatColor)
			g.drawCell(bx-1, by+1, '█', boatColor)
			g.drawCell(bx, by+1, '█', boatColor)
			g.drawCell(bx+1, by+1, '█', boatColor)
			g.drawCell(bx+2, by+1, '█', boatColor)
			g.drawCell(bx+3, by+1, '█', boatColor)
			g.drawCell(bx+4, by+1, '█', boatColor)
			g.drawCell(bx+5, by+1, '▶', boatColor)
		}
	}

	// A.2 Draw Island Military Factory Fortress
	g.drawFactories()

	// A.3 Draw Orbiting Air Defense Drones
	g.drawDrones()

	// A.4 Draw Patrolling Mobile Air Defense Tanks
	g.drawTanks()

	// A.5 Draw Static AA Gun Emplacements
	g.drawStaticAA()

	// B. Draw Bullets
	for i := 0; i < len(g.bullets); i++ {
		bullet := &g.bullets[i]
		if !bullet.Active {
			continue
		}
		bx := int(math.Round(bullet.X))
		by := int(math.Round(bullet.Y))

		sbx := bx - camX
		sby := by - camY
		if sbx >= 0 && sbx < g.width && sby >= 0 && sby < g.height-4 {
			bgStyle := g.getMapStyle(bx, by)
			color := tcell.ColorYellow
			if bullet.IsEnemy {
				color = tcell.ColorRed
			}
			g.screen.SetContent(sbx, sby, '•', nil, bgStyle.Foreground(color))
		}
	}

	// B.5 Draw Guided Missiles
	for i := 0; i < len(g.missiles); i++ {
		m := &g.missiles[i]
		if !m.Active {
			continue
		}
		mx := int(math.Round(m.X))
		my := int(math.Round(m.Y))

		smx := mx - camX
		smy := my - camY

		if smx >= 0 && smx < g.width && smy >= 0 && smy < g.height-4 {
			bgStyle := g.getMapStyle(mx, my)
			
			// Select caret/missile arrow based on the dominant direction of its velocity
			char := '¤' // Default general symbol
			if math.Abs(m.VX) > math.Abs(m.VY) {
				if m.VX > 0 {
					char = '►'
				} else {
					char = '◄'
				}
			} else {
				if m.VY > 0 {
					char = '▼'
				} else {
					char = '▲'
				}
			}

			// Render player missile in Orange and enemy missile in bold Red
			color := tcell.ColorOrange
			if m.IsEnemy {
				color = tcell.ColorRed
			}
			style := bgStyle.Foreground(color).Bold(true)
			g.screen.SetContent(smx, smy, char, nil, style)
		}
	}

	// C. Draw Explosions
	for i := 0; i < len(g.explosions); i++ {
		exp := &g.explosions[i]
		bx := exp.X
		by := exp.Y

		sbx := bx - camX
		sby := by - camY

		if sbx >= 0 && sbx < g.width && sby >= 0 && sby < g.height-4 {
			bgStyle := g.getMapStyle(bx, by)
			var r rune
			var fg tcell.Color

			if exp.Age < 4 {
				r = '*'
				fg = tcell.ColorYellow
			} else if exp.Age < 9 {
				r = '¤'
				fg = tcell.ColorOrange
			} else {
				r = '·'
				fg = tcell.ColorDarkGray
			}

			g.screen.SetContent(sbx, sby, r, nil, bgStyle.Foreground(fg))
		}
	}

	// 2. Draw Helicopter Sprite (merging smoothly onto map background style)
	h := g.heli
	hx := int(math.Round(h.X))
	hy := int(math.Round(h.Y))
	rotorChar := rotorFrames[h.RotorState]

	for r := 0; r < 3; r++ {
		for c := 0; c < 5; c++ {
			char := sprites[h.Dir][r][c]
			if char == ' ' {
				continue // Transparent sprite cell
			}

			mx := hx + c - 2
			my := hy + r - 1

			smx := mx - camX
			smy := my - camY

			// Check screen boundary limits
			if smx < 0 || smx >= g.width || smy < 0 || smy >= g.height-4 {
				continue
			}

			// Center column of the 5x3 helicopter is the spinning main rotor
			if r == 1 && c == 2 {
				char = rotorChar
			}

			// Look up original terrain background style to prevent rectangular background boxes
			bgStyle := g.getMapStyle(mx, my)

			// Pick foreground colors based on specific characters of the helicopter sprite
			var fg tcell.Color
			switch char {
			case '▲', '▼', '►', '◄':
				fg = tcell.ColorWhite // Front cabin nose (Stealth white)
			case '|', '/', '\\', '╪':
				if r == 1 && c == 2 {
					fg = tcell.ColorWhite // Rotor blades
				} else {
					fg = tcell.ColorPaleTurquoise // Tail stabilizer rotor / wings
				}
			case '-', '_', '¯', '[', ']', '=', '║':
				fg = tcell.ColorSilver // Skids, support wings, tail boom
			case '█', '▓', '▒', '╟', '╢':
				fg = tcell.ColorSlateGray // Main armored fuselage (Stealth Slate Gray)
			default:
				fg = tcell.ColorWhite
			}

			style := bgStyle.Foreground(fg)
			g.screen.SetContent(smx, smy, char, nil, style)
		}
	}

	// 3. Draw Bottom UI / HUD Dashboard
	g.drawHUD()

	if g.quitConfirming {
		g.drawQuitConfirmation()
	}

	if g.gameOver {
		g.drawGameOver()
	}

	// Double buffering display flush
	g.screen.Show()
}

// drawGameOver renders a high-impact centered game-over modal screen.
func (g *Game) drawGameOver() {
	boxW := 46
	boxH := 9
	startX := (g.width - boxW) / 2
	startY := (g.height - 4 - boxH) / 2
	if startY < 0 {
		startY = 0
	}

	borderStyle := tcell.StyleDefault.Background(tcell.ColorDarkRed).Foreground(tcell.ColorWhite)
	titleStyle := tcell.StyleDefault.Background(tcell.ColorDarkRed).Foreground(tcell.ColorYellow).Bold(true)
	textStyle := tcell.StyleDefault.Background(tcell.ColorDarkRed).Foreground(tcell.ColorWhite)

	// Fill background
	for r := 0; r < boxH; r++ {
		for c := 0; c < boxW; c++ {
			g.screen.SetContent(startX+c, startY+r, ' ', nil, borderStyle)
		}
	}

	// Draw borders
	for c := 0; c < boxW; c++ {
		g.screen.SetContent(startX+c, startY, '═', nil, borderStyle)
		g.screen.SetContent(startX+c, startY+boxH-1, '═', nil, borderStyle)
	}
	for r := 0; r < boxH; r++ {
		g.screen.SetContent(startX, startY+r, '║', nil, borderStyle)
		g.screen.SetContent(startX+boxW-1, startY+r, '║', nil, borderStyle)
	}
	g.screen.SetContent(startX, startY, '╔', nil, borderStyle)
	g.screen.SetContent(startX+boxW-1, startY, '╗', nil, borderStyle)
	g.screen.SetContent(startX, startY+boxH-1, '╚', nil, borderStyle)
	g.screen.SetContent(startX+boxW-1, startY+boxH-1, '╝', nil, borderStyle)

	// Draw content
	title := " ☠️  MISSION FAILURE  ☠️ "
	g.drawString(startX+(boxW-len(title))/2, startY+1, title, titleStyle)

	msg1 := "THE AIRCRAFT CARRIER HAS BEEN DESTROYED!"
	g.drawString(startX+(boxW-len(msg1))/2, startY+3, msg1, textStyle)

	stats := fmt.Sprintf("You survived until Wave %d", g.Wave)
	g.drawString(startX+(boxW-len(stats))/2, startY+5, stats, titleStyle)

	exitPrompt := "Press ANY KEY to exit the game"
	g.drawString(startX+(boxW-len(exitPrompt))/2, startY+7, exitPrompt, textStyle)
}

// drawQuitConfirmation renders a styled modal warning box centered on the screen.
func (g *Game) drawQuitConfirmation() {
	boxW := 42
	boxH := 7
	startX := (g.width - boxW) / 2
	startY := (g.height - 4 - boxH) / 2
	if startY < 0 {
		startY = 0
	}

	borderStyle := tcell.StyleDefault.Background(tcell.ColorDarkRed).Foreground(tcell.ColorWhite)
	titleStyle := tcell.StyleDefault.Background(tcell.ColorDarkRed).Foreground(tcell.ColorYellow).Bold(true)
	textStyle := tcell.StyleDefault.Background(tcell.ColorDarkRed).Foreground(tcell.ColorWhite)

	// Fill background
	for r := 0; r < boxH; r++ {
		for c := 0; c < boxW; c++ {
			g.screen.SetContent(startX+c, startY+r, ' ', nil, borderStyle)
		}
	}

	// Draw borders
	for c := 0; c < boxW; c++ {
		g.screen.SetContent(startX+c, startY, '═', nil, borderStyle)
		g.screen.SetContent(startX+c, startY+boxH-1, '═', nil, borderStyle)
	}
	for r := 0; r < boxH; r++ {
		g.screen.SetContent(startX, startY+r, '║', nil, borderStyle)
		g.screen.SetContent(startX+boxW-1, startY+r, '║', nil, borderStyle)
	}
	g.screen.SetContent(startX, startY, '╔', nil, borderStyle)
	g.screen.SetContent(startX+boxW-1, startY, '╗', nil, borderStyle)
	g.screen.SetContent(startX, startY+boxH-1, '╚', nil, borderStyle)
	g.screen.SetContent(startX+boxW-1, startY+boxH-1, '╝', nil, borderStyle)

	// Draw content
	title := "  CONFIRM QUIT  "
	g.drawString(startX+(boxW-len(title))/2, startY+1, title, titleStyle)

	msg := "Are you sure you want to exit?"
	g.drawString(startX+(boxW-len(msg))/2, startY+3, msg, textStyle)

	opts := "[Y]es, Quit  |  [N]o, Resume"
	g.drawString(startX+(boxW-len(opts))/2, startY+5, opts, titleStyle)
}

// drawHUD prints diagnostic status metrics and cockpit gauges
func (g *Game) drawHUD() {
	hudY := g.height - 4
	hudStyle := tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(tcell.ColorWhite)
	borderStyle := tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(tcell.ColorDarkCyan)

	// Draw top separating boundary line (Line H-4)
	for x := 0; x < g.width; x++ {
		g.screen.SetContent(x, hudY, '═', nil, borderStyle)
	}
	// Add Title Label onto boundary line
	hudTitle := fmt.Sprintf(" 🚁 COCKPIT HUD PANEL (WAVE %d) 🚁 ", g.Wave)
	g.drawString(2, hudY, hudTitle, borderStyle.Foreground(tcell.ColorYellow))

	// Scan for active enemy guided missiles to trigger flashing alert on Cockpit dashboard
	hasIncoming := false
	for i := 0; i < len(g.missiles); i++ {
		if g.missiles[i].Active && g.missiles[i].IsEnemy {
			hasIncoming = true
			break
		}
	}
	if hasIncoming {
		if (g.heli.RotorState/2)%2 == 0 {
			g.drawString(g.width-33, hudY, "⚠️ WARNING: INCOMING MISSILE ⚠️", borderStyle.Foreground(tcell.ColorRed).Bold(true))
		}
		// Tactical audio warning bell: beep every 20 frames (800ms) to avoid audio clutter
		if g.Ticks%20 == 0 {
			_ = g.screen.Beep()
		}
	}

	// Clear background of lines H-3, H-2, H-1
	for dy := 1; dy <= 3; dy++ {
		for x := 0; x < g.width; x++ {
			g.screen.SetContent(x, hudY+dy, ' ', nil, hudStyle)
		}
	}

	// Compute telemetry figures
	speedKnots := 0
	if !g.heli.Landed {
		// Calculate magnitude of velocity vector (adjusted for terminal coordinate ratio)
		vMag := math.Sqrt(g.heli.VX*g.heli.VX + (g.heli.VY*2.0)*(g.heli.VY*2.0))
		speedKnots = int(vMag * 450.0) // Scaled up to represent knots
	}

	altitudeFeet := 150
	if g.heli.Landed {
		altitudeFeet = 0
	}

	// Check Landing Pad Alignment
	padX := g.carrier.X + g.carrier.Width/3
	padY := g.carrier.Y + g.carrier.Height/2
	aligned := int(math.Round(g.heli.X)) >= padX-1 && int(math.Round(g.heli.X)) <= padX+1 &&
		int(math.Round(g.heli.Y)) >= padY-1 && int(math.Round(g.heli.Y)) <= padY+1

	alignStr := "NO"
	alignStyle := hudStyle.Foreground(tcell.ColorRed)
	if aligned {
		alignStr = "READY"
		alignStyle = hudStyle.Foreground(tcell.ColorGreen)
	}

	statusStr := "AIRBORNE"
	statusStyle := tcell.StyleDefault.Background(tcell.ColorGreen).Foreground(tcell.ColorBlack)
	if g.heli.Landed {
		if g.heli.Fuel < 100.0 {
			statusStr = "LANDED (REFUELING...)"
			statusStyle = tcell.StyleDefault.Background(tcell.ColorYellow).Foreground(tcell.ColorBlack)
		} else {
			statusStr = "LANDED (READY)"
			statusStyle = tcell.StyleDefault.Background(tcell.ColorGrey).Foreground(tcell.ColorWhite)
		}
	} else if g.heli.Fuel <= 0 {
		statusStr = "OUT OF FUEL"
		statusStyle = tcell.StyleDefault.Background(tcell.ColorRed).Foreground(tcell.ColorWhite)
	}

	fuelColor := tcell.ColorGreen
	if g.heli.Fuel < 25.0 {
		fuelColor = tcell.ColorRed
	} else if g.heli.Fuel < 50.0 {
		fuelColor = tcell.ColorOrange
	}
	fuelStyle := hudStyle.Foreground(fuelColor)

	// Display row H-3: Instruments
	instrumentText := fmt.Sprintf(
		"GPS: (%d, %d)   |   SPEED: %3d KTS   |   HEADING: %3d° (%-2s)   |   ALTITUDE: %3d FT   |   FUEL: ",
		int(math.Round(g.heli.X)), int(math.Round(g.heli.Y)),
		speedKnots, dirDegrees[g.heli.Dir], dirNames[g.heli.Dir], altitudeFeet,
	)
	g.drawString(2, hudY+1, instrumentText, hudStyle)
	
	fuelText := fmt.Sprintf("%3.1f%%", g.heli.Fuel)
	g.drawString(2+len(instrumentText), hudY+1, fuelText, fuelStyle)

	// Display Guided Missile Ammo count as premium HUD icons
	ammoLabel := "   |   MISSILES: "
	g.drawString(2+len(instrumentText)+len(fuelText), hudY+1, ammoLabel, hudStyle)

	ammoColor := tcell.ColorGreen
	if g.heli.MissileAmmo == 0 {
		ammoColor = tcell.ColorRed
	} else if g.heli.MissileAmmo <= 2 {
		ammoColor = tcell.ColorOrange
	}
	ammoStyle := hudStyle.Foreground(ammoColor).Bold(true)

	ammoStr := ""
	for i := 0; i < 4; i++ {
		if i < g.heli.MissileAmmo {
			ammoStr += "▲ "
		} else {
			ammoStr += "· "
		}
	}
	g.drawString(2+len(instrumentText)+len(fuelText)+len(ammoLabel), hudY+1, ammoStr, ammoStyle)

	// Display row H-2: Status Metrics
	statusLabel := "FLIGHT STATUS: "
	g.drawString(2, hudY+2, statusLabel, hudStyle)
	g.drawString(2+len(statusLabel), hudY+2, " "+statusStr+" ", statusStyle)

	offset := 2 + len(statusLabel) + len(statusStr) + 2

	padLabel := "   |   ALIGN: "
	g.drawString(offset, hudY+2, padLabel, hudStyle)
	g.drawString(offset+len(padLabel), hudY+2, alignStr, alignStyle)

	offset += len(padLabel) + len(alignStr)

	scoreLabel := "   |   BOATS SUNK: "
	g.drawString(offset, hudY+2, scoreLabel, hudStyle)
	scoreValStr := fmt.Sprintf("%d", g.boatsSunk)
	g.drawString(offset+len(scoreLabel), hudY+2, scoreValStr, hudStyle.Foreground(tcell.ColorYellow))

	offset += len(scoreLabel) + len(scoreValStr)

	armorColor := tcell.ColorGreen
	if g.heli.Armor < 25.0 {
		armorColor = tcell.ColorRed
	} else if g.heli.Armor < 50.0 {
		armorColor = tcell.ColorOrange
	}
	armorStyle := hudStyle.Foreground(armorColor)

	armorLabel := "   |   ARMOR: "
	g.drawString(offset, hudY+2, armorLabel, hudStyle)
	armorText := fmt.Sprintf("%3.0f%%", g.heli.Armor)
	g.drawString(offset+len(armorLabel), hudY+2, armorText, armorStyle)

	offset += len(armorLabel) + len(armorText)

	lockLabel := "   |   LOCK: "
	g.drawString(offset, hudY+2, lockLabel, hudStyle)

	lockedBoat, lockedFactory, lockedTank, lockedStaticAA := g.lockedBoat, g.lockedFactory, g.lockedTank, g.lockedStaticAA
	lockStr := "NONE"
	lockColor := tcell.ColorRed
	if lockedBoat != nil {
		lockStr = "BOAT"
		lockColor = tcell.ColorGreen
	} else if lockedFactory != nil {
		fIdx := -1
		for idx := range g.factories {
			if &g.factories[idx] == lockedFactory {
				fIdx = idx
				break
			}
		}
		activeDrones := 0
		if fIdx != -1 {
			for d := 0; d < len(g.drones); d++ {
				if g.drones[d].Active && g.drones[d].FactoryIdx == fIdx {
					activeDrones++
				}
			}
		}
		totalDrones := activeDrones + lockedFactory.DronesRemaining
		if totalDrones > 0 {
			lockStr = fmt.Sprintf("FACTORY (DRONES: %d/10)", totalDrones)
		} else {
			lockStr = "FACTORY (SHIELDS DOWN!)"
		}
		lockColor = tcell.ColorGreen
	} else if lockedTank != nil {
		lockStr = "TANK"
		lockColor = tcell.ColorGreen
	} else if lockedStaticAA != nil {
		lockStr = "STATIC AA"
		lockColor = tcell.ColorGreen
	}
	lockStyle := hudStyle.Foreground(lockColor).Bold(true)
	g.drawString(offset+len(lockLabel), hudY+2, lockStr, lockStyle)

	offset += len(lockLabel) + len(lockStr)

	// Display Carrier HP health bar
	carrierColor := tcell.ColorGreen
	if g.carrier.Health < 25.0 {
		carrierColor = tcell.ColorRed
	} else if g.carrier.Health < 50.0 {
		carrierColor = tcell.ColorOrange
	}
	carrierStyle := hudStyle.Foreground(carrierColor)

	carrierLabel := "   |   CARRIER: "
	g.drawString(offset, hudY+2, carrierLabel, hudStyle)
	
	barStr := "["
	pct := int(math.Round(g.carrier.Health))
	filled := pct / 10
	for b := 0; b < 10; b++ {
		if b < filled {
			barStr += "█"
		} else {
			barStr += "░"
		}
	}
	barStr += "]"
	g.drawString(offset+len(carrierLabel), hudY+2, barStr, carrierStyle.Bold(true))
	carrierText := fmt.Sprintf(" %3d%%", pct)
	g.drawString(offset+len(carrierLabel)+len(barStr), hudY+2, carrierText, carrierStyle)


	// Display row H-1: Control Instructions
	controlStyle := hudStyle.Foreground(tcell.ColorSilver)
	g.drawString(2, hudY+3, "CONTROLS: ARROWS/WASD = Fly | DOWN/S = Brakes | SPACE = Cannon | F = Guided Missile | L = Land/Takeoff", controlStyle)
}

// drawString is a helper to draw string labels cell by cell
func (g *Game) drawString(x, y int, str string, style tcell.Style) {
	col := 0
	for _, r := range str {
		g.screen.SetContent(x+col, y, r, nil, style)
		col++
	}
}

// drawCell draws a single cell with safety boundary checking and dynamic background styling
func (g *Game) drawCell(x, y int, r rune, fg tcell.Color) {
	camX, camY := g.getCameraOffset()
	sx := x - camX
	sy := y - camY
	if sx >= 0 && sx < g.width && sy >= 0 && sy < g.height-4 {
		bgStyle := g.getMapStyle(x, y)
		g.screen.SetContent(sx, sy, r, nil, bgStyle.Foreground(fg))
	}
}

// drawFactories renders active 7x3 industrial building sprites (smokestacks, glowing beacons, load gates, and rising smoke columns)
func (g *Game) drawFactories() {
	for fIdx := range g.factories {
		fact := &g.factories[fIdx]
		if !fact.Active {
			continue
		}

		fx := int(math.Round(fact.X))
		fy := int(math.Round(fact.Y))

		// If the factory is destroying (burning/exploding)
		isDestroying := fact.SinkingTimer > 0

		// 17x5 factory sprite
		var factorySprite = [5][17]rune{
			{' ', '░', '█', '░', ' ', ' ', ' ', ' ', '☼', ' ', ' ', ' ', ' ', '░', '█', '░', ' '},
			{' ', '║', '█', '║', ' ', ' ', '┌', '─', '┴', '─', '┐', ' ', ' ', '║', '█', '║', ' '},
			{'╓', '─', '╨', '─', '┴', '─', '┘', ' ', ' ', ' ', '└', '─', '┴', '─', '╨', '─', '╖'},
			{'║', ' ', '█', ' ', ' ', '█', ' ', ' ', '█', ' ', ' ', '█', ' ', ' ', '█', ' ', '║'},
			{'╙', '─', '─', '─', '─', '─', '[', '▓', '▓', '▓', ']', '─', '─', '─', '─', '─', '╜'},
		}

		for r := 0; r < 5; r++ {
			for c := 0; c < 17; c++ {
				mx := fx + c - 8
				my := fy + r - 2

				char := factorySprite[r][c]
				if char == ' ' {
					continue
				}

				var fg tcell.Color
				if isDestroying {
					// Flickering fire base for dying factory
					flicker := (g.Ticks + r + c) % 3
					if flicker == 0 {
						char = '▲'
						fg = tcell.ColorRed
					} else if flicker == 1 {
						char = '☼'
						fg = tcell.ColorOrange
					} else {
						char = '█'
						fg = tcell.ColorDarkGray
					}
				} else {
					// Standard factory coloring
					switch char {
					case '║', '┌', '─', '┐', '└', '┘', '┴':
						fg = tcell.ColorSilver
					case '☼':
						// Flashing beacon (out-of-phase warning system based on factory index)
						phaseOffset := fIdx * 4
						if ((g.Ticks + phaseOffset) / 8)%2 == 0 {
							fg = tcell.ColorRed
						} else {
							fg = tcell.ColorYellow
						}
					case '▓':
						fg = tcell.ColorDarkCyan // Shutter door / load gates
					case '╓', '╖', '╙', '╜':
						fg = tcell.ColorSteelBlue
					case '░':
						fg = tcell.ColorDarkGray // Smokestack exhaust collars
					default:
						fg = tcell.ColorGray // Factory concrete structure
					}
				}

				g.drawCell(mx, my, char, fg)
			}
		}

		// Draw active smoke stack emissions if not destroyed/sinking
		if !isDestroying {
			g.drawFactorySmoke(fx-6, fy-2)
			g.drawFactorySmoke(fx+6, fy-2)
		}
	}
}

// drawFactorySmoke renders procedurally billowing, rising grey smoke columns
func (g *Game) drawFactorySmoke(sx, sy int) {
	colHeight := 5
	for h := 1; h <= colHeight; h++ {
		// drift to the right + wiggle
		wiggle := int(math.Sin(float64(g.Ticks)/5.0+float64(h)) * 0.8)
		smX := sx + h/2 + wiggle
		smY := sy - h

		if smX < 0 || smX >= g.worldWidth || smY < 0 || smY >= g.worldHeight {
			continue
		}

		// phase-based density
		phase := (g.Ticks/3 - h) % 3
		if phase < 0 {
			phase += 3
		}

		if phase == 0 || phase == 1 {
			var r rune
			var fg tcell.Color
			if h < 3 {
				r = '█'
				fg = tcell.ColorDarkGray
			} else if h < 5 {
				r = '▒'
				fg = tcell.ColorDarkGray
			} else {
				r = '░'
				fg = tcell.ColorGray
			}

			g.drawCell(smX, smY, r, fg)
		}
	}
}

// drawDrones renders the orbiting air-defense drone icons in highly visible LightCyan
func (g *Game) drawDrones() {
	for i := 0; i < len(g.drones); i++ {
		drone := &g.drones[i]
		if !drone.Active {
			continue
		}

		dx := int(math.Round(drone.X))
		dy := int(math.Round(drone.Y))

		// Draw drone symbol
		g.drawCell(dx, dy, '⌖', tcell.ColorLightCyan)
	}
}

// drawTanks renders beautiful, premium retro tank sprites pointing in patrol direction
func (g *Game) drawTanks() {
	for i := 0; i < len(g.tanks); i++ {
		tank := &g.tanks[i]
		if !tank.Active {
			continue
		}

		tx := int(math.Round(tank.X))
		ty := int(math.Round(tank.Y))

		isBurning := tank.SinkingTimer > 0

		color := tcell.ColorBlack // Sleek tactical black tank armor
		treadColor := tcell.ColorDarkGray // Dark gray heavy treads
		gunColor := tcell.ColorSilver // Sleek silver dual gun barrels
		fireColor := tcell.ColorOrange

		if isBurning {
			color = tcell.ColorDarkRed
			treadColor = tcell.ColorDarkGray
			gunColor = tcell.ColorDarkGray
		}

		if tank.PatrolDir == 0 {
			// Vertical Tank (5x3 sprite)
			if tank.VY < 0 {
				// Moving North: Dual high-velocity AA guns pointing North
				g.drawCell(tx-1, ty-1, '║', gunColor)
				g.drawCell(tx+1, ty-1, '║', gunColor)

				// Side treads + central rounded turret
				g.drawCell(tx-2, ty, '▒', treadColor)
				g.drawCell(tx-1, ty, '(', color)
				g.drawCell(tx, ty, '▓', color)
				g.drawCell(tx+1, ty, ')', color)
				g.drawCell(tx+2, ty, '▒', treadColor)

				// Rear treads + sloped armor housing
				g.drawCell(tx-2, ty+1, '▒', treadColor)
				g.drawCell(tx-1, ty+1, ' ', treadColor)
				g.drawCell(tx, ty+1, '▄', color)
				g.drawCell(tx+1, ty+1, ' ', treadColor)
				g.drawCell(tx+2, ty+1, '▒', treadColor)
			} else {
				// Moving South: Front sloped armor housing + treads
				g.drawCell(tx-2, ty-1, '▒', treadColor)
				g.drawCell(tx-1, ty-1, ' ', treadColor)
				g.drawCell(tx, ty-1, '▀', color)
				g.drawCell(tx+1, ty-1, ' ', treadColor)
				g.drawCell(tx+2, ty-1, '▒', treadColor)

				// Side treads + central rounded turret
				g.drawCell(tx-2, ty, '▒', treadColor)
				g.drawCell(tx-1, ty, '(', color)
				g.drawCell(tx, ty, '▓', color)
				g.drawCell(tx+1, ty, ')', color)
				g.drawCell(tx+2, ty, '▒', treadColor)

				// Dual AA guns pointing South
				g.drawCell(tx-1, ty+1, '║', gunColor)
				g.drawCell(tx+1, ty+1, '║', gunColor)
			}
			
			if isBurning {
				flicker := (g.Ticks / 3) % 2
				if flicker == 0 {
					g.drawCell(tx, ty, '▲', tcell.ColorRed)
				} else {
					g.drawCell(tx, ty, '☼', fireColor)
				}
			}
		} else {
			// Horizontal Tank (5x3 sprite)
			if tank.VX < 0 {
				// Moving West (Pointing Left)
				// Upper tracks
				g.drawCell(tx-2, ty-1, '▄', treadColor)
				g.drawCell(tx-1, ty-1, '▒', treadColor)
				g.drawCell(tx, ty-1, '▒', treadColor)
				g.drawCell(tx+1, ty-1, '▒', treadColor)
				g.drawCell(tx+2, ty-1, '▄', treadColor)

				// Dual AA gun barrels pointing West, chassis, rounded turret
				g.drawCell(tx-2, ty, '═', gunColor)
				g.drawCell(tx-1, ty, '═', gunColor)
				g.drawCell(tx, ty, '▓', color)
				g.drawCell(tx+1, ty, '▒', color)
				g.drawCell(tx+2, ty, ']', color)

				// Lower tracks
				g.drawCell(tx-2, ty+1, '▀', treadColor)
				g.drawCell(tx-1, ty+1, '▒', treadColor)
				g.drawCell(tx, ty+1, '▒', treadColor)
				g.drawCell(tx+1, ty+1, '▒', treadColor)
				g.drawCell(tx+2, ty+1, '▀', treadColor)
			} else {
				// Moving East (Pointing Right)
				// Upper tracks
				g.drawCell(tx-2, ty-1, '▄', treadColor)
				g.drawCell(tx-1, ty-1, '▒', treadColor)
				g.drawCell(tx, ty-1, '▒', treadColor)
				g.drawCell(tx+1, ty-1, '▒', treadColor)
				g.drawCell(tx+2, ty-1, '▄', treadColor)

				// Chassis, rounded turret, dual barrels pointing East
				g.drawCell(tx-2, ty, '[', color)
				g.drawCell(tx-1, ty, '▒', color)
				g.drawCell(tx, ty, '▓', color)
				g.drawCell(tx+1, ty, '═', gunColor)
				g.drawCell(tx+2, ty, '═', gunColor)

				// Lower tracks
				g.drawCell(tx-2, ty+1, '▀', treadColor)
				g.drawCell(tx-1, ty+1, '▒', treadColor)
				g.drawCell(tx, ty+1, '▒', treadColor)
				g.drawCell(tx+1, ty+1, '▒', treadColor)
				g.drawCell(tx+2, ty+1, '▀', treadColor)
			}
			
			if isBurning {
				flicker := (g.Ticks / 3) % 2
				if flicker == 0 {
					g.drawCell(tx, ty, '▲', tcell.ColorRed)
				} else {
					g.drawCell(tx, ty, '☼', fireColor)
				}
			}
		}
	}
}

// drawStaticAA renders static AA gun emplacements along the coast
func (g *Game) drawStaticAA() {
	for i := 0; i < len(g.staticAAs); i++ {
		aa := &g.staticAAs[i]
		if !aa.Active {
			continue
		}
		ax := int(math.Round(aa.X))
		ay := int(math.Round(aa.Y))
		isBurning := aa.SinkingTimer > 0

		gunColor := tcell.ColorSilver
		baseColor := tcell.ColorDarkCyan
		shieldColor := tcell.ColorDarkGray
		centerColor := tcell.ColorRed
		fireColor := tcell.ColorOrange

		if isBurning {
			gunColor = tcell.ColorDarkGray
			baseColor = tcell.ColorDarkRed
			shieldColor = tcell.ColorDarkGray
			centerColor = tcell.ColorOrange
		}

		// Draw dual barrels pointing North-ish
		g.drawCell(ax-1, ay-1, '║', gunColor)
		g.drawCell(ax+1, ay-1, '║', gunColor)

		// Draw base
		g.drawCell(ax-1, ay, '▕', shieldColor)
		g.drawCell(ax, ay, '╬', baseColor)
		g.drawCell(ax+1, ay, '▏', shieldColor)

		// Draw glowing radar/light on top of base
		if !isBurning && (g.Ticks/10)%2 == 0 {
			g.drawCell(ax, ay, '☼', centerColor)
		}

		// Draw foundation support
		g.drawCell(ax-1, ay+1, '▀', shieldColor)
		g.drawCell(ax, ay+1, '█', shieldColor)
		g.drawCell(ax+1, ay+1, '▀', shieldColor)

		// Draw fire effect if burning
		if isBurning {
			flicker := (g.Ticks / 3) % 2
			if flicker == 0 {
				g.drawCell(ax, ay, '▲', tcell.ColorRed)
			} else {
				g.drawCell(ax, ay, '☼', fireColor)
			}
		}
	}
}

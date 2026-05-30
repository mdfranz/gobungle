package main

import (
	"fmt"
	"math"

	"github.com/gdamore/tcell/v2"
)

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

	// Ocean styling with pseudo-random wave patterns (static coordinates map to waves)
	isWave := (x*9+y*13)%23 == 0
	if isWave {
		return tcell.StyleDefault.Background(navyBlue).Foreground(lightBlue)
	}

	return tcell.StyleDefault.Background(navyBlue).Foreground(navyBlue)
}

// draw handles screen rendering
func (g *Game) draw() {
	// 1. Draw Ocean Background & Aircraft Carrier
	for y := 0; y < g.height-4; y++ {
		for x := 0; x < g.width; x++ {
			style := g.getMapStyle(x, y)
			r := ' '

			// Add carrier markings on deck cells
			if x >= g.carrier.X && x < g.carrier.X+g.carrier.Width &&
				y >= g.carrier.Y && y < g.carrier.Y+g.carrier.Height {

				cy := y - g.carrier.Y
				cx := x - g.carrier.X

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
			} else {
				// Sea waves
				isWave := (x*9+y*13)%23 == 0
				if isWave {
					r = '~'
				}
			}

			g.screen.SetContent(x, y, r, nil, style)
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

	// B. Draw Bullets
	for i := 0; i < len(g.bullets); i++ {
		bullet := &g.bullets[i]
		if !bullet.Active {
			continue
		}
		bx := int(math.Round(bullet.X))
		by := int(math.Round(bullet.Y))

		if bx >= 0 && bx < g.width && by >= 0 && by < g.height-4 {
			bgStyle := g.getMapStyle(bx, by)
			color := tcell.ColorYellow
			if bullet.IsEnemy {
				color = tcell.ColorRed
			}
			g.screen.SetContent(bx, by, '•', nil, bgStyle.Foreground(color))
		}
	}

	// C. Draw Explosions
	for i := 0; i < len(g.explosions); i++ {
		exp := &g.explosions[i]
		bx := exp.X
		by := exp.Y

		if bx >= 0 && bx < g.width && by >= 0 && by < g.height-4 {
			bgStyle := g.getMapStyle(bx, by)
			var r rune
			var fg tcell.Color

			if exp.Age < 3 {
				r = '*'
				fg = tcell.ColorYellow
			} else if exp.Age < 6 {
				r = '¤'
				fg = tcell.ColorOrange
			} else {
				r = '·'
				fg = tcell.ColorDarkGray
			}

			g.screen.SetContent(bx, by, r, nil, bgStyle.Foreground(fg))
		}
	}

	// 2. Draw Helicopter Sprite (merging smoothly onto map background style)
	h := g.heli
	hx := int(math.Round(h.X))
	hy := int(math.Round(h.Y))
	rotorChar := rotorFrames[h.RotorState]

	for r := 0; r < 3; r++ {
		for c := 0; c < 3; c++ {
			char := sprites[h.Dir][r][c]
			if char == ' ' {
				continue // Transparent sprite cell
			}

			mx := hx + c - 1
			my := hy + r - 1

			// Check screen boundary limits
			if mx < 0 || mx >= g.width || my < 0 || my >= g.height-4 {
				continue
			}

			// Center cell of helicopter is the spinning main rotor
			if r == 1 && c == 1 {
				char = rotorChar
			}

			// Look up original terrain background style to prevent rectangular background boxes
			bgStyle := g.getMapStyle(mx, my)

			// Pick foreground colors based on specific characters of the helicopter sprite
			var fg tcell.Color
			switch char {
			case '▲', '▼', '►', '◄':
				fg = tcell.ColorYellow // Front cabin nose
			case '|', '/', '\\':
				if r == 1 && c == 1 {
					fg = tcell.ColorWhite // Rotor blades
				} else {
					fg = tcell.ColorPaleTurquoise // Tail rotor / boom
				}
			case '-', '_', '¯', '[', ']', '=':
				fg = tcell.ColorSilver // Skids & support wings
			default:
				fg = tcell.ColorWhite
			}

			style := bgStyle.Foreground(fg)
			g.screen.SetContent(mx, my, char, nil, style)
		}
	}

	// 3. Draw Bottom UI / HUD Dashboard
	g.drawHUD()

	// Double buffering display flush
	g.screen.Show()
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
	g.drawString(2, hudY, " 🚁 COCKPIT HUD PANEL 🚁 ", borderStyle.Foreground(tcell.ColorYellow))

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
		"SPEED: %3d KTS   |   HEADING: %3d° (%-2s)   |   ALTITUDE: %3d FT   |   FUEL: ",
		speedKnots, dirDegrees[g.heli.Dir], dirNames[g.heli.Dir], altitudeFeet,
	)
	g.drawString(2, hudY+1, instrumentText, hudStyle)
	g.drawString(2+len(instrumentText), hudY+1, fmt.Sprintf("%3.1f%%", g.heli.Fuel), fuelStyle)

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
	g.drawString(offset+len(armorLabel), hudY+2, fmt.Sprintf("%3.0f%%", g.heli.Armor), armorStyle)

	// Display row H-1: Control Instructions
	controlStyle := hudStyle.Foreground(tcell.ColorSilver)
	g.drawString(2, hudY+3, "CONTROLS: ARROWS/WASD = Fly | DOWN/S = Brakes | SPACE = Fire Cannon | L = Land/Takeoff", controlStyle)
}

// drawString is a helper to draw string labels cell by cell
func (g *Game) drawString(x, y int, str string, style tcell.Style) {
	for i, r := range str {
		g.screen.SetContent(x+i, y, r, nil, style)
	}
}

// drawCell draws a single cell with safety boundary checking and dynamic background styling
func (g *Game) drawCell(x, y int, r rune, fg tcell.Color) {
	if x >= 0 && x < g.width && y >= 0 && y < g.height-4 {
		bgStyle := g.getMapStyle(x, y)
		g.screen.SetContent(x, y, r, nil, bgStyle.Foreground(fg))
	}
}

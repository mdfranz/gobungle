![Gobungle Carrier Damage State](burning.png)

# 🚁 Gobungle: Tactical Helicopter Combat & Carrier Defense

Gobungle is a high-octane, terminal-based tactical combat game written in Go using the `tcell` library. Command a state-of-the-art attack helicopter, defend your mothership aircraft carrier against rogue warships, and execute surgical lock-on guided missile strikes!

Inspired by Will Wright's legendary 1984 8-bit classic [Raid on Bungeling Bay](https://en.wikipedia.org/wiki/Raid_on_Bungeling_Bay), Gobungle adapts the iconic helicopter carrier defense formula into a fast-paced command line experience.

[![Raid on Bungeling Bay C64 Gameplay](https://img.youtube.com/vi/teWJkLaut9s/hqdefault.jpg)](https://www.youtube.com/watch?v=teWJkLaut9s)


---

## 🎮 Gameplay & Mechanics

Your mission is to seek out and destroy three heavily armed rogue gunboats patrolling the ocean while protecting your home aircraft carrier.

### ⚓ The Aircraft Carrier (Mothership)
* **Your Safe Haven**: The carrier is marked by a yellow deck with an **`H`** landing pad. 
* **Replenishment**: Landing on the carrier pad slowly **refuels** your helicopter, **repairs** your armor, **re-arms** your guided missiles (up to 4 capacity), and **repairs the carrier's own health**.
* **Defend at All Costs**: Active enemy gunboats periodically launch powerful guided missiles targeting the center of your carrier deck. If the carrier's health drops to 0%, the round is lost and reset.

### 💨 Dynamic Billowing Smoke & Inferno State
The visual state of your carrier dynamically reflects its health (0% - 100%):
* **Granular Fire Outbreaks**: Up to **12 unique deck sources** ignite one by one as the ship takes damage.
* **Plume Height & Density**: Plumes rise organically up to **17 cells high**. As health deteriorates, smoke thickens from sparse grey ash (`░`) to dense black pillars (`█`).
* **Convection Rates**: Under minor damage, smoke lazily drifts upwards. When the ship is critically damaged, the convection rate doubles, sending smoke billowing furiously.
* **Flickering Fire Base**: Active deck fires flicker at the base of each smoke column, cycling rapidly through Red, Orange, and Yellow flames (`▲`, `☼`).
* **Horizontal Wind Curling**: Plumes swirl and wiggle horizontally (`math.Sin`) as they drift Eastward under oceanic wind conditions.

### ⚔️ Combat & Interception
* **Aerial Cannons**: Your high-velocity cannon bullets fly up to a range of 35 cells. Use them to shred gunboats or **manually intercept and shoot down incoming enemy guided missiles** in mid-air to protect your carrier!
* **Guided Missiles**: Fire high-impact guided missiles at locked gunboats. Targets must be within a ±45-degree forward field-of-view aperture. Fired missiles start at speed 0.5 and accelerate up to speed 5.0, tracking their targets continuously.
* **Enemy Anti-Air (AA)**: Gunboats defend themselves with rapid-fire standard AA flak (range 55) and launch guided missiles directly targeting your carrier's flight deck.

---

## ⌨️ Controls & Keybindings

| Key / Action | Control (Keyboard) | Alternative (WASD) |
| :--- | :--- | :--- |
| **Move / Accelerate Forward** | `Up Arrow` | `W` / `w` |
| **Air Brakes (Dampen Speed)** | `Down Arrow` | `S` / `s` |
| **Rotate Counterclockwise** | `Left Arrow` | `A` / `a` |
| **Rotate Clockwise** | `Right Arrow` | `D` / `d` |
| **Take Off (from Carrier Pad)**| `Space` / `Up Arrow` / `L` | `W` / `w` |
| **Land (over Carrier Pad)** | `L` (when aligned & hovering slowly) | `l` |
| **Fire Aerial Cannon** | `Spacebar` | `Spacebar` |
| **Fire Guided Missile** | `F` / `f` / `M` / `m` | `F` / `f` / `M` / `m` |
| **Graceful Quit Game** | `Escape` or `Ctrl+C` | `Escape` or `Ctrl+C` |

---

## 🛠️ Installation & Building

### Prerequisites
* [Go](https://go.dev/doc/install) 1.20 or newer installed.
* A terminal supporting 256 colors or true color (e.g., standard Linux/macOS terminal).

### Compiling and Running
1. Clone or navigate to the repository directory:
   ```bash
   cd gobungle
   ```
2. Build the executable using the provided `Makefile`:
   ```bash
   make build
   ```
3. Run the game:
   ```bash
   ./gobungle
   ```

---

## 🖥️ Cockpit HUD Display

Your helicopter features an advanced real-time heads-up display split at the bottom of the screen:
```text
CARRIER: [████████░░] 75%  |  ARMOR: [██████████] 100%  |  FUEL: [██████████] 100%
COORDINATES: (45, 12) | DIR: E (90°) | CANNON: READY | MISSILES: 4/4 [LOCK: BOAT-2]
⚠️ WARNING: INCOMING MISSILE ⚠️
```
* **Blinking HUD Warnings**: The dashboard flashes a bright red `⚠️ WARNING: INCOMING MISSILE ⚠️` alert whenever an active enemy missile is flying toward your carrier deck, giving you time to race back and intercept it!

# Modular Interface Refactor Plan

This document outlines the strategy for migrating the Gobungle game engine from its current tightly-coupled "singleton hub" architecture to a **Modular Interface-Driven Composition**. 

The primary goal of this refactor is to decouple the core game logic (physics, state management) from the rendering and input systems (`tcell`). This will allow the engine to support multiple frontends, such as a Web Browser UI (via WebAssembly or WebSockets), without modifying the underlying game rules.

## Phase 1: Define Core Interfaces

We need to create boundaries between the game logic and the I/O systems. We will define these interfaces in a new file (e.g., `interfaces.go`) or within `types.go`.

### The `Renderer` Interface
Abstracts away the terminal screen. Instead of the game knowing about `tcell.Screen`, it only knows it can draw things to a generic canvas.

```go
type Color int // Generic color type to replace tcell.Color

type Style struct {
    Fg Color
    Bg Color
}

type Renderer interface {
    Init() error
    Clear()
    Size() (width int, height int)
    DrawChar(x, y int, ch rune, style Style)
    DrawString(x, y int, text string, style Style)
    Show()
    Close()
}
```

### The `InputHandler` Interface
Abstracts away keyboard polling.

```go
type InputEvent interface{} // Can be a KeyPressEvent, ResizeEvent, or QuitEvent

type InputHandler interface {
    PollEvent() InputEvent
}
```

## Phase 2: Refactor the `Game` Struct

Currently, `Game` directly holds `screen tcell.Screen`. This needs to be replaced with our new interfaces.

```go
// types.go
type Game struct {
    renderer Renderer
    input    InputHandler
    
    // Engine State
    width      int
    height     int
    quit       chan struct{}
    
    // Game Entities
    heli       Helicopter
    carrier    Carrier
    // ... other entities
}
```

## Phase 3: Implement Concrete `tcell` Wrappers

We will move the existing `tcell` logic into concrete structs that implement our new interfaces. This ensures the current terminal version of the game still works perfectly.

```go
// tcell_backend.go
type TcellRenderer struct {
    screen tcell.Screen
}

func (t *TcellRenderer) Init() error { ... }
func (t *TcellRenderer) DrawChar(x, y int, ch rune, style Style) { ... }
// ... implements Renderer

type TcellInputHandler struct {
    screen tcell.Screen
}

func (i *TcellInputHandler) PollEvent() InputEvent { ... }
// ... implements InputHandler
```

## Phase 4: Update the Game Loop and Draw Logic

Update `draw.go` and `main.go` to use the `renderer` and `input` interfaces instead of calling `tcell` directly. 

```go
// main.go
func main() {
    // 1. Initialize the specific backend
    tcellScreen, _ := tcell.NewScreen()
    renderer := &TcellRenderer{screen: tcellScreen}
    input := &TcellInputHandler{screen: tcellScreen}
    
    // 2. Inject into the engine
    game := &Game{
        renderer: renderer,
        input:    input,
        // ...
    }
    
    go game.gameLoop()
    game.inputLoop()
}
```

## Phase 5: Future Web UI Implementation

Once the system is modular, adding a Web UI becomes a straightforward additive task. We simply write a new backend.

**Option A: WebSockets (Client/Server)**
- Create a `WebSocketRenderer` that serializes `DrawChar` commands into JSON arrays.
- Create a `WebSocketInputHandler` that listens for keydown events over the socket.
- Write a simple HTML/JS frontend that consumes the JSON and draws to an `<canvas>`.

**Option B: WebAssembly (Wasm)**
- Compile the Go code to Wasm.
- Create a `CanvasRenderer` that uses the `syscall/js` package to directly manipulate the DOM and HTML5 Canvas API.
- Create a `DOMInputHandler` that attaches event listeners to the browser window.

```go
// main_js.go (WebAssembly Entrypoint)
func main() {
    renderer := &WasmCanvasRenderer{}
    input := &WasmInputHandler{}
    
    game := &Game{
        renderer: renderer,
        input:    input,
        // ...
    }
    // ...
}
```

// Package app is the deck's root Bubble Tea model: screen routing
// (Deck ⇄ Palette), focus management, and intent recording. Like palette,
// app records the user's choice and cmd/gearshifter injects — app never
// imports tmux (ARCHITECTURE.md §2). What the deck contains is layout's
// business: app consumes placements and owns only how they behave.
package app

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/kylesnowschwartz/gearshifter/internal/catalog"
	"github.com/kylesnowschwartz/gearshifter/internal/deck"
	"github.com/kylesnowschwartz/gearshifter/internal/layout"
	"github.com/kylesnowschwartz/gearshifter/internal/palette"
	"github.com/kylesnowschwartz/gearshifter/internal/theme"
	"github.com/kylesnowschwartz/gearshifter/internal/widget"
)

type screen int

const (
	screenDeck screen = iota
	screenPalette
)

// pressFrameDuration is the armed flash between firing a tile and the
// popup closing — long enough to read as a press, short enough to never
// feel like lag (P2; Textual's press animation is the prior art,
// TUI-AESTHETICS.md §6).
const pressFrameDuration = 150 * time.Millisecond

// pressFrameDoneMsg ends the armed frame; the app quits and the caller
// injects the recorded selection.
type pressFrameDoneMsg struct{}

// Model routes between the deck screen and the embedded palette.
type Model struct {
	commands []catalog.Command
	order    []layout.Placement
	styles   *theme.Styles
	focus    int

	screen  screen
	palette palette.Model
	armed   bool // inside the press frame: input is inert, the tick quits
	width   int
	height  int

	selected   *catalog.Command
	arg        string
	insertOnly bool
}

// New builds the app over a placed deck (layout.Default or, from P4, a
// parsed layout.toml). Focus order = placement order; st must be the
// same registry the placements' tiles were built with.
func New(commands []catalog.Command, placements []layout.Placement, st *theme.Styles) Model {
	return Model{commands: commands, order: placements, styles: st}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		m.width, m.height = size.Width, size.Height
	}
	if m.screen == screenPalette {
		return m.routePalette(msg)
	}
	return m.updateDeck(msg)
}

func (m Model) View() tea.View {
	if m.screen == screenPalette {
		return m.palette.View()
	}
	var view tea.View
	view.AltScreen = true
	view.MouseMode = tea.MouseModeCellMotion // requested from P0 so tmux never scrolls
	// Colored themes own the popup surface (nil = terminal default);
	// without this, FgBase text vanishes on light terminals.
	view.BackgroundColor = m.styles.Background
	view.ForegroundColor = m.styles.Foreground
	if m.width == 0 {
		return view
	}
	view.Content = m.viewDeck()
	return view
}

// updateDeck dispatches deck input by kind; each handler owns one input
// device. Keyboard policy lives here in app — it owns focus. Quit keys
// always work; everything else is gated once, here, so a degraded canvas
// (or an empty deck) can never focus, fire, or hit an invisible tile.
func (m Model) updateDeck(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(pressFrameDoneMsg); ok {
		return m, tea.Quit
	}
	if m.armed {
		return m, nil // the press frame owns the screen; its tick quits
	}
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case "ctrl+c", "esc", "q":
			return m, tea.Quit
		}
	}
	if len(m.order) == 0 || m.canvasTooSmall() {
		return m, nil
	}
	switch msg := msg.(type) {
	case tea.MouseClickMsg:
		return m.handleDeckClick(msg)
	case tea.MouseWheelMsg:
		return m.handleDeckWheel(msg)
	case tea.KeyPressMsg:
		return m.handleDeckKey(msg)
	}
	return m, nil
}

func (m Model) handleDeckClick(msg tea.MouseClickMsg) (tea.Model, tea.Cmd) {
	if msg.Button != tea.MouseLeft {
		return m, nil
	}
	// Named-layer hit-testing (M3 P1 decision): the compositor knows
	// every tile's bounds; a hit both focuses and fires — click = do.
	hit := m.compositor().Hit(msg.X, msg.Y)
	i, ok := hitTile(hit)
	if !ok || i >= len(m.order) {
		return m, nil
	}
	m.focus = i
	if g, isGear := m.order[i].Tile.(widget.Gear); isGear {
		// Gated column: each value row is its own target — one click
		// to any state. Border/label clicks just focus.
		if v, ok := g.ValueAt(msg.Y - hit.Bounds().Min.Y); ok {
			g = g.WithCursor(v)
			m.order[i].Tile = g
			return m.activate(g.Activate())
		}
		return m, nil
	}
	return m.activate(m.order[i].Tile.Activate())
}

func (m Model) handleDeckWheel(msg tea.MouseWheelMsg) (tea.Model, tea.Cmd) {
	switch msg.Button {
	case tea.MouseWheelDown:
		m.focus = wrapIndex(m.focus, +1, len(m.order))
	case tea.MouseWheelUp:
		m.focus = wrapIndex(m.focus, -1, len(m.order))
	}
	return m, nil
}

// handleDeckKey owns everything below the quit keys (those live in
// updateDeck so they survive a degraded canvas).
func (m Model) handleDeckKey(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "enter":
		return m.activate(m.order[m.focus].Tile.Activate())
	case "left", "h", "shift+tab":
		m.focus = wrapIndex(m.focus, -1, len(m.order))
	case "right", "l", "tab":
		m.focus = wrapIndex(m.focus, +1, len(m.order))
	case "down", "j":
		// j/k walk the gated column inside a focused gear; between tiles
		// otherwise (h/l always move between tiles — mock D interactions).
		if g, ok := m.order[m.focus].Tile.(widget.Gear); ok {
			m.order[m.focus].Tile = g.CursorNext()
			return m, nil
		}
		m.focus = wrapIndex(m.focus, +1, len(m.order))
	case "up", "k":
		if g, ok := m.order[m.focus].Tile.(widget.Gear); ok {
			m.order[m.focus].Tile = g.CursorPrev()
			return m, nil
		}
		m.focus = wrapIndex(m.focus, -1, len(m.order))
	case "/":
		return m.openPalette()
	}
	return m, nil
}

// wrapIndex moves i by delta with wrap-around over n items.
func wrapIndex(i, delta, n int) int {
	return ((i+delta)%n + n) % n
}

// activate translates a tile intent. TileActivated records the command for
// cmd/gearshifter to inject; ScreenRequested swaps to the palette.
func (m Model) activate(intent tea.Msg) (tea.Model, tea.Cmd) {
	switch intent := intent.(type) {
	case widget.TileActivatedMsg:
		sel := intent.Command
		m.selected = &sel
		m.insertOnly = intent.Insert
		return m.arm()
	case widget.GearShiftedMsg:
		sel := intent.Command
		m.selected = &sel
		m.arg = intent.Value
		return m.arm()
	case widget.ScreenRequestedMsg:
		return m.openPalette()
	}
	return m, nil
}

// arm starts the press frame: the fired tile flashes armed for
// pressFrameDuration, then the tick quits and the selection injects.
func (m Model) arm() (tea.Model, tea.Cmd) {
	m.armed = true
	return m, tea.Tick(pressFrameDuration, func(time.Time) tea.Msg { return pressFrameDoneMsg{} })
}

// openPalette swaps in a fresh palette sized to the current canvas.
func (m Model) openPalette() (tea.Model, tea.Cmd) {
	m.screen = screenPalette
	m.palette = palette.New(m.commands, m.styles)
	updated, _ := m.palette.Update(tea.WindowSizeMsg{Width: m.width, Height: m.height})
	m.palette = updated.(palette.Model)
	return m, nil
}

// routePalette forwards messages to the embedded palette. Esc is
// intercepted (back to deck, not quit); palette-issued cmds are swallowed
// because standalone-palette quit semantics don't apply when embedded.
func (m Model) routePalette(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case "esc":
			m.screen = screenDeck
			return m, nil
		case "ctrl+c":
			return m, tea.Quit
		}
	}
	updated, _ := m.palette.Update(msg)
	m.palette = updated.(palette.Model)
	if sel, ok := m.palette.Selection(); ok {
		m.selected = &sel
		m.insertOnly = m.palette.InsertOnly()
		return m, tea.Quit
	}
	return m, nil
}

// marginX is the deck's horizontal margin — the smallest step of the
// Fibonacci scale, like every other gap (vertical spacing lives with the
// placements in layout).
var marginX = deck.Scale[0]

// compositor lays every tile into a named layer over a base layer holding
// the header/footer. The same compositor renders the view AND answers
// mouse hit-tests, so clicks can never disagree with pixels.
func (m Model) compositor() *lipgloss.Compositor {
	grid := deck.New(m.width - 2*marginX)

	tileRows := 0
	layers := make([]*lipgloss.Layer, 0, len(m.order)+1)
	for i, p := range m.order {
		x, w := grid.Cell(p.Col, p.Tile.Span())
		rs := widget.RenderState{Focused: i == m.focus, Armed: m.armed && i == m.focus}
		layers = append(layers, lipgloss.NewLayer(p.Tile.View(rs, w)).
			ID("tile:"+strconv.Itoa(i)).X(marginX+x).Y(p.Y).Z(1))
		if bottom := p.Y + p.Tile.Rows(); bottom > tileRows {
			tileRows = bottom
		}
	}

	// Base layer: the wordmark — the single authored grid break (Samara):
	// pinned to line 1 as a reverse block, obeying no column boundary —
	// plus the footer hint line.
	margin := strings.Repeat(" ", marginX)
	base := make([]string, 0, tileRows+2)
	base = append(base, margin+m.styles.Chrome.Wordmark.Render(" GEARSHIFTER "))
	for len(base) < tileRows+1 {
		base = append(base, "")
	}
	base = append(base, margin+m.styles.Chrome.Footer.Render("h/l tiles · j/k in gear · Enter fire · / all commands · Esc close"))
	baseLayer := lipgloss.NewLayer(strings.Join(base, "\n")).X(0).Y(0).Z(0)

	return lipgloss.NewCompositor(append([]*lipgloss.Layer{baseLayer}, layers...)...)
}

// hitTile decodes a compositor hit back to an order index.
func hitTile(hit lipgloss.LayerHit) (int, bool) {
	id, ok := strings.CutPrefix(hit.ID(), "tile:")
	if !ok {
		return 0, false
	}
	i, err := strconv.Atoi(id)
	return i, err == nil
}

// minCanvas is the smallest canvas the current placements render on:
// every grid column at least two cells wide, every tile row on screen
// (the footer hint is allowed to clip first). The first, simple
// degradation rule (M3-DECK P4): below this, fail with words — a clear
// message instead of overdrawn tiles. The wordmark retracts with the
// grid (layout law: breaks need a grid to break).
func (m Model) minCanvas() (w, h int) {
	for _, p := range m.order {
		if bottom := p.Y + p.Tile.Rows(); bottom > h {
			h = bottom
		}
	}
	return 2*marginX + deck.MinWidth(), h
}

func (m Model) canvasTooSmall() bool {
	w, h := m.minCanvas()
	return m.width < w || m.height < h
}

func (m Model) viewDeck() string {
	if m.canvasTooSmall() {
		w, h := m.minCanvas()
		return strings.Repeat(" ", marginX) + m.styles.Chrome.Degraded.Render(fmt.Sprintf(
			"canvas %d×%d is too small for this layout (needs %d×%d) — enlarge the popup",
			m.width, m.height, w, h))
	}
	return m.compositor().Render()
}

// Selection returns the command chosen on either screen, if any.
func (m Model) Selection() (catalog.Command, bool) {
	if m.selected == nil {
		return catalog.Command{}, false
	}
	return *m.selected, true
}

// Arg returns the gear value committed with the selection, if any — the
// injection becomes "/command value".
func (m Model) Arg() string { return m.arg }

// InsertOnly reports whether the selection asked to skip Enter.
func (m Model) InsertOnly() bool { return m.insertOnly }

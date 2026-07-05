// Package app is the deck's root Bubble Tea model: screen routing
// (Deck ⇄ Palette), focus management, and intent recording. Like palette,
// app records the user's choice and cmd/gearshifter injects — app never
// imports tmux (ARCHITECTURE.md §2).
package app

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/kylesnowschwartz/gearshifter/internal/catalog"
	"github.com/kylesnowschwartz/gearshifter/internal/deck"
	"github.com/kylesnowschwartz/gearshifter/internal/palette"
	"github.com/kylesnowschwartz/gearshifter/internal/widget"
)

type screen int

const (
	screenDeck screen = iota
	screenPalette
)

var (
	wordmark = lipgloss.NewStyle().Bold(true).Reverse(true)
	faint    = lipgloss.NewStyle().Faint(true)
)

// placement puts a tile at a grid column and row. All geometry flows from
// deck.Grid and the Fibonacci scale — Samara's law: tiles are placed by
// the system, never nudged.
type placement struct {
	tile widget.Tile
	col  int
	y    int
}

// Model routes between the deck screen and the embedded palette.
type Model struct {
	commands []catalog.Command
	order    []*placement
	focus    int

	screen  screen
	palette palette.Model
	width   int
	height  int

	selected   *catalog.Command
	arg        string
	insertOnly bool
}

// New builds the default deck layout from the catalog: gear rail (span 5,
// MODEL over EFFORT) beside a 2×2 button field (span 4 each) — the φ
// split — with the launcher as a full-width bottom bar. Focus order =
// reading order. state marks each gear's live value (V7).
func New(commands []catalog.Command, state catalog.GearState) Model {
	m := Model{commands: commands}
	add := func(t widget.Tile, col, y int) {
		m.order = append(m.order, &placement{tile: t, col: col, y: y})
	}

	model := widget.NewGear(findCommand(commands, "model"), "MODEL",
		[]string{"haiku", "sonnet", "opus", "fable"}, deck.RailSpan).
		WithCurrent(state.Model)
	effort := widget.NewGear(findCommand(commands, "effort"), "EFFORT",
		[]string{"low", "medium", "high", "max"}, deck.RailSpan).
		WithCurrent(state.Effort)
	add(model, 0, bodyY)
	add(effort, 0, bodyY+model.Rows()+rowGap)

	for i, b := range []struct{ name, label string }{
		{"review", "REVIEW"},
		{"context", "CONTEXT"},
		{"compact", "COMPACT"},
		{"resume", "RESUME"},
	} {
		btn := widget.NewButton(findCommand(commands, b.name), b.label, deck.MainSpan/2)
		add(btn, deck.RailSpan+(i%2)*(deck.MainSpan/2), bodyY+(i/2)*(btn.Rows()+rowGap))
	}

	railH := model.Rows() + rowGap + effort.Rows()
	add(widget.NewLauncher(len(commands), deck.Columns), 0, bodyY+railH+rowGap)
	return m
}

// findCommand looks a command up by name so buttons carry the real
// catalog entry (argument hints drive the hint-aware Enter policy). A
// missing name still yields a working button for the bare command.
func findCommand(commands []catalog.Command, name string) catalog.Command {
	for _, c := range commands {
		if c.Name == name {
			return c
		}
	}
	return catalog.Command{Name: name}
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

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		m.width, m.height = size.Width, size.Height
	}
	if m.screen == screenPalette {
		return m.updatePalette(msg)
	}
	return m.updateDeck(msg)
}

func (m Model) updateDeck(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.MouseClickMsg:
		if msg.Button != tea.MouseLeft {
			return m, nil
		}
		// Named-layer hit-testing (M2 P0 decision): the compositor knows
		// every tile's bounds; a hit both focuses and fires — click = do.
		hit := m.compositor().Hit(msg.X, msg.Y)
		i, ok := hitTile(hit)
		if !ok || i >= len(m.order) {
			return m, nil
		}
		m.focus = i
		if g, isGear := m.order[i].tile.(widget.Gear); isGear {
			// Gated column: each value row is its own target — one click
			// to any state. Border/label clicks just focus.
			if v, ok := g.ValueAt(msg.Y - hit.Bounds().Min.Y); ok {
				g = g.WithCursor(v)
				m.order[i].tile = g
				return m.activate(g.Activate())
			}
			return m, nil
		}
		return m.activate(m.order[i].tile.Activate())
	case tea.MouseWheelMsg:
		switch msg.Button {
		case tea.MouseWheelDown:
			m.focus = (m.focus + 1) % len(m.order)
		case tea.MouseWheelUp:
			m.focus = (m.focus - 1 + len(m.order)) % len(m.order)
		}
		return m, nil
	}
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "ctrl+c", "esc", "q":
		return m, tea.Quit
	case "enter":
		return m.activate(m.order[m.focus].tile.Activate())
	case "left", "h", "shift+tab":
		m.focus = (m.focus - 1 + len(m.order)) % len(m.order)
	case "right", "l", "tab":
		m.focus = (m.focus + 1) % len(m.order)
	case "down", "j":
		// j/k walk the gated column inside a focused gear; between tiles
		// otherwise (h/l always move between tiles — mock D interactions).
		if g, ok := m.order[m.focus].tile.(widget.Gear); ok {
			m.order[m.focus].tile = g.CursorNext()
			return m, nil
		}
		m.focus = (m.focus + 1) % len(m.order)
	case "up", "k":
		if g, ok := m.order[m.focus].tile.(widget.Gear); ok {
			m.order[m.focus].tile = g.CursorPrev()
			return m, nil
		}
		m.focus = (m.focus - 1 + len(m.order)) % len(m.order)
	case "/":
		return m.openPalette()
	}
	return m, nil
}

// activate translates a tile intent. TileActivated records the command for
// cmd/gearshifter to inject; ScreenRequested swaps to the palette.
func (m Model) activate(intent tea.Msg) (tea.Model, tea.Cmd) {
	switch intent := intent.(type) {
	case widget.TileActivatedMsg:
		sel := intent.Command
		m.selected = &sel
		return m, tea.Quit
	case widget.GearShiftedMsg:
		sel := intent.Command
		m.selected = &sel
		m.arg = intent.Value
		return m, tea.Quit
	case widget.ScreenRequestedMsg:
		return m.openPalette()
	}
	return m, nil
}

// openPalette swaps in a fresh palette sized to the current canvas.
func (m Model) openPalette() (tea.Model, tea.Cmd) {
	m.screen = screenPalette
	m.palette = palette.New(m.commands)
	updated, _ := m.palette.Update(tea.WindowSizeMsg{Width: m.width, Height: m.height})
	m.palette = updated.(palette.Model)
	return m, nil
}

// updatePalette forwards messages to the embedded palette. Esc is
// intercepted (back to deck, not quit); palette-issued cmds are swallowed
// because standalone-palette quit semantics don't apply when embedded.
func (m Model) updatePalette(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (m Model) View() tea.View {
	if m.screen == screenPalette {
		return m.palette.View()
	}
	var view tea.View
	view.AltScreen = true
	view.MouseMode = tea.MouseModeCellMotion // clicks arrive in P1; requested now so tmux never scrolls
	if m.width == 0 {
		return view
	}
	view.Content = m.viewDeck()
	return view
}

// Layout constants, all from the Fibonacci scale (deck.Scale): 1-cell
// margin and gutters, tiles start under a 1-line header + 1-line gap.
const (
	marginX = 1
	bodyY   = 2
	rowGap  = 1
)

// compositor lays every tile into a named layer over a base layer holding
// the header/footer. The same compositor renders the view AND answers
// mouse hit-tests, so clicks can never disagree with pixels.
func (m Model) compositor() *lipgloss.Compositor {
	grid := deck.New(m.width - 2*marginX)

	tileRows := 0
	layers := make([]*lipgloss.Layer, 0, len(m.order)+1)
	for i, p := range m.order {
		x, w := grid.Cell(p.col, p.tile.Span())
		layers = append(layers, lipgloss.NewLayer(p.tile.View(i == m.focus, w)).
			ID("tile:"+strconv.Itoa(i)).X(marginX+x).Y(p.y).Z(1))
		if bottom := p.y + p.tile.Rows(); bottom > tileRows {
			tileRows = bottom
		}
	}

	// Base layer: the wordmark — the single authored grid break (Samara):
	// pinned to line 1 as a reverse block, obeying no column boundary —
	// plus the footer hint line.
	margin := strings.Repeat(" ", marginX)
	base := make([]string, 0, tileRows+2)
	base = append(base, margin+wordmark.Render(" GEARSHIFTER "))
	for len(base) < tileRows+1 {
		base = append(base, "")
	}
	base = append(base, margin+faint.Render("h/l tiles · j/k in gear · Enter fire · / all commands · Esc close"))
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

func (m Model) viewDeck() string {
	return m.compositor().Render()
}

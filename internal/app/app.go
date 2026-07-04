// Package app is the deck's root Bubble Tea model: screen routing
// (Deck ⇄ Palette), focus management, and intent recording. Like palette,
// app records the user's choice and cmd/gearshifter injects — app never
// imports tmux (ARCHITECTURE.md §2).
package app

import (
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

// placement puts a tile at a grid column; rows stack in slice order within
// their zone (rail or main). All geometry flows from deck.Grid — Samara's
// law: tiles are placed by the system, never nudged.
type placement struct {
	tile widget.Tile
	col  int
}

// Model routes between the deck screen and the embedded palette.
type Model struct {
	commands []catalog.Command
	rail     []placement // launcher now; gears in P2
	main     []placement // buttons, two per row
	order    []*placement
	focus    int

	screen  screen
	palette palette.Model
	width   int
	height  int

	selected   *catalog.Command
	insertOnly bool
}

// New builds the default deck layout from the catalog: launcher rail
// (span 5) beside a 2×2 button field (span 4 each) — the φ split.
// The gears claim the rail in M3 P2; the launcher then drops to a bar.
func New(commands []catalog.Command) Model {
	m := Model{commands: commands}
	buttonRows := 2
	launcherRows := buttonRows*4 + 1 // match the button block height (4-row tiles + gutter)
	m.rail = []placement{
		{tile: widget.NewLauncher(len(commands), deck.RailSpan, launcherRows), col: 0},
	}
	for i, b := range []struct{ name, label string }{
		{"review", "REVIEW"},
		{"context", "CONTEXT"},
		{"compact", "COMPACT"},
		{"resume", "RESUME"},
	} {
		cmd := findCommand(commands, b.name)
		m.main = append(m.main, placement{
			tile: widget.NewButton(cmd, b.label, deck.MainSpan/2),
			col:  deck.RailSpan + (i%2)*(deck.MainSpan/2),
		})
	}
	// Focus order = reading order: launcher, then buttons.
	for i := range m.rail {
		m.order = append(m.order, &m.rail[i])
	}
	for i := range m.main {
		m.order = append(m.order, &m.main[i])
	}
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
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "ctrl+c", "esc", "q":
		return m, tea.Quit
	case "enter":
		return m.activate(m.order[m.focus].tile.Activate())
	case "left", "h", "up", "k", "shift+tab":
		m.focus = (m.focus - 1 + len(m.order)) % len(m.order)
	case "right", "l", "down", "j", "tab":
		m.focus = (m.focus + 1) % len(m.order)
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

func (m Model) viewDeck() string {
	grid := deck.New(m.width - 2) // margin: 1 cell each side (Scale[0])
	margin := strings.Repeat(" ", 1)

	// The wordmark is the single authored grid break (Samara): pinned to
	// line 1, rendered as a reverse block, obeying no column boundary.
	header := margin + wordmark.Render(" GEARSHIFTER ")

	_, railW := grid.Cell(0, deck.RailSpan)
	rail := m.rail[0].tile.View(m.order[m.focus] == &m.rail[0], railW)

	// Button field: two per row, geometry from the grid columns. Row
	// spacers are relative to the rail's right edge so absolute grid x
	// positions survive the horizontal join.
	var rows []string
	for i := 0; i < len(m.main); i += 2 {
		row := make([]string, 0, 4)
		rowX := railW
		for j := i; j < min(i+2, len(m.main)); j++ {
			p := m.main[j]
			x, w := grid.Cell(p.col, p.tile.Span())
			row = append(row, strings.Repeat(" ", x-rowX), p.tile.View(m.order[m.focus] == &m.main[j], w))
			rowX = x + w
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, row...))
	}
	mainBlock := strings.Join(rows, "\n\n") // 1-row gutter between button rows (Scale[0])

	body := lipgloss.JoinHorizontal(lipgloss.Top, margin, rail, mainBlock)

	footer := margin + faint.Render("h/l move · Enter fire · / all commands · Esc close")

	return strings.Join([]string{header, "", body, "", footer}, "\n")
}

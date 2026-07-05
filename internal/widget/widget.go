// Package widget holds the deck tile archetypes (Button, Gear, Launcher).
// Tiles emit intent Msgs and never touch tmux (ARCHITECTURE.md §2); widget
// knows commands, not layout — geometry arrives as a width from the deck
// grid via View.
package widget

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/kylesnowschwartz/gearshifter/internal/catalog"
)

// TileActivatedMsg reports a fired tile; the app layer translates it into
// an injection (past-tense intent, ARCHITECTURE.md §3).
type TileActivatedMsg struct {
	Command catalog.Command
}

// GearShiftedMsg reports a committed gear value; the app injects
// "/command value".
type GearShiftedMsg struct {
	Command catalog.Command
	Value   string
}

// ScreenRequestedMsg asks the app to swap screens (Launcher → palette).
type ScreenRequestedMsg struct{}

// Tile is a deck widget. P0 tiles are passive views the app activates;
// tiles with interactive innards (Gear) arrive in M3 P2.
type Tile interface {
	// Activate returns the tile's Enter/click intent.
	Activate() tea.Msg
	// View renders the tile at exactly width cells.
	View(focused bool, width int) string
	// Span is the tile's width in grid columns.
	Span() int
	// Rows is the tile's height in cells.
	Rows() int
}

var (
	faint      = lipgloss.NewStyle().Faint(true)
	reversed   = lipgloss.NewStyle().Reverse(true)
	normalTile = lipgloss.NewStyle().Border(lipgloss.NormalBorder())
	focusTile  = lipgloss.NewStyle().Border(lipgloss.DoubleBorder())
)

// box renders bordered tile chrome around pre-styled content rows. Mock D
// chrome: single border normally, double-border focus ring. lipgloss v2
// Style.Width is the TOTAL frame width (borders included) — content rows
// get width-2 cells.
func box(focused bool, width int, rows ...string) string {
	style := normalTile
	if focused {
		style = focusTile
	}
	return style.Width(width).Render(strings.Join(rows, "\n"))
}

// center pads s to exactly width cells, centered. Rows are styled AFTER
// padding, one style per full row (nested styles reset ANSI mid-row — M2
// gotcha). Cell math via lipgloss.Width — labels contain multibyte runes.
func center(s string, width int) string {
	w := lipgloss.Width(s)
	if w > width {
		return string([]rune(s)[:max(0, width)])
	}
	left := (width - w) / 2
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", width-w-left)
}

// Button fires a slash command: big centered label, dim /command sublabel.
type Button struct {
	Cmd   catalog.Command
	Label string
	span  int
}

func NewButton(cmd catalog.Command, label string, span int) Button {
	return Button{Cmd: cmd, Label: label, span: span}
}

func (b Button) Activate() tea.Msg { return TileActivatedMsg{Command: b.Cmd} }
func (b Button) Span() int         { return b.span }
func (b Button) Rows() int         { return 4 } // border 2 + label + sublabel

func (b Button) View(focused bool, width int) string {
	inner := width - 2
	label := center(b.Label, inner)
	if focused {
		label = reversed.Render(label)
	}
	sub := faint.Render(center("/"+b.Cmd.Name, inner))
	return box(focused, width, label, sub)
}

// Gear selects one value of an enum command (gated column, locked UX
// decision: all values visible, current highlighted, one click to any
// state). Current starts unknown (-1) — stateless until the V7 spike
// answers how to sniff the session's live value.
type Gear struct {
	Cmd     catalog.Command
	Label   string
	Values  []string
	current int
	cursor  int
	span    int
}

func NewGear(cmd catalog.Command, label string, values []string, span int) Gear {
	return Gear{Cmd: cmd, Label: label, Values: values, current: -1, span: span}
}

// Activate commits the cursor value.
func (g Gear) Activate() tea.Msg {
	return GearShiftedMsg{Command: g.Cmd, Value: g.Values[g.cursor]}
}

func (g Gear) Span() int { return g.span }
func (g Gear) Rows() int { return len(g.Values) + 2 }

// CursorNext / CursorPrev walk the gated column (j/k inside a focused
// gear), wrapping.
func (g Gear) CursorNext() Gear {
	g.cursor = (g.cursor + 1) % len(g.Values)
	return g
}

func (g Gear) CursorPrev() Gear {
	g.cursor = (g.cursor - 1 + len(g.Values)) % len(g.Values)
	return g
}

// WithCursor points the cursor at a clicked value row.
func (g Gear) WithCursor(i int) Gear {
	if i >= 0 && i < len(g.Values) {
		g.cursor = i
	}
	return g
}

// ValueAt maps a row offset inside the tile (0 = top border) to a value
// index, for mouse hits.
func (g Gear) ValueAt(rowInTile int) (int, bool) {
	i := rowInTile - 1
	return i, i >= 0 && i < len(g.Values)
}

// View hand-rolls the box so the label embeds in the top border
// (┌ MODEL ───┐, mock D chrome); border charset matches the lipgloss
// Normal/Double borders the buttons use.
func (g Gear) View(focused bool, width int) string {
	h, v, tl, tr, bl, br := "─", "│", "┌", "┐", "└", "┘"
	if focused {
		h, v, tl, tr, bl, br = "═", "║", "╔", "╗", "╚", "╝"
	}
	inner := width - 2
	title := " " + g.Label + " "
	if lipgloss.Width(title) > inner {
		title = string([]rune(title)[:max(0, inner)])
	}
	rows := []string{tl + title + strings.Repeat(h, max(0, inner-lipgloss.Width(title))) + tr}
	for i, val := range g.Values {
		mark := "  "
		if i == g.current {
			mark = "▐ " // the session's current value (V7 fills this in)
		}
		line := mark + val
		line += strings.Repeat(" ", max(0, inner-lipgloss.Width(line)))
		switch {
		case focused && i == g.cursor:
			line = reversed.Render(line)
		case i == g.current:
			line = lipgloss.NewStyle().Bold(true).Render(line)
		}
		rows = append(rows, v+line+v)
	}
	rows = append(rows, bl+strings.Repeat(h, inner)+br)
	return strings.Join(rows, "\n")
}

// Launcher opens the searchable palette screen (the escape hatch to the
// full catalog): a 3-row full-width bar, label left, catalog count right.
type Launcher struct {
	Count int
	span  int
}

func NewLauncher(count, span int) Launcher {
	return Launcher{Count: count, span: span}
}

func (l Launcher) Activate() tea.Msg { return ScreenRequestedMsg{} }
func (l Launcher) Span() int         { return l.span }
func (l Launcher) Rows() int         { return 3 }

func (l Launcher) View(focused bool, width int) string {
	inner := width - 2
	label := " ALL COMMANDS…"
	if focused {
		label = reversed.Render(label)
	}
	count := strconv.Itoa(l.Count) + " cmds "
	gap := max(0, inner-lipgloss.Width(" ALL COMMANDS…")-lipgloss.Width(count))
	line := label + strings.Repeat(" ", gap) + faint.Render(count)
	return box(focused, width, line)
}

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
// chrome: single border normally, double-border focus ring.
func box(focused bool, width int, rows ...string) string {
	style := normalTile
	if focused {
		style = focusTile
	}
	return style.Width(width - 2).Render(strings.Join(rows, "\n"))
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

// Launcher opens the searchable palette screen (the escape hatch to the
// full catalog).
type Launcher struct {
	Count int // catalog size, shown as microcopy
	span  int
	rows  int
}

func NewLauncher(count, span, rows int) Launcher {
	return Launcher{Count: count, span: span, rows: rows}
}

func (l Launcher) Activate() tea.Msg { return ScreenRequestedMsg{} }
func (l Launcher) Span() int         { return l.span }
func (l Launcher) Rows() int         { return l.rows }

func (l Launcher) View(focused bool, width int) string {
	inner := width - 2
	content := make([]string, 0, l.rows-2)
	// Vertically center the two content lines in the tall tile.
	pad := (l.rows - 2 - 2) / 2
	for i := 0; i < pad; i++ {
		content = append(content, strings.Repeat(" ", inner))
	}
	label := center("ALL COMMANDS…", inner)
	if focused {
		label = reversed.Render(label)
	}
	content = append(content, label, faint.Render(center(strconv.Itoa(l.Count)+" cmds", inner)))
	for len(content) < l.rows-2 {
		content = append(content, strings.Repeat(" ", inner))
	}
	return box(focused, width, content...)
}

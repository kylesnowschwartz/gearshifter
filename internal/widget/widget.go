// Package widget holds the deck tile archetypes (Button, Gear, Launcher).
// Tiles emit intent Msgs and never touch tmux (ARCHITECTURE.md §2); widget
// knows commands, not layout — geometry arrives as a width from the deck
// grid via View, and every style arrives from *theme.Styles at
// construction (widgets never build styles).
package widget

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/kylesnowschwartz/gearshifter/internal/catalog"
	"github.com/kylesnowschwartz/gearshifter/internal/theme"
)

// TileActivatedMsg reports a fired tile; the app layer translates it into
// an injection (past-tense intent, ARCHITECTURE.md §3). Insert asks for
// insert-without-Enter — the tile-level Tab (D2 policy): "/goal " lands
// in the prompt ready for its argument instead of firing bare.
type TileActivatedMsg struct {
	Command catalog.Command
	Insert  bool
}

// GearShiftedMsg reports a committed gear value; the app injects
// "/command value".
type GearShiftedMsg struct {
	Command catalog.Command
	Value   string
}

// ScreenRequestedMsg asks the app to swap screens (Launcher → palette).
type ScreenRequestedMsg struct{}

// RenderState is what a tile needs to know about itself to render:
// whether it holds the focus ring, and whether it is inside the armed
// press frame (the ~150ms flash between fire and popup close, P2).
type RenderState struct {
	Focused bool
	Armed   bool
}

// Tile is a deck widget. P0 tiles are passive views the app activates;
// tiles with interactive innards (Gear) arrive in M3 P2.
type Tile interface {
	// Activate returns the tile's Enter/click intent.
	Activate() tea.Msg
	// View renders the tile at exactly width cells.
	View(rs RenderState, width int) string
	// Span is the tile's width in grid columns.
	Span() int
	// Rows is the tile's height in cells.
	Rows() int
}

// borderRows/borderCols are the chrome every tile pays: top + bottom
// border rows, left + right border columns.
const (
	borderRows = 2
	borderCols = 2
)

// tileFrame is a tile's chrome resolved for one render: the charset and
// border style the render state picked, plus the inner content width.
// Every tile frames the same way — colored themes signal focus by border
// color, plain by charset swap — so the pick and the edge assembly live
// here, once. Border chars and content are styled as separate sequential
// segments, never nested (M2 gotcha).
type tileFrame struct {
	chars lipgloss.Border
	style lipgloss.Style
	inner int
}

func newTileFrame(fs theme.FrameStyles, rs RenderState, width int) tileFrame {
	chars, style := fs.Border, fs.Frame
	if rs.Focused || rs.Armed {
		chars, style = fs.BorderFocus, fs.FrameFocus
	}
	return tileFrame{chars: chars, style: style, inner: max(0, width-borderCols)}
}

// top renders a plain top border row: ┌────┐.
func (f tileFrame) top() string {
	return f.style.Render(f.chars.TopLeft + strings.Repeat(f.chars.Top, f.inner) + f.chars.TopRight)
}

// titledTop embeds pre-truncated text directly in the top border:
// ┌ MODEL ───┐ (mock D chrome).
func (f tileFrame) titledTop(title string) string {
	return f.style.Render(f.chars.TopLeft + title +
		strings.Repeat(f.chars.Top, max(0, f.inner-lipgloss.Width(title))) + f.chars.TopRight)
}

// bottom renders a plain bottom border row: └────┘.
func (f tileFrame) bottom() string {
	return f.style.Render(f.chars.BottomLeft + strings.Repeat(f.chars.Bottom, f.inner) + f.chars.BottomRight)
}

// nameplateBottom splices styled text between joint caps in the bottom
// border: └┤ /compact ├──┘ (superfile's border-embedded info slot). The
// plate must already fit within inner-nameplateJoints cells.
func (f tileFrame) nameplateBottom(plate string, plateStyle lipgloss.Style) string {
	w := lipgloss.Width(plate)
	return f.style.Render(f.chars.BottomLeft+f.chars.MiddleRight) +
		plateStyle.Render(plate) +
		f.style.Render(f.chars.MiddleLeft+strings.Repeat(f.chars.Bottom, f.inner-w-nameplateJoints)+f.chars.BottomRight)
}

// row wraps one pre-styled, exactly-inner-wide content row in side
// borders: │content│.
func (f tileFrame) row(content string) string {
	side := f.style.Render(f.chars.Left)
	return side + content + side
}

// truncate cuts s to at most width display cells. Rune-count slicing
// would overflow the tile on wide runes (CJK/emoji labels from a user
// layout.toml) and desync the compositor's hit-testing.
func truncate(s string, width int) string {
	if lipgloss.Width(s) <= width {
		return s
	}
	var b strings.Builder
	w := 0
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if w+rw > width {
			break
		}
		b.WriteRune(r)
		w += rw
	}
	return b.String()
}

// center pads s to exactly width cells, centered. Rows are styled AFTER
// padding, one style per full row (nested styles reset ANSI mid-row — M2
// gotcha). Cell math via lipgloss.Width — labels contain multibyte runes.
func center(s string, width int) string {
	s = truncate(s, max(0, width))
	w := lipgloss.Width(s)
	left := (width - w) / 2
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", width-w-left)
}

// Button fires a slash command: big centered label, /command nameplate
// in the bottom border.
type Button struct {
	Cmd    catalog.Command
	Label  string
	Insert bool // insert-without-Enter instead of firing (layout.toml `insert = true`)
	styles *theme.Styles
	span   int
}

func NewButton(st *theme.Styles, cmd catalog.Command, label string, span int) Button {
	return Button{Cmd: cmd, Label: label, styles: st, span: span}
}

// WithInsert makes the button insert "/cmd " without Enter when fired.
func (b Button) WithInsert() Button {
	b.Insert = true
	return b
}

// buttonContentRows: one big centered label; the /command lives in the
// bottom border, which bought back the old sublabel row (superfile's
// border-embedded info slot, tui-research/02 §3; promoted 2026-07-05).
const buttonContentRows = 1

// nameplateJoints is the two border caps around the plate (┤ ├);
// nameplateMin is the smallest plate worth splicing (" /a " = 4 cells)
// before the bottom border degrades to plain.
const (
	nameplateJoints = 2
	nameplateMin    = 4
)

func (b Button) Activate() tea.Msg { return TileActivatedMsg{Command: b.Cmd, Insert: b.Insert} }
func (b Button) Span() int         { return b.span }
func (b Button) Rows() int         { return borderRows + buttonContentRows }

// View renders the frame with the /command nameplate spliced into the
// bottom border: └┤ /compact ├──┘. When the nameplate can't fit, it
// degrades to a plain border (superfile's truncate-away fallback).
func (b Button) View(rs RenderState, width int) string {
	st := b.styles.Button
	f := newTileFrame(st.FrameStyles, rs, width)
	label := center(b.Label, f.inner)
	switch {
	case rs.Armed:
		label = b.styles.Armed.Render(label)
	case rs.Focused:
		label = st.LabelFocus.Render(label)
	default:
		label = st.Label.Render(label)
	}
	// The nameplate truncates before it disappears: a button must show
	// which command it fires (review finding — the old sublabel always
	// did), so only a tile too narrow for any identity drops it.
	plate := truncate(" /"+b.Cmd.Name+" ", max(0, f.inner-nameplateJoints))
	bottom := f.bottom()
	if lipgloss.Width(plate) >= nameplateMin {
		bottom = f.nameplateBottom(plate, st.Sub)
	}
	return f.top() + "\n" + f.row(label) + "\n" + bottom
}

// Gear selects one value of an enum command (gated column, locked UX
// decision: all values visible, current highlighted, one click to any
// state). Current starts unknown (-1) — stateless until the V7 spike
// answers how to sniff the session's live value.
type Gear struct {
	Cmd     catalog.Command
	Label   string
	Values  []string
	styles  *theme.Styles
	current int
	cursor  int
	span    int
}

func NewGear(st *theme.Styles, cmd catalog.Command, label string, values []string, span int) Gear {
	return Gear{Cmd: cmd, Label: label, Values: values, styles: st, current: -1, span: span}
}

// Activate commits the cursor value.
func (g Gear) Activate() tea.Msg {
	return GearShiftedMsg{Command: g.Cmd, Value: g.Values[g.cursor]}
}

func (g Gear) Span() int { return g.span }
func (g Gear) Rows() int { return borderRows + len(g.Values) } // one row per value

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

// WithCurrent marks the session's live value (▐ + bold) and starts the
// cursor there. Settings values come in several shapes — "opus" but also
// "claude-fable-5[1m]" — so an exact match wins first, then a value that
// appears inside the setting (case-insensitive; exact-first keeps "opus"
// from being claimed by a value like "o"). No match = stateless render.
func (g Gear) WithCurrent(setting string) Gear {
	g.current = -1
	setting = strings.ToLower(setting)
	if setting == "" {
		return g
	}
	match := -1
	for i, v := range g.Values {
		v = strings.ToLower(v)
		if setting == v {
			match = i
			break
		}
		if match == -1 && strings.Contains(setting, v) {
			match = i
		}
	}
	if match >= 0 {
		g.current = match
		g.cursor = match
	}
	return g
}

// ValueAt maps a row offset inside the tile (0 = top border) to a value
// index, for mouse hits.
func (g Gear) ValueAt(rowInTile int) (int, bool) {
	i := rowInTile - 1
	return i, i >= 0 && i < len(g.Values)
}

// View renders the frame with the label embedded in the top border
// (┌ MODEL ───┐) over one row per value.
func (g Gear) View(rs RenderState, width int) string {
	st := g.styles.Gear
	f := newTileFrame(st.FrameStyles, rs, width)
	rows := []string{f.titledTop(truncate(" "+g.Label+" ", f.inner))}
	for i, val := range g.Values {
		mark := theme.MarkBlank
		if i == g.current {
			mark = theme.MarkCurrent // the session's current value (V7 fills this in)
		}
		// Truncate before padding: an over-wide value from a user
		// layout.toml would bleed past the tile and desync the
		// compositor's hit-testing (review finding).
		line := truncate(mark+val, f.inner)
		line += strings.Repeat(" ", max(0, f.inner-lipgloss.Width(line)))
		switch {
		case rs.Armed && i == g.cursor:
			line = g.styles.Armed.Render(line)
		case rs.Focused && i == g.cursor:
			line = st.ValueCursor.Render(line)
		case i == g.current:
			line = st.ValueCurrent.Render(line)
		default:
			line = st.Value.Render(line)
		}
		rows = append(rows, f.row(line))
	}
	rows = append(rows, f.bottom())
	return strings.Join(rows, "\n")
}

// Launcher opens the searchable palette screen (the escape hatch to the
// full catalog): a 3-row full-width bar, label left, catalog count right.
type Launcher struct {
	Count  int
	styles *theme.Styles
	span   int
}

func NewLauncher(st *theme.Styles, count, span int) Launcher {
	return Launcher{Count: count, styles: st, span: span}
}

// launcherContentRows: the bar is a single label/count line.
const launcherContentRows = 1

func (l Launcher) Activate() tea.Msg { return ScreenRequestedMsg{} }
func (l Launcher) Span() int         { return l.span }
func (l Launcher) Rows() int         { return borderRows + launcherContentRows }

func (l Launcher) View(rs RenderState, width int) string {
	st := l.styles.Launcher
	// no armed frame: the launcher swaps screens, it doesn't fire
	f := newTileFrame(st.FrameStyles, RenderState{Focused: rs.Focused}, width)
	labelStyle := st.Label
	if rs.Focused {
		labelStyle = st.LabelFocus
	}
	label := " ALL COMMANDS…"
	count := strconv.Itoa(l.Count) + " cmds "
	// The count is all-or-nothing: when the bar is too narrow for both,
	// the label wins and pads out alone — rows must never exceed the
	// tile width (compositor hit-testing; the old box() path wrapped
	// instead, silently breaking Rows()=3).
	var line string
	if gap := f.inner - lipgloss.Width(label) - lipgloss.Width(count); gap >= 0 {
		line = labelStyle.Render(label) + strings.Repeat(" ", gap) + st.Count.Render(count)
	} else {
		label = truncate(label, f.inner)
		line = labelStyle.Render(label) + strings.Repeat(" ", max(0, f.inner-lipgloss.Width(label)))
	}
	return f.top() + "\n" + f.row(line) + "\n" + f.bottom()
}

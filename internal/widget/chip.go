// Chips are the strip's compact tile forms (STRIP-EMBED step 2): one
// row, no frame, packed by the app's flow instead of the 13-column
// grid. Same Tile interface, same intent Msgs — a chip is a tile that
// spends its chrome budget on a glyph instead of borders.
package widget

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/kylesnowschwartz/gearshifter/internal/catalog"
	"github.com/kylesnowschwartz/gearshifter/internal/theme"
)

// FlowTile is a tile the compact flow can pack: it knows its natural
// width (the flow grants exactly that many cells, wrapping to the next
// row when a chip doesn't fit).
type FlowTile interface {
	Tile
	NaturalWidth() int
}

// chipPad is the one-cell breathing room on each side of a chip's
// content — it doubles as the click target's slack.
const chipPad = 1

// Chip fires a slash command: `⧉ COPY` on one row.
type Chip struct {
	Cmd    catalog.Command
	Label  string
	Glyph  string
	Insert bool
	styles *theme.Styles
}

func NewChip(st *theme.Styles, cmd catalog.Command, label, glyph string, insert bool) Chip {
	return Chip{Cmd: cmd, Label: label, Glyph: glyph, Insert: insert, styles: st}
}

func (c Chip) Activate() tea.Msg { return TileActivatedMsg{Command: c.Cmd, Insert: c.Insert} }
func (c Chip) Span() int         { return 1 } // compact flow ignores grid spans
func (c Chip) Rows() int         { return 1 }

func (c Chip) NaturalWidth() int {
	return chipPad + lipgloss.Width(c.Glyph) + 1 + lipgloss.Width(c.Label) + chipPad
}

func (c Chip) View(rs RenderState, width int) string {
	line := pad(c.Glyph+" "+c.Label, width)
	st := c.styles.Button
	switch {
	case rs.Armed:
		return c.styles.Armed.Render(line)
	case rs.Focused:
		return st.LabelFocus.Render(line)
	}
	return st.Label.Render(line)
}

// GearChip is the gear's compact form. Collapsed it is a live-state
// badge (`M:fable`); expanded it becomes the gated column rotated 90° —
// all values on one row, current marked, cursor tracking hover/keys —
// per the locked gear UX (all values visible, one click to any state).
type GearChip struct {
	Cmd      catalog.Command
	Label    string
	Values   []string
	Expanded bool
	styles   *theme.Styles
	current  int
	cursor   int
}

func NewGearChip(st *theme.Styles, cmd catalog.Command, label string, values []string) GearChip {
	return GearChip{Cmd: cmd, Label: label, Values: values, styles: st, current: -1}
}

// Activate commits the cursor value (meaningful only while expanded;
// the app expands a collapsed chip instead of activating it).
func (g GearChip) Activate() tea.Msg {
	return GearShiftedMsg{Command: g.Cmd, Value: g.Values[g.cursor]}
}

func (g GearChip) Span() int { return 1 }
func (g GearChip) Rows() int { return 1 }

// Expand / Collapse toggle the value row. Expanding parks the cursor on
// the current value so Enter with no navigation re-commits the status
// quo, never a surprise.
func (g GearChip) Expand() GearChip {
	g.Expanded = true
	if g.current >= 0 {
		g.cursor = g.current
	}
	return g
}

func (g GearChip) Collapse() GearChip {
	g.Expanded = false
	return g
}

func (g GearChip) CursorNext() GearChip {
	g.cursor = (g.cursor + 1) % len(g.Values)
	return g
}

func (g GearChip) CursorPrev() GearChip {
	g.cursor = (g.cursor - 1 + len(g.Values)) % len(g.Values)
	return g
}

// WithCursor points the cursor at a hovered/clicked value.
func (g GearChip) WithCursor(i int) GearChip {
	if i >= 0 && i < len(g.Values) {
		g.cursor = i
	}
	return g
}

// WithCurrent marks the session's live value (matchValue is the one
// matching rule both gear forms share); the cursor follows only while
// collapsed — an expanded row's cursor belongs to the user.
func (g GearChip) WithCurrent(setting string) GearChip {
	g.current = matchValue(g.Values, setting)
	if g.current >= 0 && !g.Expanded {
		g.cursor = g.current
	}
	return g
}

// badge is the collapsed form: `M:fable` (label initial + current
// value, `—` when stateless).
func (g GearChip) badge() string {
	val := "—"
	if g.current >= 0 {
		val = g.Values[g.current]
	}
	return g.badgeInitial() + ":" + val
}

// segment is one value's cell range inside the expanded row — the ONE
// geometry both View and ValueAtX read, so hover/click can never
// disagree with pixels.
type segment struct {
	start, end int // [start, end) in tile-local cells
	value      int
}

// segments lays out the expanded row: ` M: haiku sonnet ▐fable max `.
func (g GearChip) segments() (prefix string, segs []segment) {
	prefix = strings.Repeat(" ", chipPad) + g.badgeInitial() + ": "
	x := lipgloss.Width(prefix)
	for i, v := range g.Values {
		cell := g.valueCell(i, v)
		w := lipgloss.Width(cell)
		segs = append(segs, segment{start: x, end: x + w, value: i})
		x += w + 1 // one-space separator
	}
	return prefix, segs
}

func (g GearChip) badgeInitial() string {
	if g.Label == "" {
		return " "
	}
	return string([]rune(g.Label)[0])
}

// valueCell renders one value's text (mark included, unstyled).
func (g GearChip) valueCell(i int, v string) string {
	if i == g.current {
		return "▐" + v
	}
	return v
}

func (g GearChip) NaturalWidth() int {
	if !g.Expanded {
		return chipPad + lipgloss.Width(g.badge()) + chipPad
	}
	_, segs := g.segments()
	if len(segs) == 0 {
		return chipPad * 2
	}
	return segs[len(segs)-1].end + chipPad
}

// SettingName / Remark implement Remarkable for the refresh loop.
func (g GearChip) SettingName() string      { return g.Cmd.Name }
func (g GearChip) Remark(value string) Tile { return g.WithCurrent(value) }

// fittingSegments counts the leading segments whose cells fit within
// limit — the expanded row's truncation boundary (whole values only).
func fittingSegments(segs []segment, limit int) int {
	for i, s := range segs {
		if s.end > limit {
			return i
		}
	}
	return len(segs)
}

// ValueAtX maps a cell offset inside the expanded tile to a value
// index, for mouse hits. False when collapsed or in the gaps.
func (g GearChip) ValueAtX(xInTile int) (int, bool) {
	if !g.Expanded {
		return 0, false
	}
	_, segs := g.segments()
	for _, s := range segs {
		if xInTile >= s.start && xInTile < s.end {
			return s.value, true
		}
	}
	return 0, false
}

func (g GearChip) View(rs RenderState, width int) string {
	st := g.styles.Gear
	if !g.Expanded {
		line := pad(g.badge(), width)
		switch {
		case rs.Armed:
			return g.styles.Armed.Render(line)
		case rs.Focused:
			return st.ValueCursor.Render(line)
		case g.current >= 0:
			return st.ValueCurrent.Render(line)
		}
		return st.Value.Render(line)
	}
	// Expanded: style each value cell on its own (sequential segments,
	// never nested — M2 gotcha), separators unstyled. Values that don't
	// fit the granted width are cut at the segment boundary and marked
	// with an ellipsis — never mid-cell, never mid-ANSI (review
	// 2026-07-06: the row used to overflow its layer; the renderer clips
	// silently, so clipped values looked absent while j/k could still
	// fire them with no hint they existed).
	prefix, segs := g.segments()
	var b strings.Builder
	b.WriteString(st.Value.Render(prefix))
	shown := fittingSegments(segs, width)
	if shown < len(segs) {
		// Something is cut: re-fit with one cell reserved for the mark.
		shown = fittingSegments(segs, width-1)
	}
	for i, s := range segs[:shown] {
		if i > 0 {
			b.WriteString(" ")
		}
		cell := g.valueCell(s.value, g.Values[s.value])
		switch {
		case rs.Armed && s.value == g.cursor:
			b.WriteString(g.styles.Armed.Render(cell))
		case rs.Focused && s.value == g.cursor:
			// The cursor highlight is a focus artifact: when hover
			// leaves the strip (rs.Focused drops with the ring), a
			// lingering white block would read as a stuck selection
			// (companion QA 2026-07-06). The current-value mark stays.
			b.WriteString(st.ValueCursor.Render(cell))
		case s.value == g.current:
			b.WriteString(st.ValueCurrent.Render(cell))
		default:
			b.WriteString(st.Value.Render(cell))
		}
	}
	if shown < len(segs) {
		b.WriteString(st.Value.Render("…"))
	}
	line := b.String()
	if gap := width - lipgloss.Width(line); gap > 0 {
		line += strings.Repeat(" ", gap)
	}
	return line
}

// LauncherChip opens the palette: `/ ALL COMMANDS (101)` on one row.
type LauncherChip struct {
	Count  int
	styles *theme.Styles
}

func NewLauncherChip(st *theme.Styles, count int) LauncherChip {
	return LauncherChip{Count: count, styles: st}
}

func (l LauncherChip) Activate() tea.Msg { return ScreenRequestedMsg{} }
func (l LauncherChip) Span() int         { return 1 }
func (l LauncherChip) Rows() int         { return 1 }

func (l LauncherChip) label() string {
	return "/ ALL COMMANDS (" + strconv.Itoa(l.Count) + ")"
}

func (l LauncherChip) NaturalWidth() int {
	return chipPad + lipgloss.Width(l.label()) + chipPad
}

func (l LauncherChip) View(rs RenderState, width int) string {
	line := pad(l.label(), width)
	st := l.styles.Launcher
	if rs.Focused {
		return st.LabelFocus.Render(line)
	}
	return st.Label.Render(line)
}

// pad centers nothing: chips are left-aligned inside one pad cell each
// side, truncated when the flow granted less than natural width.
func pad(s string, width int) string {
	s = Truncate(s, max(0, width-2*chipPad))
	line := strings.Repeat(" ", chipPad) + s
	if gap := width - lipgloss.Width(line); gap > 0 {
		line += strings.Repeat(" ", gap)
	}
	return line
}

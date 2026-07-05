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
// injects the recorded selection — or, in persistent mode, delivers it
// mid-loop and keeps running.
type pressFrameDoneMsg struct{}

// stateRefreshInterval is persistent mode's gear-state poll cadence — a
// long-lived strip can't capture state once at startup the way the popup
// does. Slow enough to be invisible, fast enough that a /model change
// shows within a breath.
const stateRefreshInterval = 5 * time.Second

// stateRefreshMsg carries freshly polled gear settings (command name →
// live value, layout.GearSettings shape) back into the update loop.
type stateRefreshMsg map[string]string

// noticeMsg carries an injection result (confirmation or error text)
// back from the off-loop inject command into the footer.
type noticeMsg string

// PersistentHooks is everything strip mode wires into the app — one
// semantic bundle so "is this a strip?" is one question (Inject != nil),
// never two drifting nil checks.
type PersistentHooks struct {
	// Inject delivers a fired selection to the target pane. Runs inside
	// a tea command, off the update loop — it may block on tmux.
	Inject func(cmd catalog.Command, arg string, insertOnly bool) error
	// Refresh polls live gear settings every stateRefreshInterval; also
	// off-loop. Nil disables the tick.
	Refresh func() map[string]string
	// Seed is the startup gear-settings snapshot — the placements were
	// built from it, so the first poll that matches it is a no-op
	// instead of a cursor-snapping re-mark.
	Seed map[string]string
}

// Model routes between the deck screen and the embedded palette.
type Model struct {
	commands []catalog.Command
	order    []layout.Placement
	styles   *theme.Styles
	focus    int
	// focusHidden suppresses the focus ring after hover leaves every
	// tile — tmux sends no pane-leave event, so off-tile motion (the
	// gutters and footer are the exit lanes) is the best signal that
	// the pointer is gone. m.focus itself stays valid for the keyboard;
	// any key, wheel, or click re-shows the ring.
	focusHidden bool

	screen  screen
	palette palette.Model
	armed   bool // inside the press frame: input is inert, the tick quits
	width   int
	height  int

	selected   *catalog.Command
	arg        string
	insertOnly bool

	// Persistent (strip) mode: hooks deliver each selection mid-loop
	// instead of quitting and keep gear state live; notice is the
	// footer's fire/error feedback; lastSettings dedups refresh applies
	// per gear so unchanged values never move a cursor. All zero in
	// popup mode.
	hooks        PersistentHooks
	notice       string
	lastSettings map[string]string

	// compact renders the chip flow instead of the 13-column grid
	// (STRIP-EMBED step 2): chips snap to a column grid, wrapping at
	// the canvas edge, no wordmark — built for the ~33-col tcm sidebar.
	compact bool
}

// Compact flips the model to the chip-flow rendering; the placements
// must already be chip tiles (layout.Compacted).
func (m Model) Compact() Model {
	m.compact = true
	return m
}

// New builds the app over a placed deck (layout.Default or, from P4, a
// parsed layout.toml). Focus order = placement order; st must be the
// same registry the placements' tiles were built with.
func New(commands []catalog.Command, placements []layout.Placement, st *theme.Styles) Model {
	return Model{commands: commands, order: placements, styles: st}
}

// Persistent flips the model into strip mode (STRIP-EMBED.md step 1):
// firing a tile delivers through h.Inject and the program keeps
// running; h.Refresh is polled every stateRefreshInterval to keep gear
// markers live. Both hooks run off the update loop (inside tea
// commands), so they may block on tmux round trips.
func (m Model) Persistent(h PersistentHooks) Model {
	m.hooks = h
	m.lastSettings = h.Seed
	return m
}

func (m Model) Init() tea.Cmd {
	return m.refreshTick()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		m.width, m.height = size.Width, size.Height
	}
	// Refresh and inject results land regardless of screen — a palette
	// detour must not stall gear state or drop the tick chain.
	switch msg := msg.(type) {
	case stateRefreshMsg:
		return m.applyRefresh(msg), m.refreshTick()
	case noticeMsg:
		m.notice = string(msg)
		return m, nil
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
	// AllMotion (SGR 1003) for hover focus — spiked 2026-07-05: tmux
	// forwards bare motion into display-popup, popup-local coords, same
	// as clicks (S1). CellMotion was P0's floor so tmux never scrolls.
	view.MouseMode = tea.MouseModeAllMotion
	m.styles.ApplySurface(&view)
	if m.width == 0 {
		return view
	}
	view.Content = m.viewDeck()
	return view
}

// updateDeck dispatches deck input by kind; each handler owns one input
// device. Keyboard policy lives here in app — it owns focus. Quit keys
// always work — during the press frame they ABORT (selection cleared,
// zero side effects), so a misclick can be cancelled and a lost tick
// can never wedge the popup; everything else is gated once, here, so a
// degraded canvas (or an empty deck) can never focus, fire, or hit an
// invisible tile.
func (m Model) updateDeck(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(pressFrameDoneMsg); ok {
		if m.hooks.Inject == nil {
			return m, tea.Quit
		}
		return m.deliver()
	}
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case "ctrl+c", "esc", "q":
			if m.armed {
				m.selected = nil
				m.arg = ""
				m.insertOnly = false
				if m.hooks.Inject != nil {
					// Persistent mode: aborting a misclick must not cost
					// the whole long-lived strip — disarm and live on.
					m.armed = false
					return m, nil
				}
			}
			// Esc closes an expanded gear chip before it means quit —
			// backing out of the value row is the smaller retreat.
			if key.String() == "esc" && m.collapseGearChips() {
				return m, nil
			}
			if key.String() == "esc" && m.hooks.Inject != nil {
				// Persistent mode: esc is too easy to hit on the way to
				// another pane, and losing the long-lived strip to a
				// stray press is disproportionate (companion QA,
				// 2026-07-06). q / ctrl+c stay the deliberate quits.
				return m, nil
			}
			return m, tea.Quit
		}
	}
	if m.armed {
		return m, nil // the press frame owns the screen; its tick fires
	}
	if len(m.order) == 0 || m.canvasTooSmall() {
		return m, nil
	}
	// Fresh input retires the previous fire's footer notice; the key
	// legend comes back (hover motion floods, so it doesn't count).
	switch msg.(type) {
	case tea.KeyPressMsg, tea.MouseClickMsg:
		m.notice = ""
		m.focusHidden = false
	case tea.MouseWheelMsg:
		m.focusHidden = false
	}
	switch msg := msg.(type) {
	case tea.MouseClickMsg:
		return m.handleDeckClick(msg)
	case tea.MouseMotionMsg:
		return m.handleDeckMotion(msg)
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
	m.collapseGearChipsExcept(i)
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
	if gc, isChip := m.order[i].Tile.(widget.GearChip); isChip {
		// Chip form: first click opens the value row; a click on a value
		// fires it and collapses (the gated column rotated 90°).
		if !gc.Expanded {
			m.order[i].Tile = gc.Expand()
			return m, nil
		}
		if v, ok := gc.ValueAtX(msg.X - hit.Bounds().Min.X); ok {
			gc = gc.WithCursor(v)
			m.order[i].Tile = gc.Collapse()
			return m.activate(gc.Activate())
		}
		return m, nil
	}
	return m.activate(m.order[i].Tile.Activate())
}

// collapseGearChips closes every expanded gear chip; true when one was
// open (Esc consumes the keypress in that case).
func (m Model) collapseGearChips() bool {
	return m.collapseGearChipsExcept(-1)
}

// collapseGearChipsExcept closes expanded gear chips other than i — a
// click or focus move elsewhere retracts an open value row.
func (m Model) collapseGearChipsExcept(i int) bool {
	closed := false
	for j, p := range m.order {
		if gc, ok := p.Tile.(widget.GearChip); ok && gc.Expanded && j != i {
			m.order[j].Tile = gc.Collapse()
			closed = true
		}
	}
	return closed
}

// handleDeckMotion is hover: the focus ring follows the pointer, and
// inside a gear the value cursor tracks the hovered row (mirroring
// click's row targeting — hover shows exactly what a click would do).
// Off-tile motion changes nothing, and an unchanged model renders an
// identical frame, so the per-cell motion flood costs no repaints.
func (m Model) handleDeckMotion(msg tea.MouseMotionMsg) (tea.Model, tea.Cmd) {
	hit := m.compositor().Hit(msg.X, msg.Y)
	i, ok := hitTile(hit)
	if !ok || i >= len(m.order) {
		m.focusHidden = true
		return m, nil
	}
	m.focusHidden = false
	m.focus = i
	if g, isGear := m.order[i].Tile.(widget.Gear); isGear {
		if v, ok := g.ValueAt(msg.Y - hit.Bounds().Min.Y); ok {
			m.order[i].Tile = g.WithCursor(v)
		}
	}
	if gc, isChip := m.order[i].Tile.(widget.GearChip); isChip && gc.Expanded {
		if v, ok := gc.ValueAtX(msg.X - hit.Bounds().Min.X); ok {
			m.order[i].Tile = gc.WithCursor(v)
		}
	}
	return m, nil
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
	// An expanded gear chip captures navigation until it collapses —
	// its value row is a one-line modal (Esc backs out, in updateDeck).
	if gc, ok := m.order[m.focus].Tile.(widget.GearChip); ok && gc.Expanded {
		switch key.String() {
		case "left", "h", "up", "k":
			m.order[m.focus].Tile = gc.CursorPrev()
			return m, nil
		case "right", "l", "down", "j":
			m.order[m.focus].Tile = gc.CursorNext()
			return m, nil
		case "enter":
			m.order[m.focus].Tile = gc.Collapse()
			return m.activate(gc.Activate())
		}
		return m, nil
	}
	switch key.String() {
	case "enter":
		if gc, ok := m.order[m.focus].Tile.(widget.GearChip); ok {
			m.order[m.focus].Tile = gc.Expand()
			return m, nil
		}
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
		if m.hooks.Inject != nil {
			// Persistent mode: deliver and land back on the deck — the
			// strip outlives every selection.
			m.screen = screenDeck
			return m.deliver()
		}
		return m, tea.Quit
	}
	return m, nil
}

// deliver fires the recorded selection through the injector and keeps
// the program alive — the point of persistent mode (the popup's
// quit-then-inject handoff can't work when there's no quit). The inject
// runs inside the returned command, off the update loop (tmux round
// trips must never freeze the UI); its result lands in the footer
// notice, the strip's only feedback surface.
func (m Model) deliver() (tea.Model, tea.Cmd) {
	m.armed = false
	if m.selected == nil {
		return m, nil
	}
	sel, arg, insertOnly := *m.selected, m.arg, m.insertOnly
	m.selected, m.arg, m.insertOnly = nil, "", false
	inject := m.hooks.Inject
	return m, func() tea.Msg {
		if err := inject(sel, arg, insertOnly); err != nil {
			return noticeMsg(err.Error())
		}
		notice := "→ /" + sel.Name
		if arg != "" {
			notice += " " + arg
		}
		return noticeMsg(notice)
	}
}

// refreshTick schedules the next gear-state poll; the poll itself runs
// inside the tick command, off the update loop. Nil without a Refresh
// hook — popup mode never ticks.
func (m Model) refreshTick() tea.Cmd {
	refresh := m.hooks.Refresh
	if refresh == nil {
		return nil
	}
	return tea.Tick(stateRefreshInterval, func(time.Time) tea.Msg {
		return stateRefreshMsg(refresh())
	})
}

// applyRefresh re-marks gears whose polled value changed since the last
// poll (Seed counts as the zeroth). Per-gear granularity: an unchanged
// gear is never touched, so a poll can't snap a mid-navigation cursor
// on a gear whose state didn't move; when a gear's value did change,
// snapping its cursor to the new current is WithCurrent's normal
// behavior.
func (m Model) applyRefresh(settings map[string]string) Model {
	for i, p := range m.order {
		switch g := p.Tile.(type) {
		case widget.Gear:
			if v, ok := settings[g.Cmd.Name]; ok && v != m.lastSettings[g.Cmd.Name] {
				m.order[i].Tile = g.WithCurrent(v)
			}
		case widget.GearChip:
			if v, ok := settings[g.Cmd.Name]; ok && v != m.lastSettings[g.Cmd.Name] {
				m.order[i].Tile = g.WithCurrent(v)
			}
		}
	}
	m.lastSettings = settings
	return m
}

// marginX is the deck's horizontal margin — the smallest step of the
// Fibonacci scale, like every other gap (vertical spacing lives with the
// placements in layout).
var marginX = deck.Scale[0]

// compositor lays every tile into a named layer over a base layer holding
// the header/footer. The same compositor renders the view AND answers
// mouse hit-tests, so clicks can never disagree with pixels.
func (m Model) compositor() *lipgloss.Compositor {
	layers, tileRows := m.gridLayers()
	hint := "h/l tiles · j/k in gear · Enter fire · / all commands · Esc close"
	if m.hooks.Inject != nil {
		// Persistent mode never quits on esc — the hint must not claim
		// otherwise.
		hint = "h/l tiles · j/k in gear · Enter fire · / all commands · q quit"
	}
	if m.compact {
		layers, tileRows = m.flowLayers()
		hint = "Enter fire · / all · q quit"
	}

	// Base layer: the wordmark — the single authored grid break (Samara):
	// pinned to line 1 as a reverse block, obeying no column boundary —
	// plus the footer hint line.
	margin := strings.Repeat(" ", marginX)
	base := make([]string, 0, tileRows+2)
	if !m.compact {
		// Compact drops the wordmark — every row belongs to chips.
		base = append(base, margin+theme.BlendForeground(" GEARSHIFTER ",
			m.styles.Chrome.Wordmark, m.styles.Chrome.WordmarkBlend))
	}
	for len(base) < tileRows+1 {
		base = append(base, "")
	}
	if m.notice != "" {
		// Persistent mode's fire/error feedback replaces the hint line —
		// the strip has no closing flash to say "it landed".
		hint = m.notice
	}
	// The footer (hint or a long tmux error) must never exceed the
	// canvas — over-wide lines clip cell-by-cell downstream, losing the
	// tail silently; truncating here keeps the cut honest.
	base = append(base, margin+m.styles.Chrome.Footer.Render(truncateCells(hint, max(0, m.width-2*marginX))))
	baseLayer := lipgloss.NewLayer(strings.Join(base, "\n")).X(0).Y(0).Z(0)

	return lipgloss.NewCompositor(append([]*lipgloss.Layer{baseLayer}, layers...)...)
}

// gridLayers places tiles by the 13-column grid (the popup deck).
func (m Model) gridLayers() ([]*lipgloss.Layer, int) {
	grid := deck.New(m.width - 2*marginX)
	tileRows := 0
	layers := make([]*lipgloss.Layer, 0, len(m.order))
	for i, p := range m.order {
		x, w := grid.Cell(p.Col, p.Tile.Span())
		rs := widget.RenderState{Focused: i == m.focus && !m.focusHidden, Armed: m.armed && i == m.focus}
		layers = append(layers, lipgloss.NewLayer(p.Tile.View(rs, w)).
			ID("tile:"+strconv.Itoa(i)).X(marginX+x).Y(p.Y).Z(1))
		if bottom := p.Y + p.Tile.Rows(); bottom > tileRows {
			tileRows = bottom
		}
	}
	return layers, tileRows
}

// flowBodyY is the first chip row: the very top — compact mode drops
// the wordmark (companion QA 2026-07-06: a strip pane has no rows to
// spare on branding).
const flowBodyY = 0

// flowLayers packs chips onto a column grid, wrapping at the canvas
// edge (STRIP-EMBED step 2). Every chip is granted the width of the
// widest one and positions snap to column starts, so labels line up
// row over row (companion QA 2026-07-06 — natural-width packing read
// ragged). Wider tiles (the launcher, an expanded gear) span multiple
// columns, truncating in place rather than overflowing the hit-test
// bounds. Gear chips own their row: expanding one must never reflow
// the chips around it (spike QA — the unfold was moving the field).
func (m Model) flowLayers() ([]*lipgloss.Layer, int) {
	maxX := m.width - marginX
	colW := m.flowColWidth()
	step := colW + 1
	x, y := marginX, flowBodyY
	layers := make([]*lipgloss.Layer, 0, len(m.order))
	for i, p := range m.order {
		_, ownRow := p.Tile.(widget.GearChip)
		w := max(colW, flowWidth(p.Tile, maxX-marginX))
		if x > marginX && (ownRow || x+w > maxX) {
			x, y = marginX, y+1
		}
		if w > maxX-x {
			w = maxX - x
		}
		rs := widget.RenderState{Focused: i == m.focus && !m.focusHidden, Armed: m.armed && i == m.focus}
		layers = append(layers, lipgloss.NewLayer(p.Tile.View(rs, w)).
			ID("tile:"+strconv.Itoa(i)).X(x).Y(y).Z(1))
		if ownRow {
			x, y = marginX, y+1
		} else {
			x += step * ((w + step) / step) // next column boundary past this tile
		}
	}
	return layers, y + 1
}

// flowColWidth is the flow's column unit: the widest collapsed chip.
// The launcher is excluded — its long label would force one-chip-per-
// row columns, so it spans columns instead of defining them.
func (m Model) flowColWidth() int {
	w := 0
	for _, p := range m.order {
		var nw int
		switch t := p.Tile.(type) {
		case widget.LauncherChip:
			continue
		case widget.GearChip:
			nw = t.Collapse().NaturalWidth()
		case widget.FlowTile:
			nw = t.NaturalWidth()
		}
		if nw > w {
			w = nw
		}
	}
	return w
}

// flowWidth is a tile's packed width: natural for chips, the full row
// for anything else, clamped to the canvas.
func flowWidth(t widget.Tile, maxW int) int {
	w := maxW
	if ft, ok := t.(widget.FlowTile); ok {
		w = ft.NaturalWidth()
	}
	return min(w, maxW)
}

// truncateCells cuts plain text to at most w display cells (style is
// applied after, one style per row — M2 gotcha).
func truncateCells(s string, w int) string {
	if lipgloss.Width(s) <= w {
		return s
	}
	var b strings.Builder
	x := 0
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if x+rw > w {
			break
		}
		b.WriteRune(r)
		x += rw
	}
	return b.String()
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
	if m.compact {
		return m.flowMinCanvas()
	}
	for _, p := range m.order {
		if bottom := p.Y + p.Tile.Rows(); bottom > h {
			h = bottom
		}
	}
	return 2*marginX + deck.MinWidth(), h
}

// flowMinCanvas: the widest COLLAPSED chip must fit one row (an
// expanded gear clamps instead of raising the floor), and every flowed
// row plus the footer must be on screen at the current width.
func (m Model) flowMinCanvas() (w, h int) {
	for _, p := range m.order {
		var nw int
		switch t := p.Tile.(type) {
		case widget.GearChip:
			nw = t.Collapse().NaturalWidth()
		case widget.FlowTile:
			nw = t.NaturalWidth()
		}
		if nw > w {
			w = nw
		}
	}
	_, bottom := m.flowLayers()
	return w + 2*marginX, bottom + 1
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

package app

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/kylesnowschwartz/gearshifter/internal/agent"
	"github.com/kylesnowschwartz/gearshifter/internal/catalog"
	"github.com/kylesnowschwartz/gearshifter/internal/deck"
	"github.com/kylesnowschwartz/gearshifter/internal/layout"
	"github.com/kylesnowschwartz/gearshifter/internal/theme"
	"github.com/kylesnowschwartz/gearshifter/internal/widget"
)

var testStyles = theme.Plain()

func testCommands() []catalog.Command {
	return []catalog.Command{
		{Name: "btw", ArgumentHint: "<question>"},
		{Name: "compact"},
		{Name: "context"},
		{Name: "resume"},
		{Name: "review"},
	}
}

func newTestModel() Model {
	// haiku/low: cursors start on current (index 0), keeping key-walk
	// expectations simple.
	cmds := testCommands()
	m := New(cmds, layout.Default(cmds, agent.State{Model: "haiku", Effort: "low"}, testStyles, layout.SortNone), testStyles)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 82, Height: 26})
	return updated.(Model)
}

func press(m Model, keys ...string) Model {
	for _, k := range keys {
		var key tea.Key
		switch k {
		case "enter":
			key = tea.Key{Code: tea.KeyEnter}
		case "esc":
			key = tea.Key{Code: tea.KeyEscape}
		default:
			key = tea.Key{Code: rune(k[0]), Text: k}
		}
		updated, _ := m.Update(tea.KeyPressMsg(key))
		m = updated.(Model)
	}
	return m
}

// Focus order (4×4 default): 0 MODEL, 1 EFFORT, 2 COMPACT, 3 COPY,
// 4 CLEAR, 5 CONTEXT, 6 RESUME, 7 CONFIG, 8 AGENTS, 9 MEMORY, 10 COST,
// 11 DOCTOR, 12 EXPORT, 13 STATUS, 14 HOOKS, 15 MCP, 16 PERMS,
// 17 RELOAD, 18 launcher.

func TestFocusWalksAndEnterFires(t *testing.T) {
	m := press(newTestModel(), "l", "l", "enter") // MODEL → EFFORT → COMPACT
	sel, ok := m.Selection()
	if !ok || sel.Name != "compact" {
		t.Errorf("Selection() = %v %v, want compact", sel.Name, ok)
	}
	if m.InsertOnly() || m.Arg() != "" {
		t.Error("deck button fire must be plain: no insert-only, no arg")
	}
}

// Firing a tile enters the press frame: the selection is recorded
// immediately, input goes inert for the flash, and the frame's tick —
// not the keypress — quits the program (P2).
func TestPressFrameArmsThenQuits(t *testing.T) {
	m := press(newTestModel(), "l", "l", "enter") // fire COMPACT
	if !m.armed {
		t.Fatal("firing a tile must enter the press frame")
	}
	m = press(m, "l", "enter") // non-quit input is inert inside the frame
	if !m.armed || m.focus != 2 {
		t.Error("non-quit input during the press frame must be swallowed")
	}
	updated, cmd := m.Update(pressFrameDoneMsg{})
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("the press-frame tick must quit")
	}
	if sel, ok := m.Selection(); !ok || sel.Name != "compact" {
		t.Errorf("selection must survive the frame, got %v %v", sel.Name, ok)
	}
}

// Quit keys always work — during the press frame they abort: the
// recorded selection is cleared so a misclick cancels with zero side
// effects, and a lost tick can never wedge the popup (review finding).
func TestPressFrameQuitKeysAbort(t *testing.T) {
	m := press(newTestModel(), "l", "l", "enter") // fire COMPACT, armed
	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("esc during the press frame must quit")
	}
	if _, ok := m.Selection(); ok {
		t.Error("aborting the press frame must clear the selection")
	}
	if m.Arg() != "" || m.InsertOnly() {
		t.Error("abort must clear every selection modifier")
	}
}

func TestFocusWraps(t *testing.T) {
	m := press(newTestModel(), "h", "enter") // wrap back to the launcher bar
	if m.screen != screenPalette {
		t.Error("h from MODEL must wrap to the launcher; Enter opens palette")
	}
}

func TestGearCursorAndCommit(t *testing.T) {
	m := press(newTestModel(), "j", "j", "enter") // MODEL: haiku → sonnet → opus
	sel, ok := m.Selection()
	if !ok || sel.Name != "model" || m.Arg() != "opus" {
		t.Errorf("gear commit = %v arg %q, want model opus", sel.Name, m.Arg())
	}
}

func TestGearCursorWraps(t *testing.T) {
	m := press(newTestModel(), "k", "enter") // k from haiku wraps to fable
	if sel, _ := m.Selection(); m.Arg() != "fable" || sel.Name != "model" {
		t.Errorf("gear k-wrap committed %q, want fable", m.Arg())
	}
}

func TestLauncherOpensPaletteAndSelectionBubbles(t *testing.T) {
	m := press(newTestModel(), "h", "enter") // wrap to launcher, open
	if m.screen != screenPalette {
		t.Fatal("launcher must open the palette screen")
	}
	m = press(m, "c", "t", "x") // palette filter — ctx matches context
	m = press(m, "enter")
	sel, ok := m.Selection()
	if !ok || sel.Name != "context" {
		t.Errorf("palette selection = %v %v, want context", sel.Name, ok)
	}
}

func TestSlashOpensPaletteEscReturnsToDeck(t *testing.T) {
	m := press(newTestModel(), "/")
	if m.screen != screenPalette {
		t.Fatal("/ must open the palette")
	}
	m = press(m, "esc")
	if m.screen != screenDeck {
		t.Error("esc in embedded palette must return to deck, not quit")
	}
	if _, ok := m.Selection(); ok {
		t.Error("esc must not select")
	}
}

func TestEscOnDeckQuitsWithoutSelection(t *testing.T) {
	m := press(newTestModel(), "esc")
	if _, ok := m.Selection(); ok {
		t.Error("esc on deck must not select")
	}
}

func TestClickFiresButton(t *testing.T) {
	m := newTestModel()
	// COMPACT sits at grid col 5 span 4, body row 0 — compute the same
	// geometry the app does and click inside it.
	g := deck.New(82 - 2)
	x, _ := g.Cell(deck.RailSpan, 4)
	updated, _ := m.Update(tea.MouseClickMsg{X: 1 + x + 2, Y: 3, Button: tea.MouseLeft})
	m = updated.(Model)
	sel, ok := m.Selection()
	if !ok || sel.Name != "compact" {
		t.Errorf("click on COMPACT selected %v %v, want compact", sel.Name, ok)
	}
	if m.InsertOnly() {
		t.Error("a plain button click must not request insert-only")
	}
}

func TestClickGearValueCommitsIt(t *testing.T) {
	m := newTestModel()
	// MODEL gear top border at y=2; value rows follow — y=4 is sonnet.
	updated, _ := m.Update(tea.MouseClickMsg{X: 3, Y: 4, Button: tea.MouseLeft})
	m = updated.(Model)
	sel, ok := m.Selection()
	if !ok || sel.Name != "model" || m.Arg() != "sonnet" {
		t.Errorf("click gear row = %v arg %q, want model sonnet", sel.Name, m.Arg())
	}
}

func TestClickGearBorderOnlyFocuses(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(tea.MouseClickMsg{X: 3, Y: 2, Button: tea.MouseLeft}) // top border
	m = updated.(Model)
	if _, ok := m.Selection(); ok {
		t.Error("border click must not commit")
	}
	if m.focus != 0 {
		t.Errorf("border click focus = %d, want 0 (MODEL)", m.focus)
	}
}

func TestClickLauncherOpensPalette(t *testing.T) {
	m := newTestModel()
	// Launcher bar: the 2×3 button field bottoms at 16 (> rail 15), so the
	// skyline drops the launcher to y17; click its content row.
	updated, _ := m.Update(tea.MouseClickMsg{X: 10, Y: 18, Button: tea.MouseLeft})
	m = updated.(Model)
	if m.screen != screenPalette {
		t.Error("click on launcher bar must open the palette")
	}
}

func TestClickDeadSpaceIgnored(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(tea.MouseClickMsg{X: 0, Y: 0, Button: tea.MouseLeft})
	m = updated.(Model)
	if _, ok := m.Selection(); ok || m.screen != screenDeck {
		t.Error("click outside tiles must be a no-op")
	}
}

func TestWheelMovesFocus(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	m = updated.(Model)
	if m.focus != 1 {
		t.Errorf("wheel down: focus = %d, want 1", m.focus)
	}
}

func TestDeckViewMarksCurrentGearValues(t *testing.T) {
	content := newTestModel().View().Content
	if !strings.Contains(content, "▐ haiku") || !strings.Contains(content, "▐ low") {
		t.Error("deck must mark the live model/effort from GearState")
	}
}

func TestTinyCanvasDegradesToMessageAndInertInput(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 30, Height: 10})
	m = updated.(Model)
	content := m.View().Content
	if !strings.Contains(content, "too small") || strings.Contains(content, "MODEL") {
		t.Errorf("tiny canvas must show the too-small message, no tiles:\n%s", content)
	}
	updated, _ = m.Update(tea.MouseClickMsg{X: 3, Y: 4, Button: tea.MouseLeft})
	m = updated.(Model)
	if _, ok := m.Selection(); ok {
		t.Error("clicks on a degraded canvas must be inert")
	}
	m = press(m, "enter")
	if _, ok := m.Selection(); ok || m.screen != screenDeck {
		t.Error("enter on a degraded canvas must not fire an invisible tile")
	}
	updated, _ = m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	m = updated.(Model)
	if m.focus != 0 {
		t.Error("wheel on a degraded canvas must not move focus")
	}
}

func TestEmptyDeckNeverPanics(t *testing.T) {
	m := New(nil, nil, testStyles) // no placements: inert but alive, quit keys work
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 82, Height: 26})
	m = updated.(Model)
	m = press(m, "l", "j", "enter")
	updated, _ = m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	m = updated.(Model)
	if _, ok := m.Selection(); ok {
		t.Error("empty deck must select nothing")
	}
}

// Persistent (strip) mode tests — STRIP-EMBED.md step 1. The riskiest
// assumption: the program must survive every fire, delivering mid-loop
// through the injector instead of quitting.

type firedCall struct {
	name       string
	arg        string
	insertOnly bool
	fromMouse  bool
}

func persistentModel(fired *[]firedCall) Model {
	return newTestModel().Persistent(PersistentHooks{
		Inject: func(f Fire) error {
			*fired = append(*fired, firedCall{f.Cmd.Name, f.Arg, f.InsertOnly, f.FromMouse})
			return nil
		},
	})
}

// completeFire ends the press frame and runs the returned inject
// command by hand (tests have no Bubble Tea runner), feeding its notice
// back into the model — asserting along the way that the fire never
// quits the program.
func completeFire(t *testing.T, m Model) Model {
	t.Helper()
	updated, cmd := m.Update(pressFrameDoneMsg{})
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("a persistent fire must return the inject command")
	}
	msg := cmd()
	if _, quit := msg.(tea.QuitMsg); quit {
		t.Fatal("a persistent fire must not quit")
	}
	updated, _ = m.Update(msg)
	return updated.(Model)
}

func TestPersistentFireDeliversAndStaysAlive(t *testing.T) {
	var fired []firedCall
	m := persistentModel(&fired)

	m = press(m, "l", "l", "enter") // fire COMPACT, armed
	m = completeFire(t, m)
	if len(fired) != 1 || fired[0].name != "compact" {
		t.Fatalf("fired = %+v, want one compact delivery", fired)
	}
	if _, ok := m.Selection(); ok || m.armed {
		t.Error("delivery must clear the selection and disarm")
	}
	if !strings.Contains(m.View().Content, "→ /compact") {
		t.Error("the footer must confirm the fire")
	}

	// The strip outlives the fire: a second tile fires again, and the
	// fresh input retires the previous notice.
	m = press(m, "l", "enter") // COPY
	m = completeFire(t, m)
	if len(fired) != 2 || fired[1].name != "copy" {
		t.Errorf("second fire = %+v, want copy appended", fired)
	}
}

func TestPersistentGearFireCarriesValue(t *testing.T) {
	var fired []firedCall
	m := press(persistentModel(&fired), "j", "enter") // MODEL: haiku → sonnet
	completeFire(t, m)
	if len(fired) != 1 || fired[0].name != "model" || fired[0].arg != "sonnet" {
		t.Errorf("gear fire = %+v, want model/sonnet", fired)
	}
}

// Quit keys during the armed frame abort the fire WITHOUT killing the
// strip (a misclick must never cost the whole widget); unarmed q still
// quits deliberately.
func TestPersistentArmedAbortStaysAlive(t *testing.T) {
	var fired []firedCall
	m := press(persistentModel(&fired), "l", "l", "enter") // armed on COMPACT
	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	m = updated.(Model)
	if cmd != nil {
		t.Error("armed abort must not quit the strip")
	}
	if m.armed {
		t.Error("abort must disarm")
	}
	if _, ok := m.Selection(); ok {
		t.Error("abort must clear the selection")
	}
	// A late press-frame tick after the abort is a no-op fire.
	updated, _ = m.Update(pressFrameDoneMsg{})
	m = updated.(Model)
	if len(fired) != 0 {
		t.Errorf("aborted fire must not inject, got %+v", fired)
	}
	if _, cmd = m.Update(tea.KeyPressMsg(tea.Key{Code: 'q', Text: "q"})); cmd == nil {
		t.Error("unarmed q must still quit the strip")
	}
}

func TestPersistentInjectErrorLandsInFooter(t *testing.T) {
	m := newTestModel().Persistent(PersistentHooks{
		Inject: func(Fire) error {
			return fmt.Errorf("no Claude pane in this window")
		},
	})
	m = press(m, "l", "l", "enter")
	m = completeFire(t, m)
	if !strings.Contains(m.View().Content, "no Claude pane in this window") {
		t.Error("an inject failure must fail with words in the footer")
	}
}

// tileCenter finds order-index i's on-screen position via the same
// layers the compositor renders — tests click what pixels show.
func tileCenter(t *testing.T, m Model, i int) (x, y int) {
	t.Helper()
	layers, _ := m.gridLayers()
	if m.compact {
		layers, _ = m.flowLayers()
	}
	for _, l := range layers {
		if l.GetID() == "tile:"+strconv.Itoa(i) {
			return l.GetX() + 1, l.GetY()
		}
	}
	t.Fatalf("tile %d not placed", i)
	return 0, 0
}

// FromMouse rides the Fire: only click-fires ask the injector to hand
// tmux focus back — a keyboard fire must not yank the user out of the
// strip they deliberately focused (review 2026-07-06).
func TestFireCarriesInputKind(t *testing.T) {
	var fired []firedCall
	m := persistentModel(&fired)
	m = press(m, "l", "l", "enter") // keyboard fire
	m = completeFire(t, m)
	if len(fired) != 1 || fired[0].fromMouse {
		t.Fatalf("keyboard fire = %+v, want fromMouse=false", fired)
	}
	// Click the same tile: focus 2 is COMPACT at its placement.
	x, y := tileCenter(t, m, 2)
	updated, _ := m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	m = completeFire(t, updated.(Model))
	if len(fired) != 2 || !fired[1].fromMouse {
		t.Fatalf("click fire = %+v, want fromMouse=true", fired)
	}
}

// Error notices are sticky — the footer is the strip's only error
// surface, and a silently expiring error reads as success (review
// 2026-07-06). Confirmations keep the 5s TTL.
func TestErrorNoticePersistsPastTTL(t *testing.T) {
	var fired []firedCall
	m := persistentModel(&fired)
	updated, cmd := m.Update(noticeMsg{text: "no Claude pane in this window", isErr: true})
	m = updated.(Model)
	if cmd != nil {
		t.Error("an error notice must not arm an expiry timer")
	}
	// Even a stray expiry from an older confirmation can't clear it.
	updated, _ = m.Update(noticeExpireMsg{gen: m.noticeGen - 1})
	m = updated.(Model)
	if !strings.Contains(m.View().Content, "no Claude pane") {
		t.Error("the error must persist until fresh input")
	}
	m = press(m, "l")
	if strings.Contains(m.View().Content, "no Claude pane") {
		t.Error("fresh input must clear the error")
	}
}

func TestPersistentPaletteSelectionDeliversAndReturnsToDeck(t *testing.T) {
	var fired []firedCall
	m := press(persistentModel(&fired), "/", "c", "t", "x")
	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	m = updated.(Model)
	if m.screen != screenDeck {
		t.Error("palette delivery must land back on the deck")
	}
	if cmd == nil {
		t.Fatal("palette delivery must return the inject command")
	}
	msg := cmd()
	if _, quit := msg.(tea.QuitMsg); quit {
		t.Fatal("palette delivery must not quit")
	}
	if len(fired) != 1 || fired[0].name != "context" {
		t.Errorf("palette fire = %+v, want context", fired)
	}
	if _, ok := m.Selection(); ok {
		t.Error("delivery must clear the selection")
	}
}

func TestPersistentRefreshRemarksGears(t *testing.T) {
	seed := map[string]string{"model": "haiku", "effort": "low"} // = newTestModel's state
	m := newTestModel().Persistent(PersistentHooks{
		Inject:  func(Fire) error { return nil },
		Refresh: func() map[string]string { return nil },
		Seed:    seed,
	})
	if m.Init() == nil {
		t.Fatal("persistent mode must schedule the refresh tick")
	}
	if newTestModel().Init() != nil {
		t.Fatal("popup mode must not tick")
	}

	// The FIRST poll matching the startup seed is a per-gear no-op — it
	// must not snap a mid-navigation cursor (review finding: unseeded
	// lastSettings made the first tick re-mark every gear).
	m = press(m, "j") // MODEL cursor: haiku → sonnet
	before := m.View().Content
	updated, cmd := m.Update(stateRefreshMsg{"model": "haiku", "effort": "low"})
	m = updated.(Model)
	if cmd == nil {
		t.Error("a refresh must schedule the next tick")
	}
	if m.View().Content != before {
		t.Error("a poll matching the seed must not touch the deck")
	}

	// A changed value re-marks its own gear only.
	updated, _ = m.Update(stateRefreshMsg{"model": "haiku", "effort": "high"})
	m = updated.(Model)
	content := m.View().Content
	if !strings.Contains(content, "▐ high") {
		t.Errorf("refresh must re-mark the changed gear:\n%s", content)
	}
	if !strings.Contains(content, "▐ haiku") {
		t.Error("the unchanged gear must keep its current marker")
	}
}

func TestDeckViewRendersTiles(t *testing.T) {
	content := newTestModel().View().Content
	// "/copy" not "/compact": at the 82-cell test canvas a span-2 tile's
	// inner is ~9 cells, so longer nameplates truncate by design.
	for _, want := range []string{"GEARSHIFTER", "MODEL", "sonnet", "EFFORT", "max", "ALL COMMANDS…", "COMPACT", "/copy", "COPY", "CLEAR", "CONTEXT", "RESUME", "CONFIG", "PERMS", "RELOAD"} {
		if !strings.Contains(content, want) {
			t.Errorf("deck view missing %q", want)
		}
	}
}

// Compact (chip-flow) mode — STRIP-EMBED step 2 spike. 33 cols = the
// tcm sidebar default width the flow is built for.

func compactModel(fired *[]firedCall) Model {
	state := agent.State{Model: "haiku", Effort: "low"}
	cmds := testCommands()
	placements := layout.Compacted(layout.Default(cmds, state, testStyles, layout.SortNone), state, testStyles)
	m := New(cmds, placements, testStyles).Persistent(PersistentHooks{
		Inject: func(f Fire) error {
			*fired = append(*fired, firedCall{f.Cmd.Name, f.Arg, f.InsertOnly, f.FromMouse})
			return nil
		},
		Seed: map[string]string{"model": "haiku", "effort": "low"},
	}).Compact()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 33, Height: 20})
	return updated.(Model)
}

func TestCompactFlowRendersWithin33Cols(t *testing.T) {
	var fired []firedCall
	content := compactModel(&fired).View().Content
	for _, want := range []string{"M:haiku", "E:low", "⊟ COMPACT", "⧉ COPY", "ALL COMMANDS"} {
		if !strings.Contains(content, want) {
			t.Errorf("compact view missing %q:\n%s", want, content)
		}
	}
	if strings.Contains(content, "GEARSHIFTER") {
		t.Error("compact view must drop the wordmark — every row belongs to chips")
	}
	for i, line := range strings.Split(content, "\n") {
		if w := lipgloss.Width(line); w > 33 {
			t.Errorf("line %d is %d cells — the flow must wrap inside the canvas:\n%q", i, w, line)
		}
	}
}

func TestCompactGearChipExpandNavigateFire(t *testing.T) {
	var fired []firedCall
	m := compactModel(&fired)

	// MODEL is the first chip: top-left of the flow (row 0 — compact
	// has no wordmark). Click expands.
	updated, _ := m.Update(tea.MouseClickMsg{X: 2, Y: 0, Button: tea.MouseLeft})
	m = updated.(Model)
	gc, ok := m.order[0].Tile.(widget.GearChip)
	if !ok || !gc.Expanded {
		t.Fatalf("click on the gear badge must expand it (got %T expanded=%v)", m.order[0].Tile, gc.Expanded)
	}
	if !strings.Contains(m.View().Content, "sonnet") {
		t.Error("the expanded row must show every value")
	}

	// The expanded row captures navigation: l moves the cursor, Enter
	// fires and collapses.
	m = press(m, "l", "enter") // haiku → sonnet, fire
	m = completeFire(t, m)
	if len(fired) != 1 || fired[0].name != "model" || fired[0].arg != "sonnet" {
		t.Fatalf("fired = %+v, want model/sonnet", fired)
	}
	if gc := m.order[0].Tile.(widget.GearChip); gc.Expanded {
		t.Error("firing must collapse the value row")
	}
}

func TestCompactEscCollapsesAndNeverQuits(t *testing.T) {
	var fired []firedCall
	m := compactModel(&fired)
	m = press(m, "enter") // expand MODEL (focus starts there)
	if gc := m.order[0].Tile.(widget.GearChip); !gc.Expanded {
		t.Fatal("enter on a collapsed gear chip must expand it")
	}
	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	m = updated.(Model)
	if cmd != nil {
		t.Error("esc with an open value row must collapse, not quit")
	}
	if gc := m.order[0].Tile.(widget.GearChip); gc.Expanded {
		t.Error("esc must collapse the row")
	}
	// Persistent mode: a stray esc must never cost the long-lived strip
	// (companion QA 2026-07-06) — q stays the deliberate quit.
	if _, cmd = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape})); cmd != nil {
		t.Error("esc with nothing open must be a no-op in persistent mode")
	}
	if _, cmd = m.Update(tea.KeyPressMsg(tea.Key{Text: "q"})); cmd == nil {
		t.Error("q must still quit the persistent strip")
	}
}

// A footer notice retires itself after noticeTTL; a newer notice must
// survive an older notice's expiry timer (companion QA 2026-07-06 —
// the lingering "→ /cmd" read as ghost text).
func TestCompactNoticeAutoExpires(t *testing.T) {
	var fired []firedCall
	m := compactModel(&fired)
	updated, cmd := m.Update(noticeMsg{text: "→ /memory"})
	m = updated.(Model)
	if m.notice != "→ /memory" || cmd == nil {
		t.Fatalf("notice = %q (cmd nil=%v), want the text and an expiry timer", m.notice, cmd == nil)
	}
	updated, _ = m.Update(noticeMsg{text: "→ /compact"}) // supersedes; gen bumps
	m = updated.(Model)
	updated, _ = m.Update(noticeExpireMsg{gen: 1}) // the FIRST notice's timer
	m = updated.(Model)
	if m.notice != "→ /compact" {
		t.Errorf("a stale timer wiped a fresh notice: %q", m.notice)
	}
	updated, _ = m.Update(noticeExpireMsg{gen: 2})
	m = updated.(Model)
	if m.notice != "" {
		t.Errorf("the matching timer must clear the notice, got %q", m.notice)
	}
}

// Hover-away retracts an open value row exactly like click-away —
// otherwise the one-line modal strands keyboard-orphaned and a second
// row can open beside it (review 2026-07-06).
func TestCompactHoverAwayCollapsesGearChip(t *testing.T) {
	var fired []firedCall
	m := compactModel(&fired)
	m = press(m, "enter") // expand MODEL (focus starts there)
	x, y := tileCenter(t, m, 1)
	updated, _ := m.Update(tea.MouseMotionMsg{X: x, Y: y}) // hover EFFORT
	m = updated.(Model)
	if gc := m.order[0].Tile.(widget.GearChip); gc.Expanded {
		t.Error("hovering another tile must collapse the open value row")
	}
	if m.focus != 1 {
		t.Errorf("hover must still move focus, got %d", m.focus)
	}
}

// The flow's canvas floor (review 2026-07-06): the launcher never
// raises the width floor — it truncates in place — and the footer row
// is INSIDE the minimum height, not one past it.
func TestCompactMinCanvasExcludesLauncherAndKeepsFooter(t *testing.T) {
	var fired []firedCall
	m := compactModel(&fired)
	wMin, hMin := m.flowMinCanvas()
	colW := m.flowColWidth()
	if wMin != colW+2*marginX {
		t.Errorf("min width %d, want column unit %d + margins (launcher must not raise it)", wMin, colW)
	}
	// At exactly the minimum height, the footer must be on screen.
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 33, Height: hMin})
	m = updated.(Model)
	lines := strings.Split(m.View().Content, "\n")
	footer := -1
	for i, l := range lines {
		if strings.Contains(l, "q quit") {
			footer = i
		}
	}
	if footer < 0 || footer > hMin-1 {
		t.Errorf("footer at row %d with min height %d — it would be clipped", footer, hMin)
	}
}

// Hover leaving every tile hides the focus ring — tmux sends no
// pane-leave event, so off-tile motion (gutters, footer) is the best
// exit signal (companion QA 2026-07-06). Any key re-shows it.
func TestCompactHoverRingClearsOffTiles(t *testing.T) {
	var fired []firedCall
	m := compactModel(&fired)
	updated, _ := m.Update(tea.MouseMotionMsg{X: 2, Y: 0})
	m = updated.(Model)
	if m.focusHidden {
		t.Fatal("motion over a chip must show the ring")
	}
	updated, _ = m.Update(tea.MouseMotionMsg{X: 2, Y: 18}) // footer void
	m = updated.(Model)
	if !m.focusHidden {
		t.Error("motion off every tile must hide the ring")
	}
	if m = press(m, "l"); m.focusHidden {
		t.Error("a keypress must re-show the ring")
	}
}

// Chips snap to a column grid so labels line up row over row
// (companion QA 2026-07-06). The launcher spans columns instead of
// defining them; gear rows use the same origin.
func TestCompactChipsSnapToColumns(t *testing.T) {
	var fired []firedCall
	m := compactModel(&fired)
	step := m.flowColWidth() + 1
	layers, _ := m.flowLayers()
	for _, l := range layers {
		if off := l.GetX() - marginX; off%step != 0 {
			t.Errorf("layer at x=%d is off the column grid (step %d)", l.GetX(), step)
		}
	}
}

func TestCompactRefreshRemarksGearChips(t *testing.T) {
	var fired []firedCall
	m := compactModel(&fired)
	updated, _ := m.Update(stateRefreshMsg{"model": "claude-fable-5", "effort": "low"})
	m = updated.(Model)
	if !strings.Contains(m.View().Content, "M:fable") {
		t.Errorf("a refresh must re-mark the gear chip badge:\n%s", m.View().Content)
	}
}

// Expanding a gear chip must not move any other chip: gears own their
// row (spike QA feedback — the in-place unfold was reflowing the field).
func TestCompactGearExpansionNeverReflowsChips(t *testing.T) {
	var fired []firedCall
	m := compactModel(&fired)
	before := strings.Split(m.View().Content, "\n")
	updated, _ := m.Update(tea.MouseClickMsg{X: 2, Y: 0, Button: tea.MouseLeft}) // expand MODEL
	m = updated.(Model)
	after := strings.Split(m.View().Content, "\n")
	if len(before) != len(after) {
		t.Fatalf("expansion changed the row count: %d → %d", len(before), len(after))
	}
	for i := range before {
		if i == 0 {
			continue // MODEL's own row is the one allowed to change
		}
		if before[i] != after[i] {
			t.Errorf("row %d moved on expansion:\nbefore %q\nafter  %q", i, before[i], after[i])
		}
	}
}

// Mascot (strip step 4 / M5 P3): clawd fills the base rows left between
// the tile field and the footer, retracts before it crowds, and is
// suppressible; the compact footer's mascot is the opt-in font glyph.

func mascotModel(t *testing.T, height int) Model {
	t.Helper()
	styles, err := theme.Load("default")
	if err != nil {
		t.Fatal(err)
	}
	cmds := testCommands()
	m := New(cmds, layout.Default(cmds, agent.State{}, styles, layout.SortNone), styles)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 82, Height: height})
	return updated.(Model)
}

func TestMascotFillsSpareRowsAndRetracts(t *testing.T) {
	_, tileRows := mascotModel(t, 60).minCanvas()

	full := mascotModel(t, tileRows+8).View().Content
	if !strings.Contains(full, "█") {
		t.Error("spare rows for the 5-row clawd must render it solid")
	}
	mini := mascotModel(t, tileRows+5).View().Content
	if strings.Contains(mini, "█") || !strings.Contains(mini, "▀") {
		t.Error("3 spare rows must degrade to the half-block mini")
	}
	none := mascotModel(t, tileRows+2).View().Content
	if strings.Contains(none, "█") || strings.Contains(none, "▀") {
		t.Error("without spare rows the mascot must retract entirely")
	}
	off := mascotModel(t, tileRows+8).WithoutMascot().View().Content
	if strings.Contains(off, "█") || strings.Contains(off, "▀") {
		t.Error("WithoutMascot must suppress the sprite regardless of room")
	}
}

func TestCompactFooterGlyphIsOptIn(t *testing.T) {
	var fired []firedCall
	m := compactModel(&fired)
	if strings.Contains(m.View().Content, theme.ClawdGlyph) {
		t.Error("the footer glyph needs Clawd.ttf — it must be opt-in")
	}
	content := m.WithMascotGlyph().View().Content
	if !strings.Contains(content, theme.ClawdGlyph) {
		t.Error("WithMascotGlyph must render the clawd glyph in the footer")
	}
	for i, line := range strings.Split(content, "\n") {
		if w := lipgloss.Width(line); w > 33 {
			t.Errorf("line %d is %d cells — the glyph must not overflow the canvas:\n%q", i, w, line)
		}
	}
}

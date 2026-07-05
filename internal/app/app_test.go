package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/kylesnowschwartz/gearshifter/internal/agent"
	"github.com/kylesnowschwartz/gearshifter/internal/catalog"
	"github.com/kylesnowschwartz/gearshifter/internal/deck"
	"github.com/kylesnowschwartz/gearshifter/internal/layout"
	"github.com/kylesnowschwartz/gearshifter/internal/theme"
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
	m := New(cmds, layout.Default(cmds, agent.State{Model: "haiku", Effort: "low"}, testStyles), testStyles)
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

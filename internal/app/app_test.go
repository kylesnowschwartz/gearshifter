package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/kylesnowschwartz/gearshifter/internal/catalog"
)

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
	m := New(testCommands())
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 82, Height: 20})
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

func TestFocusWalksAndEnterFires(t *testing.T) {
	m := press(newTestModel(), "l", "enter") // launcher → REVIEW → fire
	sel, ok := m.Selection()
	if !ok || sel.Name != "review" {
		t.Errorf("Selection() = %v %v, want review", sel.Name, ok)
	}
	if m.InsertOnly() {
		t.Error("deck button fire must not be insert-only")
	}
}

func TestFocusWraps(t *testing.T) {
	m := press(newTestModel(), "h", "enter") // wrap back to RESUME
	sel, ok := m.Selection()
	if !ok || sel.Name != "resume" {
		t.Errorf("Selection() = %v %v, want resume (wrapped)", sel.Name, ok)
	}
}

func TestLauncherOpensPaletteAndSelectionBubbles(t *testing.T) {
	m := press(newTestModel(), "enter") // launcher focused first
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

func TestDeckViewRendersTiles(t *testing.T) {
	content := newTestModel().View().Content
	for _, want := range []string{"GEARSHIFTER", "ALL COMMANDS…", "REVIEW", "/review", "CONTEXT", "COMPACT", "RESUME"} {
		if !strings.Contains(content, want) {
			t.Errorf("deck view missing %q", want)
		}
	}
}

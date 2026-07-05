package palette

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/kylesnowschwartz/gearshifter/internal/catalog"
	"github.com/kylesnowschwartz/gearshifter/internal/theme"
)

func newTestModel() Model {
	m := New(testCommands(), theme.Plain()) // fixtures shared with filter_test.go
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 10})
	return updated.(Model)
}

func press(m Model, keys ...tea.Key) Model {
	for _, k := range keys {
		updated, _ := m.Update(tea.KeyPressMsg(k))
		m = updated.(Model)
	}
	return m
}

func typeText(m Model, s string) Model {
	for _, r := range s {
		m = press(m, tea.Key{Code: r, Text: string(r)})
	}
	return m
}

func TestTypingFiltersAndPromptEchoes(t *testing.T) {
	m := typeText(newTestModel(), "rev")
	content := m.View().Content
	if !strings.Contains(content, "> rev") {
		t.Error("prompt line should echo the query")
	}
	if len(m.visible) != 2 {
		t.Errorf("query rev should match 2 commands, got %d", len(m.visible))
	}
}

func TestEnterSelectsHighlighted(t *testing.T) {
	m := typeText(newTestModel(), "ctx")
	m = press(m, tea.Key{Code: tea.KeyEnter})
	sel, ok := m.Selection()
	if !ok || sel.Name != "context" {
		t.Errorf("Selection() = %v %v, want context", sel.Name, ok)
	}
	if m.InsertOnly() {
		t.Error("Enter selection must not be insert-only")
	}
}

func TestTabSelectsInsertOnly(t *testing.T) {
	m := typeText(newTestModel(), "ctx")
	m = press(m, tea.Key{Code: tea.KeyTab})
	if _, ok := m.Selection(); !ok {
		t.Fatal("Tab should select")
	}
	if !m.InsertOnly() {
		t.Error("Tab selection must be insert-only")
	}
}

func TestEscCancelsWithoutSelection(t *testing.T) {
	m := press(newTestModel(), tea.Key{Code: tea.KeyEscape})
	if _, ok := m.Selection(); ok {
		t.Error("Esc must not select")
	}
}

func TestVimKeysNavigateOnEmptyQueryAndFilterOtherwise(t *testing.T) {
	m := press(newTestModel(), tea.Key{Code: 'j', Text: "j"}, tea.Key{Code: 'j', Text: "j"}, tea.Key{Code: 'k', Text: "k"})
	if m.cursor != 1 {
		t.Errorf("j j k on empty query: cursor = %d, want 1", m.cursor)
	}
	if m.query != "" {
		t.Errorf("vim keys must not enter the query, got %q", m.query)
	}
	m = typeText(m, "re")
	m = press(m, tea.Key{Code: 'j', Text: "j"})
	if m.query != "rej" {
		t.Errorf("with active query, j should filter: query = %q, want rej", m.query)
	}
}

func TestClickSelectsRow(t *testing.T) {
	m := newTestModel()
	// y=0 prompt, y=1 first row; click the third row
	updated, _ := m.Update(tea.MouseClickMsg{X: 2, Y: 3, Button: tea.MouseLeft})
	m = updated.(Model)
	sel, ok := m.Selection()
	if !ok || sel.Name != "context" {
		t.Errorf("click y=3 selected %v %v, want context (row 2)", sel.Name, ok)
	}
}

func TestClickOutsideRowsIgnored(t *testing.T) {
	m := newTestModel()
	for _, y := range []int{0, len(testCommands()) + 1} {
		updated, _ := m.Update(tea.MouseClickMsg{X: 2, Y: y, Button: tea.MouseLeft})
		if _, ok := updated.(Model).Selection(); ok {
			t.Errorf("click at y=%d must not select", y)
		}
	}
}

func TestWheelMovesCursor(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	m = updated.(Model)
	if m.cursor != 1 {
		t.Errorf("wheel down: cursor = %d, want 1", m.cursor)
	}
}

func TestNoMatchState(t *testing.T) {
	m := typeText(newTestModel(), "zzzqqq")
	content := m.View().Content
	if !strings.Contains(content, "no commands match") {
		t.Error("gibberish query should render the empty state")
	}
	m = press(m, tea.Key{Code: tea.KeyEnter})
	if _, ok := m.Selection(); ok {
		t.Error("Enter on empty result set must not select")
	}
}

func TestViewportFollowsCursor(t *testing.T) {
	cmds := make([]catalog.Command, 30)
	for i := range cmds {
		cmds[i] = catalog.Command{Name: strings.Repeat("x", i+1)}
	}
	m := New(cmds, theme.Plain())
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 6}) // pageSize 4
	m = updated.(Model)
	for range 10 {
		m = press(m, tea.Key{Code: 'j', Text: "j"})
	}
	if m.cursor != 10 {
		t.Fatalf("cursor = %d, want 10", m.cursor)
	}
	if m.cursor < m.offset || m.cursor >= m.offset+m.pageSize() {
		t.Errorf("cursor %d outside viewport [%d,%d)", m.cursor, m.offset, m.offset+m.pageSize())
	}
}

func TestBackspaceRefilters(t *testing.T) {
	m := typeText(newTestModel(), "ctxz")
	if len(m.visible) != 0 {
		t.Fatalf("ctxz should match nothing, got %d", len(m.visible))
	}
	m = press(m, tea.Key{Code: tea.KeyBackspace})
	if m.query != "ctx" || len(m.visible) == 0 {
		t.Errorf("backspace: query %q visible %d, want ctx with matches", m.query, len(m.visible))
	}
}

package widget

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/kylesnowschwartz/gearshifter/internal/catalog"
	"github.com/kylesnowschwartz/gearshifter/internal/theme"
)

func modelGearChip() GearChip {
	return NewGearChip(testStyles, catalog.Command{Name: "model"}, "MODEL",
		[]string{"haiku", "sonnet", "opus", "fable"})
}

func TestChipViewAndIntent(t *testing.T) {
	c := NewChip(testStyles, catalog.Command{Name: "copy"}, "COPY", "⧉", false)
	view := c.View(RenderState{}, c.NaturalWidth())
	if !strings.Contains(view, "⧉ COPY") {
		t.Errorf("chip view = %q, want glyph+label", view)
	}
	if lipgloss.Width(view) != c.NaturalWidth() {
		t.Errorf("chip width %d != natural %d", lipgloss.Width(view), c.NaturalWidth())
	}
	msg, ok := c.Activate().(TileActivatedMsg)
	if !ok || msg.Command.Name != "copy" || msg.Insert {
		t.Errorf("chip intent = %+v, want copy TileActivated", msg)
	}
	insert := NewChip(testStyles, catalog.Command{Name: "goal"}, "GOAL", "•", true)
	if got := insert.Activate().(TileActivatedMsg); !got.Insert {
		t.Error("insert chip must carry the insert request")
	}
}

func TestGearChipBadgeMarksCurrent(t *testing.T) {
	g := modelGearChip()
	if badge := g.badge(); badge != "M:—" {
		t.Errorf("stateless badge = %q, want M:—", badge)
	}
	g = g.WithCurrent("opus")
	if badge := g.badge(); badge != "M:opus" {
		t.Errorf("live badge = %q, want M:opus", badge)
	}
}

// The cursor highlight is a focus artifact: an unfocused expanded row
// renders identically wherever the cursor sits (companion QA
// 2026-07-06 — a hover-stranded white block read as a stuck
// selection). Needs a colored theme; plain renders everything alike.
func TestGearChipCursorHighlightNeedsFocus(t *testing.T) {
	st, err := theme.Load("default")
	if err != nil {
		t.Fatal(err)
	}
	g := NewGearChip(st, catalog.Command{Name: "model"}, "MODEL",
		[]string{"haiku", "sonnet", "opus", "fable"}).WithCurrent("opus").Expand()
	moved := g.CursorNext() // cursor off the current value
	w := g.NaturalWidth()
	unfocused := RenderState{}
	if g.View(unfocused, w) != moved.View(unfocused, w) {
		t.Error("an unfocused row must not paint the cursor — it renders the same wherever the cursor sits")
	}
	focused := RenderState{Focused: true}
	if g.View(focused, w) == moved.View(focused, w) {
		t.Error("a focused row must paint the cursor")
	}
}

// Expanding parks the cursor on the current value; the expanded row and
// ValueAtX must agree cell-for-cell (same segments source).
func TestGearChipExpandAndHitGeometry(t *testing.T) {
	g := modelGearChip().WithCurrent("opus").Expand()
	if !g.Expanded || g.cursor != 2 {
		t.Fatalf("expand: expanded=%v cursor=%d, want true/2", g.Expanded, g.cursor)
	}
	view := g.View(RenderState{Focused: true}, g.NaturalWidth())
	if !strings.Contains(view, "haiku") || !strings.Contains(view, "▐opus") {
		t.Errorf("expanded view must show all values with the current marked:\n%q", view)
	}
	// Hit each value at the cell where the view renders it.
	plainView := " M: haiku sonnet ▐opus fable"
	for i, val := range []string{"haiku", "sonnet", "opus", "fable"} {
		x := strings.Index(plainView, val)
		if v, ok := g.ValueAtX(x); !ok || v != i {
			t.Errorf("ValueAtX(%d) over %q = %d,%v; want %d", x, val, v, ok, i)
		}
	}
	if _, ok := g.ValueAtX(1); ok {
		t.Error("the prefix must not hit a value")
	}
	if _, ok := modelGearChip().ValueAtX(3); ok {
		t.Error("a collapsed chip has no value hits")
	}
}

func TestGearChipCursorWrapsAndFires(t *testing.T) {
	g := modelGearChip().WithCurrent("haiku").Expand()
	g = g.CursorPrev() // wraps to fable
	msg, ok := g.Activate().(GearShiftedMsg)
	if !ok || msg.Value != "fable" {
		t.Errorf("fire = %+v, want model fable", msg)
	}
	if got := g.Collapse(); got.Expanded {
		t.Error("collapse must close the row")
	}
}

func TestLauncherChipShowsCount(t *testing.T) {
	l := NewLauncherChip(testStyles, 101)
	view := l.View(RenderState{}, l.NaturalWidth())
	if !strings.Contains(view, "ALL COMMANDS (101)") {
		t.Errorf("launcher chip = %q", view)
	}
	if _, ok := l.Activate().(ScreenRequestedMsg); !ok {
		t.Error("launcher chip must request the palette screen")
	}
}

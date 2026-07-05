package widget

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/kylesnowschwartz/gearshifter/internal/catalog"
	"github.com/kylesnowschwartz/gearshifter/internal/theme"
)

var testStyles = theme.Plain()

func modelGear() Gear {
	return NewGear(testStyles, catalog.Command{Name: "model"}, "MODEL",
		[]string{"haiku", "sonnet", "opus", "fable"}, 5)
}

func TestWithCurrentMatching(t *testing.T) {
	cases := []struct {
		setting string
		want    int
	}{
		{"opus", 2},
		{"claude-fable-5[1m]", 3},
		{"Sonnet", 1},
		{"default", -1},
		{"", -1},
	}
	for _, c := range cases {
		g := modelGear().WithCurrent(c.setting)
		if g.current != c.want {
			t.Errorf("WithCurrent(%q): current = %d, want %d", c.setting, g.current, c.want)
		}
		if c.want >= 0 && g.cursor != c.want {
			t.Errorf("WithCurrent(%q): cursor should start on current", c.setting)
		}
	}
}

func TestWithCurrentPrefersExactMatch(t *testing.T) {
	// containment alone would let "o" claim the setting "opus" (review
	// finding): the exact value must win over an earlier substring value.
	g := NewGear(testStyles, catalog.Command{Name: "model"}, "MODEL", []string{"o", "opus"}, 5).
		WithCurrent("opus")
	if g.current != 1 {
		t.Errorf("exact match must beat substring: current = %d, want 1", g.current)
	}
}

func TestTruncateIsCellAware(t *testing.T) {
	// Wide runes: rune-slicing would keep 4 runes = 8 cells; the tile
	// budget is in cells, and overflow desyncs compositor hit-testing.
	got := truncate("ワイドラベル", 5)
	if w := 4; len([]rune(got)) > 2 || got == "" {
		t.Errorf("truncate(wide, 5) = %q (want ≤ %d cells, non-empty)", got, w)
	}
	if truncate("plain", 10) != "plain" {
		t.Error("strings within budget pass through untouched")
	}
}

// Themes may change color, never geometry: the compositor hit-tests the
// rendered cells, so a themed tile must occupy exactly the cells its
// plain twin does (M5 P1 contract).
func TestThemedTileGeometryMatchesPlain(t *testing.T) {
	themed, err := theme.Load("default")
	if err != nil {
		t.Fatal(err)
	}
	cmd := catalog.Command{Name: "compact"}
	pairs := []struct {
		name         string
		plain, color Tile
	}{
		{"button", NewButton(testStyles, cmd, "COMPACT", 4), NewButton(themed, cmd, "COMPACT", 4)},
		{"gear", modelGear(), NewGear(themed, catalog.Command{Name: "model"}, "MODEL",
			[]string{"haiku", "sonnet", "opus", "fable"}, 5)},
		{"launcher", NewLauncher(testStyles, 42, 13), NewLauncher(themed, 42, 13)},
	}
	for _, p := range pairs {
		for _, rs := range []RenderState{{}, {Focused: true}, {Focused: true, Armed: true}} {
			a := strings.Split(p.plain.View(rs, 20), "\n")
			b := strings.Split(p.color.View(rs, 20), "\n")
			if len(a) != len(b) {
				t.Errorf("%s %+v: row count %d vs %d", p.name, rs, len(a), len(b))
				continue
			}
			for i := range a {
				if wa, wb := lipgloss.Width(a[i]), lipgloss.Width(b[i]); wa != wb {
					t.Errorf("%s %+v row %d: width %d vs %d", p.name, rs, i, wa, wb)
				}
			}
		}
	}
}

func TestGearViewMarksCurrent(t *testing.T) {
	view := modelGear().WithCurrent("opus").View(RenderState{}, 20)
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "opus") && !strings.Contains(line, "▐") {
			t.Error("current value row must carry the ▐ mark")
		}
		if strings.Contains(line, "haiku") && strings.Contains(line, "▐") {
			t.Error("non-current rows must not carry the mark")
		}
	}
}

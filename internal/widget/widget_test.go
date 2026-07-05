package widget

import (
	"strings"
	"testing"

	"github.com/kylesnowschwartz/gearshifter/internal/catalog"
)

func modelGear() Gear {
	return NewGear(catalog.Command{Name: "model"}, "MODEL",
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

func TestGearViewMarksCurrent(t *testing.T) {
	view := modelGear().WithCurrent("opus").View(false, 20)
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "opus") && !strings.Contains(line, "▐") {
			t.Error("current value row must carry the ▐ mark")
		}
		if strings.Contains(line, "haiku") && strings.Contains(line, "▐") {
			t.Error("non-current rows must not carry the mark")
		}
	}
}

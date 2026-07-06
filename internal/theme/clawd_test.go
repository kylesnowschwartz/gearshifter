package theme

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

// The mascot ladder: full 5-row clawd when rows allow, the 3-row
// half-block mini below that, nothing when even the mini would crowd.
// Plain never renders it — the sprite is pure decoration.
func TestMascotSizesToAvailableRows(t *testing.T) {
	def, err := Load("default")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		maxRows int
		want    int
	}{{10, 5}, {5, 5}, {4, 3}, {3, 3}, {2, 0}, {0, 0}}
	for _, c := range cases {
		got := def.Mascot(c.maxRows)
		if len(got) != c.want {
			t.Errorf("Mascot(%d) = %d rows, want %d", c.maxRows, len(got), c.want)
		}
		for i, row := range got {
			if w := lipgloss.Width(row); w != MascotWidth {
				t.Errorf("Mascot(%d) row %d is %d cells, want %d", c.maxRows, i, w, MascotWidth)
			}
		}
	}
	if full := strings.Join(def.Mascot(5), ""); !strings.Contains(full, "█") {
		t.Error("the full mascot must paint solid cells")
	}
	if mini := strings.Join(def.Mascot(3), ""); !strings.Contains(mini, "▀") {
		t.Error("the mini mascot must paint half-block cells")
	}
	if got := Plain().Mascot(10); got != nil {
		t.Errorf("plain must render no mascot, got %d rows", len(got))
	}
}

package layout

import (
	"testing"

	"github.com/kylesnowschwartz/gearshifter/internal/agent"
	"github.com/kylesnowschwartz/gearshifter/internal/catalog"
	"github.com/kylesnowschwartz/gearshifter/internal/widget"
)

func TestDefaultPlacementOrderIsReadingOrder(t *testing.T) {
	placements := Default(nil, agent.State{})
	want := []string{"MODEL", "EFFORT", "COMPACT", "COPY", "CLEAR", "CONTEXT", "RESUME", "CONFIG"}
	if len(placements) != len(want)+1 {
		t.Fatalf("Default yields %d placements, want %d", len(placements), len(want)+1)
	}
	for i, label := range want {
		var got string
		switch tile := placements[i].Tile.(type) {
		case widget.Gear:
			got = tile.Label
		case widget.Button:
			got = tile.Label
		}
		if got != label {
			t.Errorf("placement %d = %q, want %q", i, got, label)
		}
	}
	if _, ok := placements[len(want)].Tile.(widget.Launcher); !ok {
		t.Error("last placement must be the launcher bar")
	}
}

func TestFindCommandFallsBackToBareName(t *testing.T) {
	cmds := []catalog.Command{{Name: "review", ArgumentHint: "<pr>"}}
	if got := findCommand(cmds, "review"); got.ArgumentHint != "<pr>" {
		t.Errorf("known command must carry the catalog entry, got %+v", got)
	}
	if got := findCommand(cmds, "ghost"); got.Name != "ghost" {
		t.Errorf("missing command must still yield a working bare command, got %+v", got)
	}
}

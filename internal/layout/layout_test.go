package layout

import (
	"testing"

	"github.com/kylesnowschwartz/gearshifter/internal/agent"
	"github.com/kylesnowschwartz/gearshifter/internal/catalog"
	"github.com/kylesnowschwartz/gearshifter/internal/theme"
	"github.com/kylesnowschwartz/gearshifter/internal/widget"
)

// testStyles is shared by layout_test.go and toml_test.go: one pointer,
// so Load-vs-Default DeepEqual comparisons see identical tiles.
var testStyles = theme.Plain()

// tileLabel reads the label off whichever tile kind Default() placed.
func tileLabel(t *testing.T, tile widget.Tile) string {
	t.Helper()
	switch tile := tile.(type) {
	case widget.Gear:
		return tile.Label
	case widget.Button:
		return tile.Label
	default:
		return ""
	}
}

func assertPlacementLabels(t *testing.T, placements []Placement, want []string) {
	t.Helper()
	for i, label := range want {
		if got := tileLabel(t, placements[i].Tile); got != label {
			t.Errorf("placement %d = %q, want %q", i, got, label)
		}
	}
}

func TestDefaultPlacementOrderIsReadingOrder(t *testing.T) {
	placements := Default(nil, agent.State{}, testStyles, SortNone)
	want := []string{"MODEL", "EFFORT", "COMPACT", "COPY", "CLEAR", "CONTEXT",
		"RESUME", "CONFIG", "AGENTS", "DOCTOR", "GOAL", "MCP", "MEMORY",
		"PERMS", "PLUGIN", "RADIO", "RELOAD", "RENAME"}
	if len(placements) != len(want)+1 {
		t.Fatalf("Default yields %d placements, want %d", len(placements), len(want)+1)
	}
	assertPlacementLabels(t, placements, want)
	if _, ok := placements[len(want)].Tile.(widget.Launcher); !ok {
		t.Error("last placement must be the launcher bar")
	}
}

// SortAlpha treats the whole 16-button field as one alphabetized list,
// six-pack included — unlike SortNone, which leads with the six-pack's
// usage rank. The six-pack's shipped order (COMPACT, COPY, CLEAR, ...)
// isn't alphabetical, so seeing it reshuffle here proves the sort runs.
func TestDefaultSortAlphaSortsWholeField(t *testing.T) {
	want := []string{"MODEL", "EFFORT",
		"AGENTS", "CLEAR", "COMPACT", "CONFIG", "CONTEXT", "COPY",
		"DOCTOR", "GOAL", "MCP", "MEMORY", "PERMS", "PLUGIN",
		"RADIO", "RELOAD", "RENAME", "RESUME"}
	assertPlacementLabels(t, Default(nil, agent.State{}, testStyles, SortAlpha), want)
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

package palette

import (
	"testing"

	"github.com/kylesnowschwartz/gearshifter/internal/catalog"
)

func testCommands() []catalog.Command {
	return []catalog.Command{
		{Name: "add-dir", Description: "Add a new working directory"},
		{Name: "btw", Description: "Ask a quick side question"},
		{Name: "context", Description: "Visualize current context usage"},
		{Name: "revdiff", Description: "Review diffs with annotations"},
		{Name: "review", Description: "Review a pull request"},
	}
}

func names(cmds []catalog.Command, idx []int) []string {
	out := make([]string, len(idx))
	for i, n := range idx {
		out[i] = cmds[n].Name
	}
	return out
}

func TestFilterEmptyQueryKeepsCatalogOrder(t *testing.T) {
	cmds := testCommands()
	got := names(cmds, filterCommands(cmds, ""))
	if len(got) != len(cmds) || got[0] != "add-dir" || got[4] != "review" {
		t.Errorf("empty query = %v, want catalog order", got)
	}
}

func TestFilterPrefixBeatsSubstring(t *testing.T) {
	cmds := testCommands()
	got := names(cmds, filterCommands(cmds, "rev"))
	if len(got) < 2 || got[0] != "revdiff" && got[0] != "review" {
		t.Fatalf("query rev = %v, want prefix matches first", got)
	}
	for _, n := range got {
		if n == "btw" {
			t.Errorf("btw should not match %q", "rev")
		}
	}
}

func TestFilterSubsequenceMatches(t *testing.T) {
	cmds := testCommands()
	got := names(cmds, filterCommands(cmds, "ctx"))
	if len(got) == 0 || got[0] != "context" {
		t.Errorf("query ctx = %v, want context via subsequence", got)
	}
}

func TestFilterDescriptionFallback(t *testing.T) {
	cmds := testCommands()
	got := names(cmds, filterCommands(cmds, "side question"))
	if len(got) != 1 || got[0] != "btw" {
		t.Errorf("query 'side question' = %v, want [btw] via description", got)
	}
}

func TestFilterNoMatch(t *testing.T) {
	cmds := testCommands()
	if got := filterCommands(cmds, "zzzzqqq"); len(got) != 0 {
		t.Errorf("gibberish query matched %v", names(cmds, got))
	}
}

func TestFilterCaseInsensitive(t *testing.T) {
	cmds := testCommands()
	got := names(cmds, filterCommands(cmds, "BTW"))
	if len(got) == 0 || got[0] != "btw" {
		t.Errorf("query BTW = %v, want btw", got)
	}
}

package layout

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/kylesnowschwartz/gearshifter/internal/agent"
	"github.com/kylesnowschwartz/gearshifter/internal/catalog"
	"github.com/kylesnowschwartz/gearshifter/internal/widget"
)

func writeLayout(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "layout.toml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// The shipped example documents the default deck; this pins the two
// together so the example can't drift from Default().
func TestExampleLayoutReproducesDefaultDeck(t *testing.T) {
	cmds := []catalog.Command{{Name: "review", ArgumentHint: ""}, {Name: "model"}}
	state := agent.State{Model: "haiku", Effort: "high"}
	got, err := Load("../../examples/layout.toml", cmds, state, testStyles)
	if err != nil {
		t.Fatal(err)
	}
	want := Default(cmds, state, testStyles)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Load(examples/layout.toml) diverges from Default():\ngot  %+v\nwant %+v", got, want)
	}
}

func TestLoadFlowsRowsFromColumnOverlap(t *testing.T) {
	placements, err := Load(writeLayout(t, `
[[tile]]
type    = "button"
command = "review"
col     = 0
span    = 13

[[tile]]
type    = "button"
command = "context"
`), nil, agent.State{}, testStyles)
	if err != nil {
		t.Fatal(err)
	}
	if placements[0].Y != 2 {
		t.Errorf("first row must sit at bodyY, got %d", placements[0].Y)
	}
	// Buttons are 3 rows (label between borders, /cmd nameplate in the
	// bottom border): full-width tile rows 2..4 + rowGap → 6.
	if placements[1].Y != 6 {
		t.Errorf("overlapping tile must drop below, got y=%d, want 6", placements[1].Y)
	}
}

func TestLoadDefaultsLabelSpanAndSlash(t *testing.T) {
	placements, err := Load(writeLayout(t, `
[[tile]]
type    = "button"
command = "/radio"
`), nil, agent.State{}, testStyles)
	if err != nil {
		t.Fatal(err)
	}
	b, ok := placements[0].Tile.(widget.Button)
	if !ok {
		t.Fatalf("want a Button, got %T", placements[0].Tile)
	}
	if b.Label != "RADIO" || b.Cmd.Name != "radio" || b.Span() != 4 {
		t.Errorf("defaults: label %q cmd %q span %d, want RADIO/radio/4", b.Label, b.Cmd.Name, b.Span())
	}
}

func TestLoadMarksGearCurrentFromState(t *testing.T) {
	placements, err := Load(writeLayout(t, `
[[tile]]
type    = "gear"
command = "model"
values  = ["haiku", "sonnet"]
`), nil, agent.State{Model: "claude-sonnet-5"}, testStyles)
	if err != nil {
		t.Fatal(err)
	}
	if view := placements[0].Tile.View(widget.RenderState{}, 20); !strings.Contains(view, "▐ sonnet") {
		t.Errorf("gear must mark the live value from state:\n%s", view)
	}
}

// Every breakage fails with words that name the offending line (Kyle QA
// contract: break the file on purpose — the error names the line).
func TestLoadErrorsNameTheLine(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    string // must appear in the error
	}{
		{"unknown key", "[[tile]]\ntype = \"button\"\ncommand = \"x\"\nicon = \"magnifier\"", ":4: unknown key \"tile.icon\""},
		{"syntax error", "[[tile]]\ntype = ", ":2:"},
		{"missing type", "\n\n[[tile]]\ncommand = \"x\"", ":3: [[tile]] 1: missing type"},
		{"unknown type", "[[tile]]\ntype = \"knob\"", `:1: [[tile]] 1: unknown type "knob"`},
		{"missing command", "[[tile]]\ntype = \"button\"", ":1: [[tile]] 1: button needs command"},
		{"gear without values", "[[tile]]\ntype = \"gear\"\ncommand = \"model\"", ":1: [[tile]] 1: gear needs values"},
		{"zero span", "[[tile]]\ntype = \"button\"\ncommand = \"x\"\nspan = 0", "span 0 must be at least 1"},
		{"col out of range", "[[tile]]\ntype = \"button\"\ncommand = \"x\"\ncol = 13", "col 13 out of range 0–12"},
		{"grid overflow", "[[tile]]\ntype = \"button\"\ncommand = \"x\"\ncol = 10\nspan = 4", "col 10 + span 4 overflows the 13-column grid"},
		{"second tile line", "[[tile]]\ntype = \"launcher\"\n\n[[tile]]\ntype = \"knob\"", ":4: [[tile]] 2:"},
		{"empty file", "", "no [[tile]] entries"},
	}
	for _, c := range cases {
		_, err := Load(writeLayout(t, c.content), nil, agent.State{}, testStyles)
		if err == nil {
			t.Errorf("%s: want error containing %q, got nil", c.name, c.want)
			continue
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error %q must contain %q", c.name, err, c.want)
		}
	}
}

// kyle.toml is a personal layout, not Default-pinned — but it must
// always parse, and its insert feature must hold. (Its STYLE gear was
// removed 2026-07-05: Claude Code dropped /output-style and /config
// rejects custom styles non-interactively — DECK-CONTENT.md postscript.)
func TestKyleLayoutLoads(t *testing.T) {
	placements, err := Load("../../examples/kyle.toml", nil, agent.State{}, testStyles)
	if err != nil {
		t.Fatal(err)
	}
	var goal widget.Button
	for _, p := range placements {
		if b, ok := p.Tile.(widget.Button); ok && b.Cmd.Name == "goal" {
			goal = b
		}
	}
	if !goal.Insert {
		t.Error("GOAL must be an insert-only button (always takes a condition)")
	}
}

// The gearSetting output-style → state.Style mapping outlived kyle.toml's
// STYLE gear (a plugin /style command could revive it — DECK-CONTENT.md
// postscript), so a user layout carrying one must still live-mark.
func TestStyleGearMarksLiveOutputStyle(t *testing.T) {
	placements, err := Load(writeLayout(t,
		"[[tile]]\ntype = \"gear\"\ncommand = \"output-style\"\nlabel = \"STYLE\"\nvalues = [\"mayo-clinic\", \"other\"]"),
		nil, agent.State{Style: "mayo-clinic"}, testStyles)
	if err != nil {
		t.Fatal(err)
	}
	view := placements[0].Tile.View(widget.RenderState{}, 24)
	if !strings.Contains(view, "▐ mayo-clinic") {
		t.Errorf("STYLE gear must mark the live output style:\n%s", view)
	}
}

func TestLoadRejectsInsertOnNonButtons(t *testing.T) {
	_, err := Load(writeLayout(t, "[[tile]]\ntype = \"launcher\"\ninsert = true"), nil, agent.State{}, testStyles)
	if err == nil || !strings.Contains(err.Error(), "insert = true applies only to buttons") {
		t.Errorf("insert on a launcher must error with words, got %v", err)
	}
}

func TestLoadMissingFile(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "absent.toml"), nil, agent.State{}, testStyles); err == nil {
		t.Error("missing file must error")
	}
}

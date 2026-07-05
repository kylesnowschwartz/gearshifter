package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kylesnowschwartz/gearshifter/internal/agent"
	"github.com/kylesnowschwartz/gearshifter/internal/catalog"
	"github.com/kylesnowschwartz/gearshifter/internal/tmux"
)

func TestResolveLayout(t *testing.T) {
	tomlPath := filepath.Join(t.TempDir(), "my.toml")
	if err := os.WriteFile(tomlPath, []byte("[[tile]]\ntype = \"launcher\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name                  string
		wantInbuilt, wantPath string
		wantErr               bool
	}{
		{"telescope", "telescope", "", false},
		{"deck", "deck", "", false},
		{tomlPath, "", tomlPath, false},
		{"no-such-layout", "", "", true},
	} {
		inbuilt, path, err := resolveLayout(c.name)
		if (err != nil) != c.wantErr || inbuilt != c.wantInbuilt || path != c.wantPath {
			t.Errorf("resolveLayout(%q) = (%q, %q, %v), want (%q, %q, err=%v)",
				c.name, inbuilt, path, err, c.wantInbuilt, c.wantPath, c.wantErr)
		}
	}
}

func TestBuildInjection(t *testing.T) {
	cases := []struct {
		name        string
		pick        selection
		wantText    string
		wantNoEnter bool
	}{
		{"plain button", selection{cmd: catalog.Command{Name: "review"}}, "/review", false},
		{"gear value always enters", selection{cmd: catalog.Command{Name: "model"}, arg: "opus"}, "/model opus", false},
		{"required-arg inserts", selection{cmd: catalog.Command{Name: "btw", ArgumentHint: "<question>"}}, "/btw ", true},
		{"tab insert-only", selection{cmd: catalog.Command{Name: "context"}, insertOnly: true}, "/context ", true},
		{"gear beats insert-only", selection{cmd: catalog.Command{Name: "model"}, arg: "haiku", insertOnly: true}, "/model haiku", false},
	}
	for _, c := range cases {
		text, opts := buildInjection(c.pick)
		if text != c.wantText || opts.NoEnter != c.wantNoEnter {
			t.Errorf("%s: got (%q, NoEnter=%v), want (%q, NoEnter=%v)",
				c.name, text, opts.NoEnter, c.wantText, c.wantNoEnter)
		}
	}
}

// stripTarget policy tests. The qa-rig drives strip with an explicit
// --pane (registry resolution can't work against the nested test
// server), so the auto-scan policy is pinned here with fakes.

type scriptRunner struct {
	responses map[string]string // joined argv → canned output
	errs      map[string]bool   // joined argv → fail the call
}

func (r scriptRunner) Run(stdin string, args ...string) (string, error) {
	key := strings.Join(args, " ")
	if r.errs[key] {
		return "", fmt.Errorf("tmux %s: exit 1", key)
	}
	return r.responses[key], nil
}

type fakeProvider struct{ sessions map[int]bool }

func (f fakeProvider) State(int, string) agent.State     { return agent.State{} }
func (f fakeProvider) HasSession(pid int, _ string) bool { return f.sessions[pid] }

const panesQuery = "list-panes -t %0 -F #{pane_id}\t#{pane_pid}\t#{pane_current_path}"

func TestStripTargetScansWindowForClaudePane(t *testing.T) {
	client := tmux.NewClient(scriptRunner{responses: map[string]string{
		panesQuery: "%0\t10\t/strip\n%1\t11\t/shell\n%2\t12\t/claude",
	}})
	// The strip's own pane (pid 10) also has a session — a strip run from
	// inside a Claude project dir must still never target itself.
	provider := fakeProvider{sessions: map[int]bool{10: true, 12: true}}
	target, err := stripTarget(client, provider, "", "%0")()
	if err != nil {
		t.Fatal(err)
	}
	if target.ID != "%2" || target.PID != 12 || target.Cwd != "/claude" {
		t.Errorf("target = %+v, want %%2 pid 12 /claude (self excluded, sessionless %%1 skipped)", target)
	}
}

func TestStripTargetNoClaudePaneFailsWithWords(t *testing.T) {
	client := tmux.NewClient(scriptRunner{responses: map[string]string{
		panesQuery: "%0\t10\t/strip\n%1\t11\t/shell",
	}})
	_, err := stripTarget(client, fakeProvider{}, "", "%0")()
	if err == nil || !strings.Contains(err.Error(), "no Claude pane") {
		t.Errorf("want a no-Claude-pane error, got %v", err)
	}
}

func TestStripTargetExplicitPin(t *testing.T) {
	client := tmux.NewClient(scriptRunner{responses: map[string]string{
		"list-panes -t %9":                              "", // PaneExists probe
		"display-message -p -t %9 #{pane_pid}":          "42",
		"display-message -p -t %9 #{pane_current_path}": "/proj",
	}})
	// No session needed: an explicit pin is trusted, not scanned.
	target, err := stripTarget(client, fakeProvider{}, "%9", "%0")()
	if err != nil {
		t.Fatal(err)
	}
	if target.ID != "%9" || target.PID != 42 || target.Cwd != "/proj" {
		t.Errorf("pinned target = %+v, want %%9 pid 42 /proj", target)
	}

	gone := tmux.NewClient(scriptRunner{errs: map[string]bool{"list-panes -t %9": true}})
	if _, err := stripTarget(gone, fakeProvider{}, "%9", "%0")(); err == nil || !strings.Contains(err.Error(), "gone") {
		t.Errorf("a dead pinned pane must fail with words, got %v", err)
	}
}

// Pinning the strip's own pane must fail with words: self-injection
// loops — the recipe's trailing Enter re-fires the focused tile,
// forever (review finding, CONFIRMED).
func TestStripTargetRejectsSelfPin(t *testing.T) {
	client := tmux.NewClient(scriptRunner{})
	if _, err := stripTarget(client, fakeProvider{}, "%0", "%0")(); err == nil || !strings.Contains(err.Error(), "itself") {
		t.Errorf("self pin must be rejected with words, got %v", err)
	}
}

func TestRunStripRejectsTelescope(t *testing.T) {
	err := runStrip([]string{"--layout", "telescope"})
	if err == nil || !strings.Contains(err.Error(), "telescope") {
		t.Errorf("strip --layout telescope must be rejected with words, got %v", err)
	}
}

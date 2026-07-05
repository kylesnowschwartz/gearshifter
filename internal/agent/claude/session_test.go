package claude

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kylesnowschwartz/agent-ouija/claude/claudedir"
)

// Tests of the moved mechanisms (process-tree walk, session matching,
// path encoding, transcript tail scan) live in agent-ouija's registry,
// claudedir, and transcript packages. What stays here is gearshifter
// policy (the settings-vs-transcript mtime arbitration, the global
// fallback) plus one black-box contract guard on the library seam.

func writeTranscript(t *testing.T, root claudedir.Root, cwd, sessionID string, lines ...string) string {
	t.Helper()
	dir := root.ProjectDirFor(cwd)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, sessionID+".jsonl")
	content := ""
	for _, l := range lines {
		content += l + "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeRegistryEntry(t *testing.T, root claudedir.Root, sessionID, cwd string) {
	t.Helper()
	dir := root.SessionsDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	entry := fmt.Sprintf(`{"pid":%d,"sessionId":%q,"cwd":%q,"startedAt":"2026-07-05T01:00:00Z"}`, os.Getpid(), sessionID, cwd)
	if err := os.WriteFile(filepath.Join(dir, "1.json"), []byte(entry), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestStateArbitratesByMtime(t *testing.T) {
	root := writeSettings(t, `{"model":"opus","effortLevel":"high"}`)
	cwd := "/proj"
	transcript := writeTranscript(t, root, cwd, "s1",
		`{"type":"assistant","message":{"model":"claude-fable-5"}}`)
	writeRegistryEntry(t, root, "s1", cwd)

	// Transcript newer than settings → session model wins.
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(root.SettingsPath(), old, old); err != nil {
		t.Fatal(err)
	}
	if s := NewAt(root).State(0, cwd); s.Model != "claude-fable-5" || s.Effort != "high" {
		t.Errorf("transcript-newer: got %+v, want session model claude-fable-5, effort high", s)
	}

	// Settings newer than transcript (gear just clicked) → settings wins.
	if err := os.Chtimes(transcript, old, old); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if err := os.Chtimes(root.SettingsPath(), now, now); err != nil {
		t.Fatal(err)
	}
	if s := NewAt(root).State(0, cwd); s.Model != "opus" {
		t.Errorf("settings-newer: got %q, want opus", s.Model)
	}
}

func TestStateFallsBackToGlobal(t *testing.T) {
	root := writeSettings(t, `{"model":"opus","effortLevel":"low"}`)
	if s := NewAt(root).State(0, "/nowhere"); s.Model != "opus" || s.Effort != "low" {
		t.Errorf("no session: got %+v, want global opus/low", s)
	}
}

// Black-box contract guard on the library seam. The deleted white-box
// tests pinned these semantics app-side; their library replacements can
// drift without failing gearshifter's own suite — and drift HAPPENED
// mid-migration (a library change to full-Entry decoding rejected lines
// with wrong-typed unrelated fields until a follow-up fix). This test
// re-establishes the guard: "<synthetic>" entries are skipped in favor of
// older real models, and format drift in fields gearshifter never reads
// must not cost it the model.
func TestStateSurvivesTranscriptDrift(t *testing.T) {
	root := writeSettings(t, `{"effortLevel":"high"}`) // no model: transcript must supply it
	cwd := "/proj"
	writeTranscript(t, root, cwd, "s1",
		`{"type":"assistant","isMeta":"yes","stop_reason":{"weird":true},"message":{"model":"claude-fable-5"}}`,
		`{"type":"assistant","message":{"model":"<synthetic>"}}`,
	)
	writeRegistryEntry(t, root, "s1", cwd)

	if s := NewAt(root).State(0, cwd); s.Model != "claude-fable-5" {
		t.Errorf("got model %q, want claude-fable-5 (synthetic skipped, drift-typed fields tolerated)", s.Model)
	}
}

// HasSession is strip mode's pane-scan predicate: the same registry
// resolution State uses, as a yes/no. Fail-open like everything here —
// no registry entry (or no root at all) means false, never an error.
func TestHasSession(t *testing.T) {
	root := writeSettings(t, `{}`)
	writeRegistryEntry(t, root, "s1", "/proj")
	if !NewAt(root).HasSession(0, "/proj") {
		t.Error("a registered pane must report a session")
	}
	if NewAt(root).HasSession(0, "/nowhere") {
		t.Error("an unregistered pane must not report a session")
	}
	if (Claude{}).HasSession(0, "/proj") {
		t.Error("a zero-root provider must degrade to false")
	}
}

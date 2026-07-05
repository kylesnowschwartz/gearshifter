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
// policy: the settings-vs-transcript mtime arbitration and the global
// fallback.

func writeTranscript(t *testing.T, home, cwd, sessionID string, lines ...string) string {
	t.Helper()
	dir := filepath.Join(home, ".claude", "projects", claudedir.EncodeProjectPath(cwd))
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

func TestStateArbitratesByMtime(t *testing.T) {
	home := writeSettings(t, `{"model":"opus","effortLevel":"high"}`)
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	cwd := "/proj"
	transcript := writeTranscript(t, home, cwd, "s1",
		`{"type":"assistant","message":{"model":"claude-fable-5"}}`)
	registry := filepath.Join(home, ".claude", "sessions")
	if err := os.MkdirAll(registry, 0o755); err != nil {
		t.Fatal(err)
	}
	entry := fmt.Sprintf(`{"pid":%d,"sessionId":"s1","cwd":"/proj","startedAt":"2026-07-05T01:00:00Z"}`, os.Getpid())
	if err := os.WriteFile(filepath.Join(registry, "1.json"), []byte(entry), 0o644); err != nil {
		t.Fatal(err)
	}

	// Transcript newer than settings → session model wins.
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(settingsPath, old, old); err != nil {
		t.Fatal(err)
	}
	if s := New(home).State(0, cwd); s.Model != "claude-fable-5" || s.Effort != "high" {
		t.Errorf("transcript-newer: got %+v, want session model claude-fable-5, effort high", s)
	}

	// Settings newer than transcript (gear just clicked) → settings wins.
	if err := os.Chtimes(transcript, old, old); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if err := os.Chtimes(settingsPath, now, now); err != nil {
		t.Fatal(err)
	}
	if s := New(home).State(0, cwd); s.Model != "opus" {
		t.Errorf("settings-newer: got %q, want opus", s.Model)
	}
}

func TestStateFallsBackToGlobal(t *testing.T) {
	home := writeSettings(t, `{"model":"opus","effortLevel":"low"}`)
	if s := New(home).State(0, "/nowhere"); s.Model != "opus" || s.Effort != "low" {
		t.Errorf("no session: got %+v, want global opus/low", s)
	}
}

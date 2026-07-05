package claude

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDescendantsOf(t *testing.T) {
	children := map[int][]int{1: {10, 20}, 10: {30}, 30: {40}}
	desc := descendantsOf(children, 10)
	for _, pid := range []int{10, 30, 40} {
		if !desc[pid] {
			t.Errorf("pid %d should be a descendant of 10", pid)
		}
	}
	if desc[20] || desc[1] {
		t.Error("siblings and ancestors are not descendants")
	}
	if len(descendantsOf(children, 0)) != 0 {
		t.Error("root 0 must yield nothing")
	}
}

func TestFindSession(t *testing.T) {
	alive := os.Getpid() // liveness check needs a real process
	entries := []sessionEntry{
		{PID: alive, SessionID: "in-tree", Cwd: "/a", StartedAt: "2026-07-05T01:00:00Z"},
		{PID: alive, SessionID: "same-cwd-old", Cwd: "/b", StartedAt: "2026-07-05T01:00:00Z"},
		{PID: alive, SessionID: "same-cwd-new", Cwd: "/b", StartedAt: "2026-07-05T02:00:00Z"},
		{PID: 99999999, SessionID: "dead", Cwd: "/b", StartedAt: "2026-07-05T03:00:00Z"},
	}
	if e, ok := findSession(entries, map[int]bool{alive: true}, "/x"); !ok || e.SessionID != "in-tree" {
		t.Errorf("pid-in-tree match: got %v %v, want in-tree", e.SessionID, ok)
	}
	if e, ok := findSession(entries, nil, "/b"); !ok || e.SessionID != "same-cwd-new" {
		t.Errorf("cwd fallback: got %v %v, want same-cwd-new (newest alive)", e.SessionID, ok)
	}
	if _, ok := findSession(entries, nil, "/nowhere"); ok {
		t.Error("no match must report not-found")
	}
	// A registry entry with no startedAt is still a live cwd match
	// (review finding: "" > "" is false, so it was never selected).
	bare := []sessionEntry{{PID: alive, SessionID: "no-started-at", Cwd: "/c"}}
	if e, ok := findSession(bare, nil, "/c"); !ok || e.SessionID != "no-started-at" {
		t.Errorf("empty startedAt: got %v %v, want no-started-at", e.SessionID, ok)
	}
}

func TestEncodeProjectPath(t *testing.T) {
	got := encodeProjectPath("/Users/kyle/Code/my_projects/gear.shifter")
	want := "-Users-kyle-Code-my-projects-gear-shifter"
	if got != want {
		t.Errorf("encodeProjectPath = %q, want %q", got, want)
	}
}

func writeTranscript(t *testing.T, home, cwd, sessionID string, lines ...string) string {
	t.Helper()
	dir := filepath.Join(home, ".claude", "projects", encodeProjectPath(cwd))
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

func TestTranscriptModel(t *testing.T) {
	home := t.TempDir()
	writeTranscript(t, home, "/proj", "abc",
		`{"type":"assistant","message":{"model":"claude-sonnet-5"}}`,
		`{"type":"user","message":{}}`,
		`{"type":"assistant","message":{"model":"claude-opus-4-8"}}`,
		`{"type":"assistant","message":{"model":"<synthetic>"}}`,
	)
	model, _ := transcriptModel(home, "/proj", "abc")
	if model != "claude-opus-4-8" {
		t.Errorf("model = %q, want claude-opus-4-8 (last real assistant entry)", model)
	}
	if m, _ := transcriptModel(home, "/proj", "missing"); m != "" {
		t.Errorf("missing transcript must yield empty, got %q", m)
	}
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

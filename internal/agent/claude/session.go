package claude

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/kylesnowschwartz/gearshifter/internal/agent"
)

// Session-specific gear state (M3 P3.5). Mechanisms verified against
// Claude Code 2.1.201 and the ecosystem's reference projects (ccusage,
// pixtuoid, tail-claude — see M3-DECK.md P3):
//
//   pane pid → descendant process tree → ~/.claude/sessions/<pid>.json
//   (Claude Code's live-process registry) → sessionId → transcript
//   ~/.claude/projects/<enc-cwd>/<sessionId>.jsonl → last assistant
//   message.model.
//
// Effort has no per-session disk source anywhere; it stays global.

// sessionEntry is one file of Claude Code's live-session registry. Only
// the fields we need are decoded.
type sessionEntry struct {
	PID       int    `json:"pid"`
	SessionID string `json:"sessionId"`
	Cwd       string `json:"cwd"`
	StartedAt string `json:"startedAt"`
}

// sessionState returns the gear state for the Claude session running in
// the target pane: session-specific model when resolvable, global
// settings otherwise; effort is always global (not persisted per session).
// The fresher file wins the model — settings.json updates the instant
// /model runs, transcripts only on the next assistant reply.
func sessionState(home string, panePID int, paneCwd string) agent.State {
	state := readSettings(home)
	entry, ok := findSession(readSessionRegistry(home), descendantsOf(processChildren(), panePID), paneCwd)
	if !ok {
		return state
	}
	model, transcriptTime := transcriptModel(home, entry.Cwd, entry.SessionID)
	if model == "" {
		return state
	}
	settingsTime := mtime(filepath.Join(home, ".claude", "settings.json"))
	if transcriptTime.After(settingsTime) || state.Model == "" {
		state.Model = model
	}
	return state
}

func readSessionRegistry(home string) []sessionEntry {
	files, _ := filepath.Glob(filepath.Join(home, ".claude", "sessions", "*.json"))
	var entries []sessionEntry
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var e sessionEntry
		if json.Unmarshal(raw, &e) == nil && e.SessionID != "" {
			entries = append(entries, e)
		}
	}
	return entries
}

// findSession matches the pane's Claude session: a registry pid inside the
// pane's process tree is definitive; otherwise fall back to cwd equality,
// preferring the newest startedAt. Registry files linger after exit, so
// entries must belong to a live process.
func findSession(entries []sessionEntry, paneTree map[int]bool, paneCwd string) (sessionEntry, bool) {
	var byCwd sessionEntry
	for _, e := range entries {
		if !pidAlive(e.PID) {
			continue
		}
		if paneTree[e.PID] {
			return e, true
		}
		if e.Cwd == paneCwd && e.StartedAt > byCwd.StartedAt {
			byCwd = e
		}
	}
	return byCwd, byCwd.SessionID != ""
}

func pidAlive(pid int) bool {
	return pid > 0 && syscall.Kill(pid, 0) == nil
}

// processChildren maps every pid to its children (one ps call).
func processChildren() map[int][]int {
	out, err := exec.Command("ps", "-axo", "pid=,ppid=").Output()
	if err != nil {
		return nil
	}
	children := map[int][]int{}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		pid, err1 := strconv.Atoi(fields[0])
		ppid, err2 := strconv.Atoi(fields[1])
		if err1 == nil && err2 == nil {
			children[ppid] = append(children[ppid], pid)
		}
	}
	return children
}

// descendantsOf returns root and every process below it.
func descendantsOf(children map[int][]int, root int) map[int]bool {
	desc := map[int]bool{}
	if root <= 0 {
		return desc
	}
	queue := []int{root}
	for len(queue) > 0 {
		pid := queue[0]
		queue = queue[1:]
		if desc[pid] {
			continue
		}
		desc[pid] = true
		queue = append(queue, children[pid]...)
	}
	return desc
}

// encodeProjectPath flattens a cwd to Claude Code's project-dir name:
// '/', '.', '_' all become '-' (symlinks resolved first).
func encodeProjectPath(p string) string {
	if resolved, err := filepath.EvalSymlinks(p); err == nil {
		p = resolved
	}
	return strings.NewReplacer("/", "-", ".", "-", "_", "-").Replace(p)
}

// transcriptTailBytes bounds the bottom-up scan; transcripts grow to
// hundreds of MB and the model sits in the last assistant entry.
const transcriptTailBytes = 256 * 1024

// transcriptModel scans the session transcript bottom-up for the last
// assistant entry's message.model (the ccusage statusline pattern).
// Returns the model with the transcript's mtime, or "" when unresolvable.
func transcriptModel(home, cwd, sessionID string) (string, time.Time) {
	path := filepath.Join(home, ".claude", "projects", encodeProjectPath(cwd), sessionID+".jsonl")
	info, err := os.Stat(path)
	if err != nil {
		return "", time.Time{}
	}
	f, err := os.Open(path)
	if err != nil {
		return "", time.Time{}
	}
	defer f.Close()
	offset := max(0, info.Size()-transcriptTailBytes)
	buf := make([]byte, info.Size()-offset)
	if _, err := f.ReadAt(buf, offset); err != nil {
		return "", time.Time{}
	}
	lines := bytes.Split(buf, []byte("\n"))
	for i := len(lines) - 1; i >= 0; i-- {
		if !bytes.Contains(lines[i], []byte(`"model"`)) {
			continue
		}
		var entry struct {
			Type    string `json:"type"`
			Message struct {
				Model string `json:"model"`
			} `json:"message"`
		}
		if json.Unmarshal(lines[i], &entry) != nil {
			continue // first line of the tail window may be truncated
		}
		if entry.Type == "assistant" && entry.Message.Model != "" && entry.Message.Model != "<synthetic>" {
			return entry.Message.Model, info.ModTime()
		}
	}
	return "", time.Time{}
}

func mtime(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

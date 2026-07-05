package claude

import (
	"os"
	"path/filepath"
	"time"

	"github.com/kylesnowschwartz/agent-ouija/claude/claudedir"
	"github.com/kylesnowschwartz/agent-ouija/claude/registry"
	"github.com/kylesnowschwartz/agent-ouija/claude/transcript"

	"github.com/kylesnowschwartz/gearshifter/internal/agent"
)

// Session-specific gear state (M3 P3.5). Mechanisms verified against
// Claude Code 2.1.201 (see M3-DECK.md P3), now implemented by the shared
// agent-ouija library:
//
//	pane pid → descendant process tree → ~/.claude/sessions/<pid>.json
//	(registry.ResolvePane) → sessionId → transcript
//	~/.claude/projects/<enc-cwd>/<sessionId>.jsonl → last assistant
//	message.model (transcript.LastAssistantModel).
//
// Effort has no per-session disk source anywhere; it stays global.

// sessionState returns the gear state for the Claude session running in
// the target pane: session-specific model when resolvable, global
// settings otherwise; effort is always global (not persisted per session).
// The fresher file wins the model — settings.json updates the instant
// /model runs, transcripts only on the next assistant reply. That
// arbitration rule is gearshifter policy, not library behavior.
func sessionState(root claudedir.Root, panePID int, paneCwd string) agent.State {
	state := readSettings(root)
	entry, ok := registry.ResolvePane(root.SessionsDir(), panePID, paneCwd)
	if !ok {
		return state
	}
	transcriptPath := filepath.Join(root.ProjectDirFor(entry.Cwd), entry.SessionID+".jsonl")
	model, transcriptTime := transcript.LastAssistantModel(transcriptPath)
	if model == "" {
		return state
	}
	if transcriptTime.After(mtime(root.SettingsPath())) || state.Model == "" {
		state.Model = model
	}
	return state
}

func mtime(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

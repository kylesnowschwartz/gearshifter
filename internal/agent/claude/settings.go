package claude

import (
	"path/filepath"

	"github.com/kylesnowschwartz/agent-ouija/claude/settings"

	"github.com/kylesnowschwartz/gearshifter/internal/agent"
)

// readSettings answers V7 (M3-DECK.md P3 spike, 2026-07-05): /model,
// /effort, and /output-style persist to ~/.claude/settings.json (keys
// model, effortLevel, outputStyle) the moment they change, so the deck's
// gears read Claude's truth instead of sniffing panes or trusting user
// statuslines. Any failure degrades to the zero value — gears render
// stateless, never error.
//
// Decoding is delegated to agent-ouija's settings package, which reads
// ONLY these fields and never logs the raw content (the file also holds
// sensitive values like env).
func readSettings(home string) agent.State {
	s := settings.Read(filepath.Join(home, ".claude", "settings.json"))
	return agent.State{Model: s.Model, Effort: s.Effort, Style: s.Style}
}

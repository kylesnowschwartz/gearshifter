package claude

import (
	"github.com/kylesnowschwartz/agent-ouija/claude/claudedir"
	"github.com/kylesnowschwartz/agent-ouija/claude/settings"

	"github.com/kylesnowschwartz/gearshifter/internal/agent"
)

// readSettings answers V7 (M3-DECK.md P3 spike, 2026-07-05): /model,
// /effort, and /output-style persist to ~/.claude/settings.json (keys
// model, effortLevel, outputStyle) the moment they change, so the deck's
// gears read Claude's truth instead of sniffing panes or trusting user
// statuslines. Any failure degrades to the zero value — gears render
// stateless, never error. settings.Read owns the secrets-safe decoding.
//
// The path comes from root.SettingsPath() — the same derivation
// sessionState stats for mtime arbitration, so the file read and the file
// arbitrated can never diverge.
func readSettings(root claudedir.Root) agent.State {
	s := settings.Read(root.SettingsPath())
	return agent.State{Model: s.Model, Effort: s.Effort, Style: s.Style}
}

// Package claude implements agent.Provider for Claude Code by reading
// its on-disk state: global settings (settings.go), plus the
// live-session registry and transcripts for session-specific model
// resolution (session.go).
package claude

import "github.com/kylesnowschwartz/gearshifter/internal/agent"

// Claude resolves gear state from a Claude Code home directory
// (~/.claude lives under Home).
type Claude struct {
	Home string
}

var _ agent.Provider = Claude{}

// New returns a provider rooted at home.
func New(home string) Claude { return Claude{Home: home} }

// State implements agent.Provider: session-specific model when the
// pane's Claude session is resolvable, global settings otherwise.
func (c Claude) State(panePID int, paneCwd string) agent.State {
	return sessionState(c.Home, panePID, paneCwd)
}

// Package claude implements agent.Provider for Claude Code by reading
// its on-disk state: global settings (settings.go), plus the
// live-session registry and transcripts for session-specific model
// resolution (session.go).
package claude

import (
	"path/filepath"

	"github.com/kylesnowschwartz/agent-ouija/claude/claudedir"

	"github.com/kylesnowschwartz/gearshifter/internal/agent"
)

// Claude resolves gear state from a Claude Code state directory
// (~/.claude under the home passed to New). The Root is built once here
// so every path in this package derives from a single seam.
type Claude struct {
	root claudedir.Root
}

var _ agent.Provider = Claude{}

// New returns a provider rooted at home.
func New(home string) Claude {
	return Claude{root: claudedir.Root(filepath.Join(home, ".claude"))}
}

// State implements agent.Provider: session-specific model when the
// pane's Claude session is resolvable, global settings otherwise.
func (c Claude) State(panePID int, paneCwd string) agent.State {
	return sessionState(c.root, panePID, paneCwd)
}

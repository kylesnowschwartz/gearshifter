// Package agent is the seam between the deck and whatever coding agent
// runs in the target pane. The deck asks one question — "what is this
// session's live state?" — through Provider; internal/agent/claude
// answers it for Claude Code today, and a pi/codex provider (or the
// future shared session-state package) slots in behind the same
// interface with consumers untouched (M3 review, ARCHITECTURE.md §2).
package agent

// State is a session's live enum-command state — the values the deck's
// gears mark as current. Zero fields mean unknown; gears render
// stateless.
type State struct {
	Model  string
	Effort string
}

// Provider resolves the live State of the agent session running in the
// target pane, identified by the pane's shell pid and working directory.
// Implementations degrade to the zero State instead of erroring — the
// deck never blocks on state.
type Provider interface {
	State(panePID int, paneCwd string) State
}

// Package claude implements agent.Provider for Claude Code by reading
// its on-disk state: global settings (settings.go), plus the
// live-session registry and transcripts for session-specific model
// resolution (session.go).
//
// This is the only gearshifter package that imports agent-ouija
// (ARCHITECTURE.md §2 / CLAUDE.md "Shared library").
package claude

import (
	"github.com/kylesnowschwartz/agent-ouija/claude/claudedir"
	"github.com/kylesnowschwartz/agent-ouija/claude/registry"

	"github.com/kylesnowschwartz/gearshifter/internal/agent"
)

// Claude resolves gear state from a Claude Code state directory. Holding
// a claudedir.Root (not a home string) keeps every path in this package
// on one derivation.
type Claude struct {
	root claudedir.Root
}

var _ agent.Provider = Claude{}

// New returns a provider rooted at the conventional ~/.claude. An
// unresolvable home yields a zero-root provider whose State degrades to
// stateless gears (gear state is fail-open by design) — it never falls
// back to CWD-relative .claude reads.
func New() Claude {
	root, err := claudedir.DefaultRoot()
	if err != nil {
		return Claude{}
	}
	return Claude{root: root}
}

// NewAt returns a provider reading Claude Code state under root; tests
// point it at a sandbox directory.
func NewAt(root claudedir.Root) Claude {
	return Claude{root: root}
}

// State implements agent.Provider: session-specific model when the
// pane's Claude session is resolvable, global settings otherwise.
func (c Claude) State(panePID int, paneCwd string) agent.State {
	if c.root == "" {
		return agent.State{}
	}
	return sessionState(c.root, panePID, paneCwd)
}

// HasSession implements agent.Provider: true when the pane resolves to a
// live entry in Claude Code's sessions registry — the same resolution
// State uses for session-specific model, exposed as a yes/no so strip
// mode can pick its injection target.
func (c Claude) HasSession(panePID int, paneCwd string) bool {
	if c.root == "" {
		return false
	}
	_, ok := registry.ResolvePane(c.root.SessionsDir(), panePID, paneCwd)
	return ok
}

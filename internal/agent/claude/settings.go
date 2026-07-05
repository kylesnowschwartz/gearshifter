package claude

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/kylesnowschwartz/gearshifter/internal/agent"
)

// readSettings answers V7 (M3-DECK.md P3 spike, 2026-07-05): /model and
// /effort persist to ~/.claude/settings.json (keys model, effortLevel)
// the moment they change, so the deck's gears read Claude's truth instead
// of sniffing panes or trusting user statuslines. Any failure degrades to
// the zero value — gears render stateless, never error.
//
// The file also holds sensitive values (env); decode ONLY these fields and
// never log the raw content.
func readSettings(home string) agent.State {
	raw, err := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
	if err != nil {
		return agent.State{}
	}
	var s struct {
		Model  string `json:"model"`
		Effort string `json:"effortLevel"`
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		return agent.State{}
	}
	return agent.State{Model: s.Model, Effort: s.Effort}
}

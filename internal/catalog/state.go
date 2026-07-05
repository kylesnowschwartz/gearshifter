package catalog

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// GearState is the session's live enum-command state, read from Claude
// Code's own settings file.
type GearState struct {
	Model  string
	Effort string
}

// ReadGearState answers V7 (M3-DECK.md P3 spike, 2026-07-05): /model and
// /effort persist to ~/.claude/settings.json (keys model, effortLevel)
// the moment they change, so the deck's gears read Claude's truth instead
// of sniffing panes or trusting user statuslines. Any failure degrades to
// the zero value — gears render stateless, never error.
//
// The file also holds sensitive values (env); decode ONLY these fields and
// never log the raw content.
func ReadGearState(home string) GearState {
	raw, err := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
	if err != nil {
		return GearState{}
	}
	var s struct {
		Model  string `json:"model"`
		Effort string `json:"effortLevel"`
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		return GearState{}
	}
	return GearState{Model: s.Model, Effort: s.Effort}
}

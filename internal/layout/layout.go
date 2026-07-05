// Package layout places tiles on the deck grid: it is the one bridge
// that knows widget + deck + catalog + agent (ARCHITECTURE.md §2).
// Widgets know commands but not geometry, deck knows geometry but not
// commands; layout binds them into Placements the app renders and
// routes. P4's layout.toml parses into these same Placement structures.
package layout

import (
	"github.com/kylesnowschwartz/gearshifter/internal/agent"
	"github.com/kylesnowschwartz/gearshifter/internal/catalog"
	"github.com/kylesnowschwartz/gearshifter/internal/deck"
	"github.com/kylesnowschwartz/gearshifter/internal/widget"
)

// Placement puts a tile at a grid column and row. All geometry flows
// from deck.Grid and the Fibonacci scale — Samara's law: tiles are
// placed by the system, never nudged.
type Placement struct {
	Tile widget.Tile
	Col  int
	Y    int
}

// Vertical spacing from the Fibonacci scale (deck.Scale): tiles start
// under a 1-line header + 1-line gap; rows separate by the smallest step.
var (
	bodyY  = deck.Scale[1]
	rowGap = deck.Scale[0]
)

// buttonsPerRow splits the button field: 2×2 over deck.MainSpan.
const buttonsPerRow = 2

// entry pairs a tile with its start column before rows are derived.
type entry struct {
	tile widget.Tile
	col  int
}

// flow derives each tile's row, skyline-style: the first row sits at
// bodyY; a tile drops below the lowest earlier tile whose column range
// overlaps its own, plus rowGap. Rows are never authored — the default
// deck and layout.toml share this one algorithm (Samara's law in code).
func flow(entries []entry) []Placement {
	placements := make([]Placement, 0, len(entries))
	for _, e := range entries {
		span := e.tile.Span()
		y := bodyY
		for _, p := range placements {
			overlaps := e.col < p.Col+p.Tile.Span() && p.Col < e.col+span
			if bottom := p.Y + p.Tile.Rows() + rowGap; overlaps && bottom > y {
				y = bottom
			}
		}
		placements = append(placements, Placement{Tile: e.tile, Col: e.col, Y: y})
	}
	return placements
}

// Default builds the default deck: gear rail (span 5, MODEL over EFFORT)
// beside a 2×2 button field (span 4 each) — the φ split — with the
// launcher as a full-width bottom bar. Placement order = reading order =
// the app's focus order. state marks each gear's live value (V7).
func Default(commands []catalog.Command, state agent.State) []Placement {
	model := widget.NewGear(findCommand(commands, "model"), "MODEL",
		[]string{"haiku", "sonnet", "opus", "fable"}, deck.RailSpan).
		WithCurrent(state.Model)
	effort := widget.NewGear(findCommand(commands, "effort"), "EFFORT",
		[]string{"low", "medium", "high", "max"}, deck.RailSpan).
		WithCurrent(state.Effort)
	entries := []entry{{model, 0}, {effort, 0}}

	buttonSpan := deck.MainSpan / buttonsPerRow
	for i, b := range []struct{ name, label string }{
		{"review", "REVIEW"},
		{"context", "CONTEXT"},
		{"compact", "COMPACT"},
		{"resume", "RESUME"},
	} {
		btn := widget.NewButton(findCommand(commands, b.name), b.label, buttonSpan)
		entries = append(entries, entry{btn, deck.RailSpan + (i%buttonsPerRow)*buttonSpan})
	}

	entries = append(entries, entry{widget.NewLauncher(len(commands), deck.Columns), 0})
	return flow(entries)
}

// findCommand looks a command up by name so tiles carry the real catalog
// entry (argument hints drive the hint-aware Enter policy). A missing
// name still yields a working tile for the bare command.
func findCommand(commands []catalog.Command, name string) catalog.Command {
	for _, c := range commands {
		if c.Name == name {
			return c
		}
	}
	return catalog.Command{Name: name}
}

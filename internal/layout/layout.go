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

// Default builds the default deck: gear rail (span 5, MODEL over EFFORT)
// beside a 2×2 button field (span 4 each) — the φ split — with the
// launcher as a full-width bottom bar. Placement order = reading order =
// the app's focus order. state marks each gear's live value (V7).
func Default(commands []catalog.Command, state agent.State) []Placement {
	var placements []Placement
	add := func(t widget.Tile, col, y int) {
		placements = append(placements, Placement{Tile: t, Col: col, Y: y})
	}

	model := widget.NewGear(findCommand(commands, "model"), "MODEL",
		[]string{"haiku", "sonnet", "opus", "fable"}, deck.RailSpan).
		WithCurrent(state.Model)
	effort := widget.NewGear(findCommand(commands, "effort"), "EFFORT",
		[]string{"low", "medium", "high", "max"}, deck.RailSpan).
		WithCurrent(state.Effort)
	add(model, 0, bodyY)
	add(effort, 0, bodyY+model.Rows()+rowGap)

	buttonSpan := deck.MainSpan / buttonsPerRow
	for i, b := range []struct{ name, label string }{
		{"review", "REVIEW"},
		{"context", "CONTEXT"},
		{"compact", "COMPACT"},
		{"resume", "RESUME"},
	} {
		btn := widget.NewButton(findCommand(commands, b.name), b.label, buttonSpan)
		add(btn, deck.RailSpan+(i%buttonsPerRow)*buttonSpan, bodyY+(i/buttonsPerRow)*(btn.Rows()+rowGap))
	}

	railH := model.Rows() + rowGap + effort.Rows()
	add(widget.NewLauncher(len(commands), deck.Columns), 0, bodyY+railH+rowGap)
	return placements
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

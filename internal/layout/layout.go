// Package layout places tiles on the deck grid: it is the one bridge
// that knows widget + deck + catalog + agent (ARCHITECTURE.md §2).
// Widgets know commands but not geometry, deck knows geometry but not
// commands; layout binds them into Placements the app renders and
// routes. P4's layout.toml parses into these same Placement structures.
package layout

import (
	"sort"

	"github.com/kylesnowschwartz/gearshifter/internal/agent"
	"github.com/kylesnowschwartz/gearshifter/internal/catalog"
	"github.com/kylesnowschwartz/gearshifter/internal/deck"
	"github.com/kylesnowschwartz/gearshifter/internal/theme"
	"github.com/kylesnowschwartz/gearshifter/internal/widget"
)

// Sort selects how Default's button field orders itself. The zero value,
// SortNone, keeps the data-ranked reading order (DECK-CONTENT.md); it is
// not user-choosable data, so only Default (not Load) takes a Sort — a
// layout.toml already authors its own tile order.
type Sort string

const (
	SortNone  Sort = ""
	SortAlpha Sort = "alpha"
)

// Placement puts a tile at a grid column and row. All geometry flows
// from deck.Grid and the Fibonacci scale — Samara's law: tiles are
// placed by the system, never nudged. Glyph is the tile's authored
// compact-chip glyph (layout.toml `glyph =`); empty means the theme
// table decides. In compact mode Col/Y are zero AND MEANINGLESS — the
// flow packs by order alone; never read them there.
type Placement struct {
	Tile  widget.Tile
	Col   int
	Y     int
	Glyph string
}

// Vertical spacing from the Fibonacci scale (deck.Scale): tiles start
// under a 1-line header + 1-line gap; rows separate by the smallest step.
var (
	bodyY  = deck.Scale[1]
	rowGap = deck.Scale[0]
)

// buttonsPerRow splits the button field: 4 across over deck.MainSpan
// (8 = 4 × span-2, the main field's even split). Flipped from 2 on
// 2026-07-05 after QA on the dense demo (examples/dense.toml).
const buttonsPerRow = 4

// entry pairs a tile with its start column (and authored chip glyph)
// before rows are derived.
type entry struct {
	tile  widget.Tile
	col   int
	glyph string
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
		placements = append(placements, Placement{Tile: e.tile, Col: e.col, Y: y, Glyph: e.glyph})
	}
	return placements
}

// buttonSpec pairs a button's command with its label and insert behavior,
// before rows or columns are derived.
type buttonSpec struct {
	name, label string
	insert      bool
}

// sixPack is the data-ranked lead (DECK-CONTENT.md, 2026-07-05): ranked
// by usage, not alphabetized. It only holds that rank order under
// SortNone — SortAlpha treats the whole field as one alphabetized list.
var sixPack = []buttonSpec{
	{"compact", "COMPACT", false},
	{"copy", "COPY", false},
	{"clear", "CLEAR", false},
	{"context", "CONTEXT", false},
	{"resume", "RESUME", false},
	{"config", "CONFIG", false},
}

// fillers round out the 4×4 button field with generic built-ins.
var fillers = []buttonSpec{
	{"agents", "AGENTS", false},
	{"doctor", "DOCTOR", false},
	{"goal", "GOAL", true},
	{"mcp", "MCP", false},
	{"memory", "MEMORY", false},
	{"permissions", "PERMS", false},
	{"plugin", "PLUGIN", false},
	{"radio", "RADIO", false},
	{"reload-plugins", "RELOAD", false},
	{"rename", "RENAME", false},
}

// Default builds the default deck: gear rail (span 5, MODEL over EFFORT)
// beside a 4×4 button field (span 2 each) — the φ split — with the
// launcher as a full-width bottom bar. Buttons are generic built-ins:
// under SortNone the data-ranked six-pack leads in reading order and the
// rest fill the dense field (flipped from 2×3 the same day, a call made
// after the dense demo); under SortAlpha the whole field alphabetizes,
// six-pack included. Placement order = reading order = the app's focus
// order. state marks each gear's live value (V7); st styles every tile.
func Default(commands []catalog.Command, state agent.State, st *theme.Styles, how Sort) []Placement {
	model := widget.NewGear(st, findCommand(commands, "model"), "MODEL",
		[]string{"haiku", "sonnet", "opus", "fable"}, deck.RailSpan).
		WithCurrent(gearSetting("model", state))
	effort := widget.NewGear(st, findCommand(commands, "effort"), "EFFORT",
		[]string{"low", "medium", "high", "max"}, deck.RailSpan).
		WithCurrent(gearSetting("effort", state))
	entries := []entry{{tile: model}, {tile: effort}}

	buttons := append(append([]buttonSpec(nil), sixPack...), fillers...)
	if how == SortAlpha {
		sort.Slice(buttons, func(i, j int) bool { return buttons[i].label < buttons[j].label })
	}

	buttonSpan := deck.MainSpan / buttonsPerRow
	for i, b := range buttons {
		btn := widget.NewButton(st, findCommand(commands, b.name), b.label, buttonSpan)
		if b.insert {
			btn = btn.WithInsert()
		}
		entries = append(entries, entry{tile: btn, col: deck.RailSpan + (i%buttonsPerRow)*buttonSpan})
	}

	entries = append(entries, entry{tile: widget.NewLauncher(st, len(commands), deck.Columns), col: 0})
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

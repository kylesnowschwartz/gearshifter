package layout

import (
	"github.com/kylesnowschwartz/gearshifter/internal/agent"
	"github.com/kylesnowschwartz/gearshifter/internal/theme"
	"github.com/kylesnowschwartz/gearshifter/internal/widget"
)

// Compacted re-expresses a placed deck as the strip's chip flow
// (STRIP-EMBED step 2): every tile archetype maps to its one-row chip
// form, order preserved. Col/Y are dropped — the compact flow packs by
// natural width at render time, so a layout.toml authored for the grid
// compacts for free. Gear state is re-marked from the same state that
// built the placements (Gear keeps its current unexported).
func Compacted(placements []Placement, state agent.State, st *theme.Styles) []Placement {
	out := make([]Placement, 0, len(placements))
	for _, p := range placements {
		switch t := p.Tile.(type) {
		case widget.Button:
			out = append(out, Placement{Tile: widget.NewChip(st, t.Cmd, t.Label, theme.Glyph(t.Cmd.Name), t.Insert)})
		case widget.Gear:
			chip := widget.NewGearChip(st, t.Cmd, t.Label, t.Values).
				WithCurrent(gearSetting(t.Cmd.Name, state))
			out = append(out, Placement{Tile: chip})
		case widget.Launcher:
			out = append(out, Placement{Tile: widget.NewLauncherChip(st, t.Count)})
		default:
			out = append(out, Placement{Tile: p.Tile})
		}
	}
	return out
}

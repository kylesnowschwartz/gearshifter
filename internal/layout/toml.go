package layout

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"github.com/kylesnowschwartz/gearshifter/internal/agent"
	"github.com/kylesnowschwartz/gearshifter/internal/catalog"
	"github.com/kylesnowschwartz/gearshifter/internal/deck"
	"github.com/kylesnowschwartz/gearshifter/internal/widget"
)

// tomlTile is one [[tile]] entry. Span is a pointer so an absent field is
// distinguishable from an explicit 0 (which must error, not default).
type tomlTile struct {
	Type    string   `toml:"type"`
	Command string   `toml:"command"`
	Label   string   `toml:"label"`
	Values  []string `toml:"values"`
	Col     int      `toml:"col"`
	Span    *int     `toml:"span"`
}

type tomlLayout struct {
	Tiles []tomlTile `toml:"tile"`
}

// Load parses and validates a user layout.toml into placements (SPEC V4,
// M3 P4). Users author what ARCHITECTURE.md §6 allows — place/size/label
// tiles from the fixed archetype set — and never author rows: col + span
// go through the same skyline flow as the default deck. Every error names
// the offending line; a bad layout must fail with words, not a broken
// deck.
func Load(path string, commands []catalog.Command, state agent.State) ([]Placement, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("layout %s: %w", path, err)
	}
	dec := toml.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields() // typos are the common breakage; name them
	var doc tomlLayout
	if err := dec.Decode(&doc); err != nil {
		return nil, tomlError(path, err)
	}
	if len(doc.Tiles) == 0 {
		return nil, fmt.Errorf("%s: no [[tile]] entries — a layout needs at least one tile", path)
	}

	lines := tileLines(raw)
	entries := make([]entry, 0, len(doc.Tiles))
	for i, t := range doc.Tiles {
		tile, err := buildTile(t, commands, state)
		if err != nil {
			loc := path // inline `tile = [{…}]` arrays have no [[tile]] header line
			if i < len(lines) {
				loc = fmt.Sprintf("%s:%d", path, lines[i])
			}
			return nil, fmt.Errorf("%s: [[tile]] %d: %w", loc, i+1, err)
		}
		entries = append(entries, entry{tile: tile, col: t.Col})
	}
	return flow(entries), nil
}

// tomlError rewrites go-toml decode failures as file:line messages.
// Syntax and unknown-key errors carry their own position; everything else
// passes through with just the path.
func tomlError(path string, err error) error {
	var sme *toml.StrictMissingError
	if errors.As(err, &sme) && len(sme.Errors) > 0 {
		row, _ := sme.Errors[0].Position()
		return fmt.Errorf("%s:%d: unknown key %q (allowed: type, command, label, values, col, span)",
			path, row, strings.Join(sme.Errors[0].Key(), "."))
	}
	var de *toml.DecodeError
	if errors.As(err, &de) {
		row, _ := de.Position()
		return fmt.Errorf("%s:%d: %s", path, row, de.Error())
	}
	return fmt.Errorf("%s: %w", path, err)
}

// tileLines maps each [[tile]] entry, in document order, to its 1-based
// line number: go-toml reports positions only for syntax-level errors, so
// semantic validation names the entry's header line itself.
func tileLines(raw []byte) []int {
	var lines []int
	for i, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(strings.ReplaceAll(strings.TrimSpace(line), " ", ""), "[[tile]]") {
			lines = append(lines, i+1)
		}
	}
	return lines
}

// buildTile validates one entry and constructs its widget. The type
// vocabulary is the archetype set (ARCHITECTURE.md §7): config type=
// matches the widget type names, and users don't script new ones.
func buildTile(t tomlTile, commands []catalog.Command, state agent.State) (widget.Tile, error) {
	switch t.Type {
	case "button":
		name, err := commandName(t)
		if err != nil {
			return nil, err
		}
		span, err := spanOf(t, deck.MainSpan/buttonsPerRow)
		if err != nil {
			return nil, err
		}
		return widget.NewButton(findCommand(commands, name), labelOf(t, name), span), nil
	case "gear":
		name, err := commandName(t)
		if err != nil {
			return nil, err
		}
		if len(t.Values) == 0 {
			return nil, fmt.Errorf(`gear needs values = ["…", …] — the enum states it shifts between`)
		}
		span, err := spanOf(t, deck.RailSpan)
		if err != nil {
			return nil, err
		}
		g := widget.NewGear(findCommand(commands, name), labelOf(t, name), t.Values, span)
		return g.WithCurrent(gearSetting(name, state)), nil
	case "launcher":
		span, err := spanOf(t, deck.Columns)
		if err != nil {
			return nil, err
		}
		return widget.NewLauncher(len(commands), span), nil
	case "":
		return nil, fmt.Errorf(`missing type — every tile needs type = "button" | "gear" | "launcher"`)
	default:
		return nil, fmt.Errorf(`unknown type %q (archetypes: "button", "gear", "launcher")`, t.Type)
	}
}

// commandName requires and normalizes the command field; a leading slash
// is tolerated ("/model" and "model" both work).
func commandName(t tomlTile) (string, error) {
	name := strings.TrimPrefix(t.Command, "/")
	if name == "" {
		return "", fmt.Errorf(`%s needs command = "<slash command name>"`, t.Type)
	}
	return name, nil
}

// labelOf defaults the tile label to the uppercased command name.
func labelOf(t tomlTile, name string) string {
	if t.Label != "" {
		return t.Label
	}
	return strings.ToUpper(name)
}

// spanOf validates col/span against the fixed 13-column grid, defaulting
// span to the archetype's natural width.
func spanOf(t tomlTile, defaultSpan int) (int, error) {
	span := defaultSpan
	if t.Span != nil {
		span = *t.Span
	}
	if span < 1 {
		return 0, fmt.Errorf("span %d must be at least 1", span)
	}
	if t.Col < 0 || t.Col >= deck.Columns {
		return 0, fmt.Errorf("col %d out of range 0–%d", t.Col, deck.Columns-1)
	}
	if t.Col+span > deck.Columns {
		return 0, fmt.Errorf("col %d + span %d overflows the %d-column grid", t.Col, span, deck.Columns)
	}
	return span, nil
}

// gearSetting maps a gear's command to the session state that marks its
// current value — the ONE binding site between command names and
// agent.State fields (Default and Load both route through it). Gears for
// other enum commands render stateless — honest until an agent.Provider
// learns their state.
func gearSetting(name string, state agent.State) string {
	switch name {
	case "model":
		return state.Model
	case "effort":
		return state.Effort
	}
	return ""
}

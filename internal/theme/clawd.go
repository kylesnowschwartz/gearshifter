package theme

// Clawd is the Claude Code mascot — an Anthropic mark, rendered here
// with acknowledgment (see README credits); the pixel grid comes from
// the clawd-icon project and ships embedded, per the sprite decisions
// in CLAUDE.md (half-block ANSI, go:embed, mascot/flourish only).

import (
	_ "embed"
	"encoding/json"
	"image/color"
	"strings"
	"sync"

	"charm.land/lipgloss/v2"
)

//go:embed clawd_grid.json
var clawdGridJSON []byte

// clawdGrid parses the embedded 16×5 role grid once. Cells name palette
// roles ("body", "eye", "bg"), never colors — themes decide color.
var clawdGrid = sync.OnceValue(func() [][]string {
	var grid [][]string
	if err := json.Unmarshal(clawdGridJSON, &grid); err != nil {
		panic("theme: embedded clawd_grid.json is invalid: " + err.Error())
	}
	return grid
})

// MascotWidth is the sprite's width in cells — both variants render the
// full 16-column grid, one cell per pixel.
const MascotWidth = 16

// ClawdGlyph is clawd as a single cell: the clawd-icon font's code
// point (Plane 16 PUA — collides with no real font). It renders tofu
// unless Clawd.ttf is installed, which is why the footer glyph is
// opt-in (--mascot-glyph) rather than a default.
const ClawdGlyph = "\U00100CC0"

// Mascot renders the clawd sprite sized to the rows available: the full
// 5-row block when it fits, the 3-row half-block mini below that, nil
// when even the mini can't fit — the mascot retracts before it crowds
// (P3 k9s recipe). nil MascotBody (plain) means no mascot at all: the
// sprite is pure decoration and plain is the reduced-decoration path.
func (s *Styles) Mascot(maxRows int) []string {
	grid := clawdGrid()
	full, mini := len(grid), (len(grid)+1)/2
	body := s.Chrome.MascotBody
	if body == nil || maxRows < mini {
		return nil
	}
	eye := s.Chrome.MascotEye
	if maxRows >= full {
		return renderClawdFull(grid, lipgloss.NewStyle().Foreground(body), lipgloss.NewStyle().Foreground(eye))
	}
	return renderClawdMini(grid, body, eye)
}

// renderClawdFull paints one terminal cell per grid pixel — the grid
// was authored for cell-sized pixels, so this is its native resolution.
func renderClawdFull(grid [][]string, body, eye lipgloss.Style) []string {
	rows := make([]string, 0, len(grid))
	for _, gridRow := range grid {
		var row strings.Builder
		for _, cell := range gridRow {
			switch cell {
			case "body":
				row.WriteString(body.Render("█"))
			case "eye":
				row.WriteString(eye.Render("█"))
			default:
				row.WriteString(" ")
			}
		}
		rows = append(rows, row.String())
	}
	return rows
}

// renderClawdMini pairs grid rows into half-block cells (▀ fg=top px,
// bg=bottom px — the locked sprite physics), halving the height to a
// 3-row flourish for short canvases like a strip pane.
func renderClawdMini(grid [][]string, body, eye color.Color) []string {
	pick := func(role string) color.Color {
		switch role {
		case "body":
			return body
		case "eye":
			return eye
		}
		return nil
	}
	rows := make([]string, 0, (len(grid)+1)/2)
	for y := 0; y < len(grid); y += 2 {
		var row strings.Builder
		for x, cell := range grid[y] {
			top := pick(cell)
			var bot color.Color
			if y+1 < len(grid) {
				bot = pick(grid[y+1][x])
			}
			switch {
			case top != nil && bot != nil:
				row.WriteString(lipgloss.NewStyle().Foreground(top).Background(bot).Render("▀"))
			case top != nil:
				row.WriteString(lipgloss.NewStyle().Foreground(top).Render("▀"))
			case bot != nil:
				row.WriteString(lipgloss.NewStyle().Foreground(bot).Render("▄"))
			default:
				row.WriteString(" ")
			}
		}
		rows = append(rows, row.String())
	}
	return rows
}

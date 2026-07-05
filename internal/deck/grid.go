// Package deck is the grid layout engine: one 13-column Fibonacci grid
// (Samara's law — one system everywhere, then break it once; 13 is
// Fibonacci so the natural splits 5:8 and 8:5 approximate φ). Tiles only
// ever receive system-computed geometry, so ad-hoc nudging is impossible
// by construction. deck knows layout, not commands (ARCHITECTURE.md §2).
package deck

// Scale is the spacing vocabulary in cells: every gap relates to every
// other by ≈φ. Gutters, margins, and tile heights draw from this scale,
// never from per-element judgment.
var Scale = [...]int{1, 2, 3, 5, 8, 13, 21}

// Columns is fixed: 13 columns, Fibonacci.
const Columns = 13

// RailSpan and MainSpan are the deck's φ split: the gear rail against the
// button field (5:8 ≈ 0.625 ≈ 1/φ).
const (
	RailSpan = 5
	MainSpan = 8
)

// Grid maps column spans to cell geometry for a given canvas width.
type Grid struct {
	gutter int
	cols   []int // per-column cell widths
}

// New builds the 13-column grid for a canvas width. The gutter is the
// smallest step of the scale — popup canvases are narrow.
func New(width int) Grid {
	g := Grid{gutter: Scale[0]}
	usable := width - g.gutter*(Columns-1)
	if usable < Columns {
		usable = Columns // degenerate floor: 1 cell per column
	}
	g.cols = make([]int, Columns)
	// Bresenham spread: remainder cells distribute evenly across the grid
	// (leftmost-first would fatten the rail and skew the φ split) —
	// deterministic, system-driven, no visual judgment involved.
	for i := range g.cols {
		g.cols[i] = (i+1)*usable/Columns - i*usable/Columns
	}
	return g
}

// Cell returns the x offset and cell width of a tile starting at column
// col (0-based) and spanning span columns. Spans are clamped to the grid.
func (g Grid) Cell(col, span int) (x, w int) {
	for i := 0; i < col && i < Columns; i++ {
		x += g.cols[i] + g.gutter
	}
	end := col + span
	if end > Columns {
		end = Columns
	}
	for i := col; i < end; i++ {
		w += g.cols[i]
	}
	if n := end - col; n > 1 {
		w += g.gutter * (n - 1)
	}
	return x, w
}

// MinWidth is the narrowest canvas the grid renders legibly on: two cells
// per column (a border needs a neighbor) plus the gutters between them.
func MinWidth() int { return Columns*2 + (Columns-1)*Scale[0] }

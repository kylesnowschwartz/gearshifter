package deck

import "testing"

func TestFullSpanCoversCanvas(t *testing.T) {
	for _, width := range []int{54, 82, 131} {
		g := New(width)
		x, w := g.Cell(0, Columns)
		if x != 0 || w != width {
			t.Errorf("width %d: Cell(0,13) = (%d,%d), want (0,%d)", width, x, w, width)
		}
	}
}

func TestPhiSplitTiles(t *testing.T) {
	g := New(82)
	_, railW := g.Cell(0, RailSpan)
	mainX, mainW := g.Cell(RailSpan, MainSpan)
	if railW+1+mainW != 82 { // rail + gutter + main fills the canvas
		t.Errorf("rail %d + gutter + main %d != 82", railW, mainW)
	}
	if mainX != railW+1 {
		t.Errorf("main starts at %d, want %d (rail edge + gutter)", mainX, railW+1)
	}
	// 5:8 ≈ φ — allow the integer-cell wobble.
	ratio := float64(railW) / float64(mainW)
	if ratio < 0.55 || ratio > 0.70 {
		t.Errorf("rail:main = %.3f, want ≈0.625 (5:8)", ratio)
	}
}

func TestAdjacentSpansTile(t *testing.T) {
	g := New(82)
	x1, w1 := g.Cell(5, 4)
	x2, _ := g.Cell(9, 4)
	if x2 != x1+w1+1 {
		t.Errorf("col 9 starts at %d, want %d (col 5 span 4 edge + gutter)", x2, x1+w1+1)
	}
}

func TestSpanClampedToGrid(t *testing.T) {
	g := New(82)
	_, w := g.Cell(9, 99)
	_, want := g.Cell(9, 4)
	if w != want {
		t.Errorf("overlong span width %d, want clamped %d", w, want)
	}
}

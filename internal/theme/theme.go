// Package theme is the deck's one color seam: a role palette (semantic
// color names) derives every lipgloss.Style the UI renders — the Crush
// two-layer registry pattern (TUI-AESTHETICS.md). Color literals live
// only in this package's palette constructors; widgets receive *Styles
// at construction and never build styles themselves. theme is a leaf:
// it imports nothing of ours (ARCHITECTURE.md §2).
package theme

import (
	"fmt"
	"image/color"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"
)

// Palette is the role layer: every color the UI may use, by semantic
// role. Hierarchy comes from the fg/bg ramps (base → subtle → muted),
// not one-off values; the bg ramp and OnAccent are reserved for the P2
// armed/fill states.
type Palette struct {
	FgBase   color.Color // primary text
	FgSubtle color.Color // secondary text (gear values)
	FgMuted  color.Color // hints, sublabels, footer

	BgBase    color.Color
	BgRaised  color.Color
	BgHighest color.Color

	Accent   color.Color // the house accent (focus, wordmark)
	OnAccent color.Color // text on accent fills

	Border      color.Color // idle tile chrome
	BorderFocus color.Color // focused tile chrome
	Mark        color.Color // gear current-value row
	Danger      color.Color
}

// Styles is the derived layer: every style the UI renders, grouped per
// widget. Zero raw colors here or below — change the Palette, the whole
// deck follows.
type Styles struct {
	Button   ButtonStyles
	Gear     GearStyles
	Launcher LauncherStyles
	Chrome   ChromeStyles
	List     ListStyles
}

// ButtonStyles renders a button tile: bordered box, centered label,
// dim /command sublabel.
type ButtonStyles struct {
	Box        lipgloss.Style
	BoxFocus   lipgloss.Style
	Label      lipgloss.Style
	LabelFocus lipgloss.Style
	Sub        lipgloss.Style
}

// GearStyles renders a gear tile: hand-rolled frame (title embedded in
// the top border) over one row per value. Rows are styled once each —
// nested styles reset ANSI mid-row (M2 gotcha).
type GearStyles struct {
	Frame        lipgloss.Style // border chars, idle
	FrameFocus   lipgloss.Style // border chars, focused
	Value        lipgloss.Style // plain value row
	ValueCursor  lipgloss.Style // j/k cursor row
	ValueCurrent lipgloss.Style // the session's live value (▐ mark row)
}

// LauncherStyles renders the full-width launcher bar.
type LauncherStyles struct {
	Box        lipgloss.Style
	BoxFocus   lipgloss.Style
	Label      lipgloss.Style
	LabelFocus lipgloss.Style
	Count      lipgloss.Style
}

// ChromeStyles renders the app shell around the tiles.
type ChromeStyles struct {
	Wordmark lipgloss.Style
	Footer   lipgloss.Style
	Degraded lipgloss.Style // the canvas-too-small message
}

// ListStyles renders the palette screen (telescope + embedded).
type ListStyles struct {
	Prompt lipgloss.Style
	Cursor lipgloss.Style
	Hint   lipgloss.Style
	Badge  lipgloss.Style
}

// New derives the full style registry from a palette. Attribute
// semantics (bold current value, reversed cursor/focus, border charset
// swap on focus) are fixed here; palettes decide only color.
func New(p Palette) *Styles {
	fgBase := lipgloss.NewStyle().Foreground(p.FgBase)
	fgMuted := lipgloss.NewStyle().Foreground(p.FgMuted)
	box := lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(p.Border)
	boxFocus := lipgloss.NewStyle().Border(lipgloss.DoubleBorder()).BorderForeground(p.BorderFocus)
	labelFocus := lipgloss.NewStyle().Foreground(p.Accent).Reverse(true)
	return &Styles{
		Button: ButtonStyles{
			Box:        box,
			BoxFocus:   boxFocus,
			Label:      fgBase,
			LabelFocus: labelFocus,
			Sub:        fgMuted,
		},
		Gear: GearStyles{
			Frame:        lipgloss.NewStyle().Foreground(p.Border),
			FrameFocus:   lipgloss.NewStyle().Foreground(p.BorderFocus),
			Value:        lipgloss.NewStyle().Foreground(p.FgSubtle),
			ValueCursor:  lipgloss.NewStyle().Reverse(true),
			ValueCurrent: lipgloss.NewStyle().Bold(true).Foreground(p.Mark),
		},
		Launcher: LauncherStyles{
			Box:        box,
			BoxFocus:   boxFocus,
			Label:      fgBase,
			LabelFocus: labelFocus,
			Count:      fgMuted,
		},
		Chrome: ChromeStyles{
			Wordmark: lipgloss.NewStyle().Bold(true).Reverse(true).Foreground(p.Accent),
			Footer:   fgMuted,
			Degraded: fgMuted,
		},
		List: ListStyles{
			Prompt: lipgloss.NewStyle().Bold(true).Foreground(p.FgBase),
			Cursor: lipgloss.NewStyle().Reverse(true).Bold(true),
			Hint:   fgMuted,
			Badge:  fgMuted,
		},
	}
}

// Plain is the colorless registry: the exact attribute-only styles the
// deck rendered before the theme seam (bold/faint/reverse, uncolored
// borders). It is the behavior-freeze reference and the reduced-
// decoration path (TUI-AESTHETICS.md accessibility note).
func Plain() *Styles {
	faint := lipgloss.NewStyle().Faint(true)
	none := lipgloss.NewStyle()
	reversed := lipgloss.NewStyle().Reverse(true)
	return &Styles{
		Button: ButtonStyles{
			Box:        lipgloss.NewStyle().Border(lipgloss.NormalBorder()),
			BoxFocus:   lipgloss.NewStyle().Border(lipgloss.DoubleBorder()),
			Label:      none,
			LabelFocus: reversed,
			Sub:        faint,
		},
		Gear: GearStyles{
			Frame:        none,
			FrameFocus:   none,
			Value:        none,
			ValueCursor:  reversed,
			ValueCurrent: lipgloss.NewStyle().Bold(true),
		},
		Launcher: LauncherStyles{
			Box:        lipgloss.NewStyle().Border(lipgloss.NormalBorder()),
			BoxFocus:   lipgloss.NewStyle().Border(lipgloss.DoubleBorder()),
			Label:      none,
			LabelFocus: reversed,
			Count:      faint,
		},
		Chrome: ChromeStyles{
			Wordmark: lipgloss.NewStyle().Bold(true).Reverse(true),
			Footer:   faint,
			Degraded: faint,
		},
		List: ListStyles{
			Prompt: lipgloss.NewStyle().Bold(true),
			Cursor: lipgloss.NewStyle().Reverse(true).Bold(true),
			Hint:   faint,
			Badge:  faint,
		},
	}
}

// placeholder is the P1 stand-in palette: neutral charcoal ramps plus
// the clawd-orange accent. Suitable, not decided — the house palette is
// M5 P5's business (M5-AESTHETIC.md); these values exist so the P2
// layout work has honest contrast to test against.
var placeholder = Palette{
	FgBase:   lipgloss.Color("#E6E6EC"),
	FgSubtle: lipgloss.Color("#B4B4C0"),
	FgMuted:  lipgloss.Color("#78788A"),

	BgBase:    lipgloss.Color("#17171E"),
	BgRaised:  lipgloss.Color("#23232C"),
	BgHighest: lipgloss.Color("#32323E"),

	Accent:   lipgloss.Color("#D97757"),
	OnAccent: lipgloss.Color("#17171E"),

	Border:      lipgloss.Color("#4B4B5A"),
	BorderFocus: lipgloss.Color("#D97757"),
	Mark:        lipgloss.Color("#D97757"),
	Danger:      lipgloss.Color("#E0565F"),
}

// themes maps --theme / @gearshifter-theme names to registries.
var themes = map[string]func() *Styles{
	"default": func() *Styles { return New(placeholder) },
	"plain":   Plain,
}

// Load resolves a theme by name, failing with the available names — a
// typo in @gearshifter-theme must fail with words (M2 lesson).
func Load(name string) (*Styles, error) {
	build, ok := themes[name]
	if !ok {
		names := make([]string, 0, len(themes))
		for n := range themes {
			names = append(names, n)
		}
		sort.Strings(names)
		return nil, fmt.Errorf("unknown theme %q (available: %s)", name, strings.Join(names, ", "))
	}
	return build(), nil
}

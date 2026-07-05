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

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Curated single-width glyphs, defined next to the styles (Crush
// icon-table pattern, TUI-AESTHETICS.md §8): one place to see the
// deck's entire ornament vocabulary.
const (
	MarkCurrent = "▐ " // gear row: the session's live value
	MarkBlank   = "  " // gear row: everything else
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

	Accent    color.Color // the house accent (focus, wordmark)
	AccentAlt color.Color // the wordmark gradient's far stop (P4)
	OnAccent  color.Color // text on accent fills

	Border      color.Color // idle tile chrome
	BorderFocus color.Color // focused tile chrome
	Mark        color.Color // gear current-value row
	Danger      color.Color
}

// Styles is the derived layer: every style the UI renders, grouped per
// widget. Zero raw colors here or below — change the Palette, the whole
// deck follows.
type Styles struct {
	// Background/Foreground paint the popup surface: colored themes own
	// their canvas instead of inheriting the terminal's (the deck was
	// designed on BgBase; on a light terminal FgBase text vanishes —
	// posting/superfile lesson, TUI-AESTHETICS.md §1). nil = terminal
	// default (plain, which is attribute-only and adapts anywhere).
	Background color.Color
	Foreground color.Color

	// Armed is the ~150ms press-frame flash between fire and popup close
	// (P2): one role, shared by every tile that fires.
	Armed lipgloss.Style

	Button   ButtonStyles
	Gear     GearStyles
	Launcher LauncherStyles
	Chrome   ChromeStyles
	List     ListStyles
}

// FrameStyles is the chrome quartet every framed tile shares: border
// charset + border-char style, idle and focused. The charset is a theme
// decision: colored themes signal focus by border color alone (the
// unanimous idiom, TUI-AESTHETICS.md §4); plain has no color to spend,
// so it swaps to the double charset instead.
type FrameStyles struct {
	Border      lipgloss.Border // frame charset, idle
	BorderFocus lipgloss.Border // frame charset, focused
	Frame       lipgloss.Style  // border chars, idle
	FrameFocus  lipgloss.Style  // border chars, focused
}

// ButtonStyles renders a button tile: one big centered label with the
// /command spliced into the bottom border as a nameplate (superfile's
// border-embedded info slot — promoted from experiment 2026-07-05).
type ButtonStyles struct {
	FrameStyles
	Label      lipgloss.Style
	LabelFocus lipgloss.Style
	Sub        lipgloss.Style // the nameplate text
}

// GearStyles renders a gear tile: framed title over one row per value.
// Rows are styled once each — nested styles reset ANSI mid-row (M2
// gotcha).
type GearStyles struct {
	FrameStyles
	Value        lipgloss.Style // plain value row
	ValueCursor  lipgloss.Style // j/k cursor row
	ValueCurrent lipgloss.Style // the session's live value (▐ mark row)
}

// LauncherStyles renders the full-width launcher bar.
type LauncherStyles struct {
	FrameStyles
	Label      lipgloss.Style
	LabelFocus lipgloss.Style
	Count      lipgloss.Style
}

// ChromeStyles renders the app shell around the tiles. WordmarkBlend
// holds the stops of the deck's ONE gradient (P4, Crush recipe —
// TUI-AESTHETICS.md §8): BlendForeground sweeps them across the
// wordmark; nil means no gradient (plain).
type ChromeStyles struct {
	Wordmark      lipgloss.Style
	WordmarkBlend []color.Color
	Footer        lipgloss.Style
	Degraded      lipgloss.Style // the canvas-too-small message
}

// ListStyles renders the palette screen (telescope + embedded).
type ListStyles struct {
	Prompt lipgloss.Style
	Cursor lipgloss.Style
	Hint   lipgloss.Style
	Badge  lipgloss.Style
}

// New derives the full style registry from a palette. Attribute
// semantics (bold current value, reversed cursor, color-only focus,
// bg-fill armed frame) are fixed here; palettes decide only color.
func New(p Palette) *Styles {
	fgBase := lipgloss.NewStyle().Foreground(p.FgBase)
	fgMuted := lipgloss.NewStyle().Foreground(p.FgMuted)
	labelFocus := lipgloss.NewStyle().Foreground(p.Accent).Reverse(true)
	frame := FrameStyles{
		Border:      lipgloss.NormalBorder(),
		BorderFocus: lipgloss.NormalBorder(),
		Frame:       lipgloss.NewStyle().Foreground(p.Border),
		FrameFocus:  lipgloss.NewStyle().Foreground(p.BorderFocus),
	}
	return &Styles{
		Background: p.BgBase,
		Foreground: p.FgBase,
		Armed:      lipgloss.NewStyle().Bold(true).Foreground(p.OnAccent).Background(p.Accent),
		Button: ButtonStyles{
			FrameStyles: frame,
			Label:       fgBase,
			LabelFocus:  labelFocus,
			Sub:         fgMuted,
		},
		Gear: GearStyles{
			FrameStyles:  frame,
			Value:        lipgloss.NewStyle().Foreground(p.FgSubtle),
			ValueCursor:  lipgloss.NewStyle().Reverse(true),
			ValueCurrent: lipgloss.NewStyle().Bold(true).Foreground(p.Mark),
		},
		Launcher: LauncherStyles{
			FrameStyles: frame,
			Label:       fgBase,
			LabelFocus:  labelFocus,
			Count:       fgMuted,
		},
		Chrome: ChromeStyles{
			Wordmark:      lipgloss.NewStyle().Bold(true).Reverse(true).Foreground(p.Accent),
			WordmarkBlend: []color.Color{p.Accent, p.AccentAlt},
			Footer:        fgMuted,
			Degraded:      fgMuted,
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
// deck rendered before the theme seam (bold/faint/reverse), and the
// reduced-decoration path (TUI-AESTHETICS.md accessibility note). With
// no color to spend, focus keeps the double-border charset swap and
// armed is a bold reverse flash.
func Plain() *Styles {
	faint := lipgloss.NewStyle().Faint(true)
	none := lipgloss.NewStyle()
	reversed := lipgloss.NewStyle().Reverse(true)
	frame := FrameStyles{
		Border:      lipgloss.NormalBorder(),
		BorderFocus: lipgloss.DoubleBorder(),
		Frame:       none,
		FrameFocus:  none,
	}
	return &Styles{
		Armed: lipgloss.NewStyle().Reverse(true).Bold(true),
		Button: ButtonStyles{
			FrameStyles: frame,
			Label:       none,
			LabelFocus:  reversed,
			Sub:         faint,
		},
		Gear: GearStyles{
			FrameStyles:  frame,
			Value:        none,
			ValueCursor:  reversed,
			ValueCurrent: lipgloss.NewStyle().Bold(true),
		},
		Launcher: LauncherStyles{
			FrameStyles: frame,
			Label:       none,
			LabelFocus:  reversed,
			Count:       faint,
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

// ApplySurface stamps the theme's popup surface onto a view: colored
// themes own their canvas (nil = terminal default, the plain path) —
// without this, FgBase text vanishes on light terminals. Every screen's
// View must route through here so a swap never color-pops.
func (s *Styles) ApplySurface(v *tea.View) {
	v.BackgroundColor = s.Background
	v.ForegroundColor = s.Foreground
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

	Accent:    lipgloss.Color("#D97757"),
	AccentAlt: lipgloss.Color("#E8A87C"),
	OnAccent:  lipgloss.Color("#17171E"),

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

// BlendForeground renders s one rune at a time, sweeping the foreground
// through the gradient stops while keeping base's attributes — under
// Reverse this paints a gradient BLOCK (each cell's background takes
// its rune's color). The deck's one gradient (P4). Fewer than two stops
// (plain) renders base unchanged, byte-identical to base.Render.
func BlendForeground(s string, base lipgloss.Style, stops []color.Color) string {
	if len(stops) < 2 || s == "" {
		return base.Render(s)
	}
	runes := []rune(s)
	colors := lipgloss.Blend1D(len(runes), stops...)
	var b strings.Builder
	for i, r := range runes {
		b.WriteString(base.Foreground(colors[i]).Render(string(r)))
	}
	return b.String()
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

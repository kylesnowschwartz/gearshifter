package theme

import (
	"image/color"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestLoadKnownThemes(t *testing.T) {
	for _, name := range []string{"default", "plain"} {
		st, err := Load(name)
		if err != nil || st == nil {
			t.Errorf("Load(%q) = %v, %v; want a registry", name, st, err)
		}
	}
}

func TestLoadUnknownThemeFailsWithWords(t *testing.T) {
	_, err := Load("dmg")
	if err == nil || !strings.Contains(err.Error(), "available: default, plain") {
		t.Errorf("a theme typo must name the available themes, got %v", err)
	}
}

func TestPlainRendersNoColor(t *testing.T) {
	// plain is the behavior-freeze reference and the reduced-decoration
	// path: attribute-only, so its output carries no color sequence
	// (SGR 3x/4x/38/48).
	st := Plain()
	for name, s := range map[string]string{
		"footer": st.Chrome.Footer.Render("hint"),
		"sub":    st.Button.Sub.Render("/compact"),
		"value":  st.Gear.Value.Render("opus"),
	} {
		if strings.Contains(s, "[3") || strings.Contains(s, "[4") || strings.Contains(s, "[9") {
			t.Errorf("plain %s output must carry no color codes: %q", name, s)
		}
	}
}

func TestColoredThemesOwnTheSurfacePlainInherits(t *testing.T) {
	// Colored palettes are designed on their own bg ramp; inheriting the
	// terminal background makes FgBase text vanish on light terminals
	// (found live, 2026-07-05). plain must keep nil = terminal default.
	def, _ := Load("default")
	if def.Background == nil || def.Foreground == nil {
		t.Error("default theme must paint the popup surface")
	}
	if p := Plain(); p.Background != nil || p.Foreground != nil {
		t.Error("plain must inherit the terminal's surface")
	}
}

func TestBlendForegroundGradientAndPlainPassthrough(t *testing.T) {
	base := lipgloss.NewStyle().Bold(true).Reverse(true)
	// Plain path: fewer than two stops must be byte-identical to base.
	if got := BlendForeground("GEAR", base, nil); got != base.Render("GEAR") {
		t.Errorf("nil stops must render base unchanged, got %q", got)
	}
	// Gradient path: distinct stops give the first and last rune
	// different foregrounds, and geometry is untouched.
	stops := []color.Color{lipgloss.Color("#D97757"), lipgloss.Color("#E8A87C")}
	out := BlendForeground("GEARSHIFTER", base, stops)
	if lipgloss.Width(out) != len("GEARSHIFTER") {
		t.Errorf("gradient must not change width: %d", lipgloss.Width(out))
	}
	if !strings.Contains(out, "217;119;87") || !strings.Contains(out, "232;168;124") {
		t.Errorf("gradient must reach both stops:\n%q", out)
	}
}

func TestDefaultThemeColorsWithoutBreakingText(t *testing.T) {
	st, _ := Load("default")
	out := st.Gear.ValueCurrent.Render("▐ opus")
	if !strings.Contains(out, "▐ opus") {
		t.Errorf("styling must wrap the row, not rewrite it: %q", out)
	}
	if out == "▐ opus" {
		t.Error("default theme must actually style the current value row")
	}
}

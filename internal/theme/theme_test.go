package theme

import (
	"strings"
	"testing"
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

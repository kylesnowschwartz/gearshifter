package catalog

import "testing"

func TestParseClaudeVersion(t *testing.T) {
	cases := map[string]string{
		"2.1.200 (Claude Code)": "2.1.200",
		"2.1.200":               "2.1.200",
		"":                      "",
		"not a version":         "",
	}
	for in, want := range cases {
		if got := ParseClaudeVersion(in); got != want {
			t.Errorf("ParseClaudeVersion(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"2.1.200", "2.1.98", 1},
		{"2.1.98", "2.1.200", -1},
		{"2.1.200", "2.1.200", 0},
		{"2.1", "2.1.0", 0},
		{"2.1.0", "2.0.99", 1},
		{"10.0.0", "9.9.9", 1},
	}
	for _, c := range cases {
		if got := compareVersions(c.a, c.b); got != c.want {
			t.Errorf("compareVersions(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

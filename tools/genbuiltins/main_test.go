package main

import (
	"strings"
	"testing"
)

func TestParseRow(t *testing.T) {
	cases := []struct {
		name string
		line string
		want row
		ok   bool
	}{
		{
			name: "plain command with hint",
			line: "| `/add-dir <path>`   | Add a working directory |",
			want: row{name: "add-dir", hint: "<path>", desc: "Add a working directory"},
			ok:   true,
		},
		{
			name: "escaped pipe in hint, min-version annotation",
			line: `| ` + "`" + `/advisor [model\|off]` + "`" + ` | {/* min-version: 2.1.98 */}Enable the [advisor tool](/en/advisor). Accepts ` + "`opus`" + ` |`,
			want: row{name: "advisor", minVersion: "2.1.98", hint: "[model|off]", desc: "Enable the advisor tool. Accepts opus"},
			ok:   true,
		},
		{
			name: "bold and multiple annotations stripped",
			line: "| `/batch <instruction>` | **[Skill](/en/skills).** Orchestrate {/* max-version: 9.9.9 */}changes |",
			want: row{name: "batch", hint: "<instruction>", desc: "Skill. Orchestrate changes"},
			ok:   true,
		},
		{name: "prose line rejected", line: "Some prose with a | pipe in it", ok: false},
		{name: "table header rejected", line: "| Command | Description |", ok: false},
		{name: "separator rejected", line: "|---|---|", ok: false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := parseRow(c.line)
			if ok != c.ok {
				t.Fatalf("ok = %v, want %v", ok, c.ok)
			}
			if ok && got != c.want {
				t.Errorf("row = %+v\nwant  %+v", got, c.want)
			}
		})
	}
}

func TestParseDocUniqueness(t *testing.T) {
	doc := "| `/dup` | first |\n| `/dup` | second |\n"
	if _, err := parseDoc(doc); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("expected duplicate error, got %v", err)
	}
}

func TestParseDocEmpty(t *testing.T) {
	if _, err := parseDoc("no table here\n"); err == nil {
		t.Error("expected error for docs with no command rows")
	}
}

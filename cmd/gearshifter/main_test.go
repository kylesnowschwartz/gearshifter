package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kylesnowschwartz/gearshifter/internal/catalog"
)

func TestResolveLayout(t *testing.T) {
	tomlPath := filepath.Join(t.TempDir(), "my.toml")
	if err := os.WriteFile(tomlPath, []byte("[[tile]]\ntype = \"launcher\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name                  string
		wantInbuilt, wantPath string
		wantErr               bool
	}{
		{"telescope", "telescope", "", false},
		{"deck", "deck", "", false},
		{tomlPath, "", tomlPath, false},
		{"no-such-layout", "", "", true},
	} {
		inbuilt, path, err := resolveLayout(c.name)
		if (err != nil) != c.wantErr || inbuilt != c.wantInbuilt || path != c.wantPath {
			t.Errorf("resolveLayout(%q) = (%q, %q, %v), want (%q, %q, err=%v)",
				c.name, inbuilt, path, err, c.wantInbuilt, c.wantPath, c.wantErr)
		}
	}
}

func TestBuildInjection(t *testing.T) {
	cases := []struct {
		name        string
		pick        selection
		wantText    string
		wantNoEnter bool
	}{
		{"plain button", selection{cmd: catalog.Command{Name: "review"}}, "/review", false},
		{"gear value always enters", selection{cmd: catalog.Command{Name: "model"}, arg: "opus"}, "/model opus", false},
		{"required-arg inserts", selection{cmd: catalog.Command{Name: "btw", ArgumentHint: "<question>"}}, "/btw ", true},
		{"tab insert-only", selection{cmd: catalog.Command{Name: "context"}, insertOnly: true}, "/context ", true},
		{"gear beats insert-only", selection{cmd: catalog.Command{Name: "model"}, arg: "haiku", insertOnly: true}, "/model haiku", false},
	}
	for _, c := range cases {
		text, opts := buildInjection(c.pick)
		if text != c.wantText || opts.NoEnter != c.wantNoEnter {
			t.Errorf("%s: got (%q, NoEnter=%v), want (%q, NoEnter=%v)",
				c.name, text, opts.NoEnter, c.wantText, c.wantNoEnter)
		}
	}
}

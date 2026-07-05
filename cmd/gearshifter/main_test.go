package main

import (
	"testing"

	"github.com/kylesnowschwartz/gearshifter/internal/catalog"
)

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

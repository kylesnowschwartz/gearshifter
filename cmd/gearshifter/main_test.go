package main

import (
	"testing"

	"github.com/kylesnowschwartz/gearshifter/internal/catalog"
)

func TestBuildInjection(t *testing.T) {
	cases := []struct {
		name        string
		sel         catalog.Command
		arg         string
		insertOnly  bool
		wantText    string
		wantNoEnter bool
	}{
		{"plain button", catalog.Command{Name: "review"}, "", false, "/review", false},
		{"gear value always enters", catalog.Command{Name: "model"}, "opus", false, "/model opus", false},
		{"required-arg inserts", catalog.Command{Name: "btw", ArgumentHint: "<question>"}, "", false, "/btw ", true},
		{"tab insert-only", catalog.Command{Name: "context"}, "", true, "/context ", true},
		{"gear beats insert-only", catalog.Command{Name: "model"}, "haiku", true, "/model haiku", false},
	}
	for _, c := range cases {
		text, opts := buildInjection(c.sel, c.arg, c.insertOnly)
		if text != c.wantText || opts.NoEnter != c.wantNoEnter {
			t.Errorf("%s: got (%q, NoEnter=%v), want (%q, NoEnter=%v)",
				c.name, text, opts.NoEnter, c.wantText, c.wantNoEnter)
		}
	}
}

package catalog

import (
	"strings"
	"testing"
)

func TestParseFrontmatter(t *testing.T) {
	doc := `---
name: revdiff
description: "Review a diff: hunt bugs, style nits"
argument-hint: '[base-branch]'
metadata:
  type: skill
tags:
  - review
---
Body text: not frontmatter.
`
	fm := parseFrontmatter(strings.NewReader(doc))
	if fm["name"] != "revdiff" {
		t.Errorf("name = %q", fm["name"])
	}
	if fm["description"] != "Review a diff: hunt bugs, style nits" {
		t.Errorf("description = %q (quotes should be stripped, inner colon kept)", fm["description"])
	}
	if fm["argument-hint"] != "[base-branch]" {
		t.Errorf("argument-hint = %q", fm["argument-hint"])
	}
	if _, ok := fm["type"]; ok {
		t.Error("nested metadata leaked into top-level fields")
	}
	if _, ok := fm["body text"]; ok {
		t.Error("body content parsed as frontmatter")
	}
}

func TestParseFrontmatterMissing(t *testing.T) {
	fm := parseFrontmatter(strings.NewReader("# Just a heading\nno frontmatter here\n"))
	if len(fm) != 0 {
		t.Errorf("expected empty map, got %v", fm)
	}
}

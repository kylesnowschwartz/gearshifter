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
	if fm.Name != "revdiff" {
		t.Errorf("name = %q", fm.Name)
	}
	if fm.Description != "Review a diff: hunt bugs, style nits" {
		t.Errorf("description = %q (quotes should be stripped, inner colon kept)", fm.Description)
	}
	if fm.ArgumentHint != "[base-branch]" {
		t.Errorf("argument-hint = %q", fm.ArgumentHint)
	}
}

// Folded (>-) and literal (|) block scalars appear in real skills
// (plannotator-compound, ultracodex's codex-workflow) — the M1 review found
// the hand-rolled parser returned the literal string ">-" for these.
func TestParseFrontmatterBlockScalars(t *testing.T) {
	folded := `---
name: codex-workflow
description: >-
  Author and run a custom Workflow that BLENDS Codex headless nodes
  into otherwise-Claude orchestration — for cross-model verification.
---
`
	fm := parseFrontmatter(strings.NewReader(folded))
	want := "Author and run a custom Workflow that BLENDS Codex headless nodes into otherwise-Claude orchestration — for cross-model verification."
	if fm.Description != want {
		t.Errorf("folded description = %q\nwant %q", fm.Description, want)
	}

	literal := `---
description: |
  Line one keeps
  its line breaks in YAML
---
`
	fm = parseFrontmatter(strings.NewReader(literal))
	if fm.Description != "Line one keeps its line breaks in YAML" {
		t.Errorf("literal description not collapsed for TSV: %q", fm.Description)
	}
}

func TestParseFrontmatterNestedMetadata(t *testing.T) {
	doc := `---
name: gws
description: Git workspace sync
metadata:
  category: "productivity"
  requires:
    bins:
      - gws
---
`
	fm := parseFrontmatter(strings.NewReader(doc))
	if fm.Name != "gws" || fm.Description != "Git workspace sync" {
		t.Errorf("nested metadata broke scalar extraction: %+v", fm)
	}
}

func TestParseFrontmatterDegradation(t *testing.T) {
	if fm := parseFrontmatter(strings.NewReader("# Just a heading\nno frontmatter\n")); fm != (frontmatter{}) {
		t.Errorf("missing frontmatter: expected zero value, got %+v", fm)
	}
	malformed := "---\ndescription: [unclosed\n  bad: : yaml: here\n---\n"
	if fm := parseFrontmatter(strings.NewReader(malformed)); fm != (frontmatter{}) {
		t.Errorf("malformed yaml should degrade to zero value, got %+v", fm)
	}
}

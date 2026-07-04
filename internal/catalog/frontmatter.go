package catalog

import (
	"bufio"
	"io"
	"strings"

	"github.com/goccy/go-yaml"
)

// frontmatter holds the fields Gearshifter reads from a skill/command
// definition's YAML header. Everything else (metadata blocks, allowed-tools,
// hooks config) is intentionally ignored.
type frontmatter struct {
	Name         string `yaml:"name"`
	Description  string `yaml:"description"`
	ArgumentHint string `yaml:"argument-hint"`
}

// parseFrontmatter reads the leading YAML block (--- ... ---) and unmarshals
// it with a real YAML parser — skills in the wild use folded (>-) and literal
// (|) block scalars for descriptions, which line-based extraction corrupts.
// Malformed YAML degrades to empty fields; it never hides a command from the
// catalog.
func parseFrontmatter(r io.Reader) frontmatter {
	var fm frontmatter
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	if !sc.Scan() || strings.TrimSpace(sc.Text()) != "---" {
		return fm
	}
	var block strings.Builder
	for sc.Scan() {
		if strings.TrimSpace(sc.Text()) == "---" {
			break
		}
		block.WriteString(sc.Text())
		block.WriteByte('\n')
	}
	if err := yaml.Unmarshal([]byte(block.String()), &fm); err != nil {
		return frontmatter{}
	}
	// Catalog output is line-oriented TSV: collapse any newlines/tabs a
	// block scalar carried into single spaces.
	fm.Name = collapseWhitespace(fm.Name)
	fm.Description = collapseWhitespace(fm.Description)
	fm.ArgumentHint = collapseWhitespace(fm.ArgumentHint)
	return fm
}

func collapseWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

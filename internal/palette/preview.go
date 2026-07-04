package palette

import (
	"os"
	"path/filepath"
	"strings"

	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"

	"github.com/kylesnowschwartz/gearshifter/internal/catalog"
)

// minPreviewAt is the popup width below which the preview pane is dropped
// entirely — a cramped preview is worse than none.
const minPreviewAt = 80

var (
	previewPane = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, false, true).
			PaddingLeft(1)
	previewTitle = lipgloss.NewStyle().Bold(true)
	previewMeta  = lipgloss.NewStyle().Faint(true)
)

// previewWidth returns the columns given to the preview pane, 0 when the
// popup is too narrow for one.
func previewWidth(total int) int {
	if total < minPreviewAt {
		return 0
	}
	return total * 2 / 5
}

// renderPreview lays out the highlighted command's metadata plus, for
// file-backed commands, the glamour-rendered markdown body. Returns the full
// (unwindowed) preview as wrapped lines of the given content width.
func renderPreview(c catalog.Command, width int) []string {
	var b strings.Builder
	b.WriteString(previewTitle.Render("/"+c.Name) + "\n")
	if c.ArgumentHint != "" {
		b.WriteString(hintStyle.Render(c.ArgumentHint) + "\n")
	}
	meta := c.Source
	if c.MinVersion != "" {
		meta += " · requires ≥ " + c.MinVersion
	}
	b.WriteString(previewMeta.Render(meta) + "\n")
	if c.Path != "" {
		b.WriteString(previewMeta.Render(tildePath(c.Path)) + "\n")
	}
	if body := renderBody(c.Path, width); body != "" {
		b.WriteString("\n" + body)
	} else if c.Description != "" {
		b.WriteString("\n" + c.Description)
	}
	wrapped := lipgloss.NewStyle().Width(width).Render(b.String())
	return strings.Split(strings.TrimRight(wrapped, "\n"), "\n")
}

// renderBody reads a command's defining markdown file and renders it with
// glamour at the given width. Empty on any failure — the preview degrades to
// the plain description rather than erroring.
func renderBody(path string, width int) string {
	if path == "" {
		return ""
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return ""
	}
	out, err := r.Render(stripFrontmatter(string(raw)))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// stripFrontmatter drops a leading `---`-delimited YAML block; the metadata
// is already shown above the body.
func stripFrontmatter(s string) string {
	if !strings.HasPrefix(s, "---\n") {
		return s
	}
	rest := s[4:]
	if i := strings.Index(rest, "\n---\n"); i >= 0 {
		return rest[i+5:]
	}
	if i := strings.Index(rest, "\n---"); i >= 0 && strings.TrimSpace(rest[i+4:]) == "" {
		return ""
	}
	return s
}

// tildePath abbreviates the user's home directory to ~ for display.
func tildePath(p string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return p
	}
	if rel, err := filepath.Rel(home, p); err == nil && !strings.HasPrefix(rel, "..") {
		return "~/" + rel
	}
	return p
}

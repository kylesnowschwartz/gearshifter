package palette

import (
	"os"
	"path/filepath"
	"strings"

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

// renderPreview lays out the highlighted command's metadata in width columns
// (content width, excluding the pane border/padding).
func renderPreview(c catalog.Command, width, height int) string {
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
	if c.Description != "" {
		b.WriteString("\n" + c.Description)
	}
	return lipgloss.NewStyle().Width(width).MaxHeight(height).Render(b.String())
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

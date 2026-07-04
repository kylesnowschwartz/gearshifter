// Package palette is the searchable command list screen (M2). It is a plain
// Bubble Tea model with no tmux knowledge: it records the user's selection
// and quits; the caller performs the injection (ARCHITECTURE §2 rule 1).
// M3 embeds this same model behind the deck's Launcher tile.
package palette

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/kylesnowschwartz/gearshifter/internal/catalog"
)

var (
	cursorStyle = lipgloss.NewStyle().Reverse(true).Bold(true)
	hintStyle   = lipgloss.NewStyle().Faint(true)
	badgeStyle  = lipgloss.NewStyle().Faint(true)
)

// Model renders the command catalog as a scrollable list. Zero value is not
// usable; construct with New.
type Model struct {
	commands []catalog.Command
	cursor   int
	offset   int // first visible row
	width    int
	height   int
	selected *catalog.Command
}

// New returns a palette over the given commands, expected pre-sorted by
// catalog.Build.
func New(commands []catalog.Command) Model {
	return Model{commands: commands}
}

// Selection returns the command chosen with Enter, if any. Valid after the
// program has finished.
func (m Model) Selection() (catalog.Command, bool) {
	if m.selected == nil {
		return catalog.Command{}, false
	}
	return *m.selected, true
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc", "q", "ctrl+c":
			return m, tea.Quit
		case "enter":
			if len(m.commands) > 0 {
				c := m.commands[m.cursor]
				m.selected = &c
			}
			return m, tea.Quit
		case "j", "down":
			m.cursor++
		case "k", "up":
			m.cursor--
		case "ctrl+d":
			m.cursor += m.pageSize() / 2
		case "ctrl+u":
			m.cursor -= m.pageSize() / 2
		}
		m.clampCursor()
	}
	return m, nil
}

func (m Model) View() tea.View {
	var b strings.Builder
	last := min(m.offset+m.pageSize(), len(m.commands))
	for i := m.offset; i < last; i++ {
		b.WriteString(m.renderRow(i))
		b.WriteByte('\n')
	}
	fmt.Fprintf(&b, "%d commands · enter run · esc cancel", len(m.commands))
	v := tea.NewView(b.String())
	v.AltScreen = true
	return v
}

// renderRow lays out one command: /name + argument hint left, source badge
// right-aligned, truncated to the popup width.
func (m Model) renderRow(i int) string {
	c := m.commands[i]
	badge := "[" + c.Source + "]"
	left := "/" + c.Name
	if c.ArgumentHint != "" {
		left += " " + hintStyle.Render(c.ArgumentHint)
	}
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(badge) - 1
	if gap < 1 {
		left = lipgloss.NewStyle().MaxWidth(max(m.width-lipgloss.Width(badge)-2, 8)).Render(left)
		gap = max(m.width-lipgloss.Width(left)-lipgloss.Width(badge)-1, 1)
	}
	row := left + strings.Repeat(" ", gap) + badgeStyle.Render(badge)
	if i == m.cursor {
		return cursorStyle.Render(row)
	}
	return row
}

// pageSize is the list viewport height: everything except the status line.
func (m Model) pageSize() int {
	return max(m.height-1, 1)
}

// clampCursor bounds the cursor and scrolls the viewport to keep it visible.
func (m *Model) clampCursor() {
	m.cursor = max(0, min(m.cursor, len(m.commands)-1))
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+m.pageSize() {
		m.offset = m.cursor - m.pageSize() + 1
	}
}

// Package palette is the searchable command list screen (M2). It is a plain
// Bubble Tea model with no tmux knowledge: it records the user's selection
// and quits; the caller performs the injection (ARCHITECTURE §2 rule 1).
// M3 embeds this same model behind the deck's Launcher tile.
package palette

import (
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/kylesnowschwartz/gearshifter/internal/catalog"
)

// debugLog, when GEARSHIFTER_DEBUG is set, writes every incoming message to
// stderr — redirect it to a file when running inside display-popup, where
// stderr is otherwise invisible.
var debugEnabled = os.Getenv("GEARSHIFTER_DEBUG") != ""

func debugLog(msg tea.Msg) {
	if debugEnabled {
		fmt.Fprintf(os.Stderr, "msg %T %+v\n", msg, msg)
	}
}

var (
	cursorStyle = lipgloss.NewStyle().Reverse(true).Bold(true)
	hintStyle   = lipgloss.NewStyle().Faint(true)
	badgeStyle  = lipgloss.NewStyle().Faint(true)
	promptStyle = lipgloss.NewStyle().Bold(true)
)

// Model renders the command catalog as a filterable, scrollable list. Zero
// value is not usable; construct with New.
type Model struct {
	commands   []catalog.Command
	query      string
	visible    []int // indices into commands, ranked by filter match
	cursor     int   // position within visible
	offset     int   // first visible row (within visible)
	width      int
	height     int
	selected   *catalog.Command
	insertOnly bool // Tab: insert without pressing Enter
}

// New returns a palette over the given commands, expected pre-sorted by
// catalog.Build.
func New(commands []catalog.Command) Model {
	return Model{
		commands: commands,
		visible:  filterCommands(commands, ""),
	}
}

// Selection returns the command chosen with Enter/Tab/click, if any. Valid
// after the program has finished.
func (m Model) Selection() (catalog.Command, bool) {
	if m.selected == nil {
		return catalog.Command{}, false
	}
	return *m.selected, true
}

// InsertOnly reports whether the selection was made with Tab, requesting
// insert-without-Enter regardless of the command's argument hint.
func (m Model) InsertOnly() bool { return m.insertOnly }

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	debugLog(msg)
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	case tea.MouseClickMsg:
		// y=0 is the prompt line; list rows follow. Coordinates arrive
		// popup-local and border-adjusted (S1/P0), so no offset math.
		if msg.Button == tea.MouseLeft {
			if row := m.offset + msg.Y - 1; msg.Y >= 1 && row < len(m.visible) {
				c := m.commands[m.visible[row]]
				m.selected = &c
				return m, tea.Quit
			}
		}
	case tea.MouseWheelMsg:
		switch msg.Button {
		case tea.MouseWheelDown:
			m.cursor++
		case tea.MouseWheelUp:
			m.cursor--
		}
		m.clampCursor()
	}
	return m, nil
}

// handleKey routes keys: navigation and control chords always work; with an
// empty query the vim keys j/k/q keep their browse meanings; any other
// printable rune types into the fuzzy filter.
func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "esc", "ctrl+c":
		return m, tea.Quit
	case "enter", "tab":
		if len(m.visible) > 0 {
			c := m.commands[m.visible[m.cursor]]
			m.selected = &c
			m.insertOnly = key == "tab"
		}
		return m, tea.Quit
	case "down", "ctrl+n":
		m.cursor++
	case "up", "ctrl+p":
		m.cursor--
	case "ctrl+d":
		m.cursor += m.pageSize() / 2
	case "ctrl+u":
		m.cursor -= m.pageSize() / 2
	case "backspace":
		if m.query != "" {
			m.setQuery(m.query[:len(m.query)-1])
		}
	default:
		if m.query == "" {
			switch key {
			case "q":
				return m, tea.Quit
			case "j":
				m.cursor++
				m.clampCursor()
				return m, nil
			case "k":
				m.cursor--
				m.clampCursor()
				return m, nil
			}
		}
		if t := msg.Text; t != "" {
			m.setQuery(m.query + t)
		}
	}
	m.clampCursor()
	return m, nil
}

// setQuery re-runs the filter and resets the viewport to the best match.
func (m *Model) setQuery(q string) {
	m.query = q
	m.visible = filterCommands(m.commands, q)
	m.cursor, m.offset = 0, 0
}

func (m Model) View() tea.View {
	var b strings.Builder
	b.WriteString(promptStyle.Render("> "+m.query) + "█\n")
	last := min(m.offset+m.pageSize(), len(m.visible))
	for i := m.offset; i < last; i++ {
		b.WriteString(m.renderRow(i))
		b.WriteByte('\n')
	}
	if len(m.visible) == 0 {
		b.WriteString(hintStyle.Render("no commands match") + "\n")
	}
	fmt.Fprintf(&b, "%d/%d · enter run · tab insert · esc cancel", len(m.visible), len(m.commands))
	v := tea.NewView(b.String())
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

// renderRow lays out one visible row: /name + argument hint left, source
// badge right-aligned, truncated to the popup width.
func (m Model) renderRow(i int) string {
	c := m.commands[m.visible[i]]
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

// pageSize is the list viewport height: everything except the prompt and
// status lines.
func (m Model) pageSize() int {
	return max(m.height-2, 1)
}

// clampCursor bounds the cursor and scrolls the viewport to keep it visible.
func (m *Model) clampCursor() {
	m.cursor = max(0, min(m.cursor, len(m.visible)-1))
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+m.pageSize() {
		m.offset = m.cursor - m.pageSize() + 1
	}
}

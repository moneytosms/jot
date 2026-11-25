package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// helpEntry is one keybinding line in the help overlay.
type helpEntry struct {
	keys string
	desc string
}

// helpSection groups keybindings by context.
type helpSection struct {
	title   string
	entries []helpEntry
}

// helpSections is the plain constant keybinding table shown by '?'. It is
// intentionally not derived from reflection so the cheatsheet stays stable
// and readable.
var helpSections = []helpSection{
	{
		title: "navigation",
		entries: []helpEntry{
			{keys: "h l / left right", desc: "previous / next day"},
			{keys: "j k / up down", desc: "previous / next week"},
			{keys: "g G", desc: "first / last day file"},
			{keys: "t", desc: "go to today"},
			{keys: "tab", desc: "toggle the journal sidebar"},
			{keys: "mouse wheel", desc: "scroll the day pane"},
		},
	},
	{
		title: "calendar",
		entries: []helpEntry{
			{keys: "H L / < >", desc: "previous / next month"},
			{keys: "y", desc: "year heatmap of the open journal"},
			{keys: "click a day", desc: "select that day"},
			{keys: "click the header", desc: "go to today"},
		},
	},
	{
		title: "journals",
		entries: []helpEntry{
			{keys: "J K", desc: "next / previous journal"},
			{keys: "n R D", desc: "create, rename or delete a journal"},
			{keys: "click a sidebar row", desc: "switch journal"},
		},
	},
	{
		title: "notes",
		entries: []helpEntry{
			{keys: "a", desc: "quick-add a one-line note"},
			{keys: "e", desc: "open the selected day in your editor"},
			{keys: "ctrl+r", desc: "refresh journals and the day"},
			{keys: "r", desc: "set a reminder"},
			{keys: "editing", desc: "always happens in your editor; jot never rewrites entries"},
		},
	},
	{
		title: "overlays",
		entries: []helpEntry{
			{keys: "/", desc: "fuzzy search every entry"},
			{keys: "ctrl+p", desc: "command palette"},
			{keys: "T", desc: "browse todos from every journal"},
			{keys: "S", desc: "settings"},
			{keys: "?", desc: "this help"},
			{keys: "enter", desc: "activate the highlighted item"},
			{keys: "j k / arrows", desc: "move in a list"},
			{keys: "pgup pgdn / ctrl+d ctrl+u", desc: "page a list"},
			{keys: "tab", desc: "switch search or todo scope"},
			{keys: "esc", desc: "close the overlay"},
		},
	},
	{
		title: "settings",
		entries: []helpEntry{
			{keys: "tab / up down", desc: "move between rows"},
			{keys: "enter", desc: "cycle theme or save the row"},
			{keys: "esc", desc: "close; the current theme stays applied"},
		},
	},
	{
		title: "quit",
		entries: []helpEntry{
			{keys: "q", desc: "quit from the browse screen"},
			{keys: "ctrl+c", desc: "quit from anywhere"},
		},
	},
}

// helpState is the scroll position of the help overlay.
type helpState struct {
	offset int
}

// openHelp opens the help cheatsheet.
func (m model) openHelp() (tea.Model, tea.Cmd) {
	m.help = helpState{}
	m.mode = modeHelp
	return m, nil
}

// helpRect is the centered help panel.
func (m model) helpRect(lay layout) boxRect {
	w := min(max(m.width-8, 1), 80)
	h := max(min(lay.paneH-2, 24), 5)
	return m.centerRect(lay, w, h)
}

// helpVisibleRows is how many cheatsheet lines fit.
func (m model) helpVisibleRows(lay layout) int {
	rect := m.helpRect(lay)
	return max(rect.h-2, 1)
}

// keyHelp feeds keys to the help overlay.
func (m model) keyHelp(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "?":
		m.closeOverlay()
		return m, nil
	case "up", "k":
		m.moveHelp(-1)
		return m, nil
	case "down", "j":
		m.moveHelp(1)
		return m, nil
	case "pgup", "ctrl+u":
		m.moveHelp(-m.helpPage())
		return m, nil
	case "pgdown", "ctrl+d":
		m.moveHelp(m.helpPage())
		return m, nil
	case "home", "g":
		m.help.offset = 0
		return m, nil
	case "end", "G":
		m.help.offset = m.helpMaxOffset()
		return m, nil
	}
	return m, nil
}

// moveHelp scrolls the cheatsheet.
func (m *model) moveHelp(delta int) {
	m.help.offset = clamp(m.help.offset+delta, 0, m.helpMaxOffset())
}

// helpPage is the number of lines a page key jumps.
func (m model) helpPage() int { return max(m.helpVisibleRows(m.layout())-1, 1) }

// helpMaxOffset is the largest valid scroll offset.
func (m model) helpMaxOffset() int {
	return max(len(m.helpTextLines())-m.helpVisibleRows(m.layout()), 0)
}

// helpTextLines renders the constant table into styled lines.
func (m model) helpTextLines() []string {
	lines := make([]string, 0, 80)
	for _, section := range helpSections {
		lines = append(lines, m.theme.accent().Bold(true).Render(strings.ToUpper(section.title)))
		for _, entry := range section.entries {
			keys := m.theme.fg().Render(padRight(entry.keys, 20))
			lines = append(lines, "  "+keys+m.theme.muted().Render(entry.desc))
		}
		lines = append(lines, "")
	}
	return lines
}

// helpView renders the scrollable cheatsheet.
func (m model) helpView(lay layout) string {
	rect := m.helpRect(lay)
	lines := m.helpTextLines()
	m.help.offset = clamp(m.help.offset, 0, m.helpMaxOffset())
	rows := m.helpVisibleRows(lay)
	end := min(m.help.offset+rows, len(lines))
	content := append([]string(nil), lines[m.help.offset:end]...)
	for len(content) < rect.h-2 {
		content = append(content, "")
	}
	return m.overlayCanvas(lay, rect, m.boxLines("KEYS", rect, content))
}

// helpScroll moves the cheatsheet by delta rows for mouse wheels.
func (m *model) helpScroll(delta int) {
	m.moveHelp(delta)
}

// handleHelpMouse scrolls the cheatsheet and closes it on an outside click.
func (m model) handleHelpMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		m.helpScroll(-3)
		return m, nil
	case tea.MouseButtonWheelDown:
		m.helpScroll(3)
		return m, nil
	case tea.MouseButtonLeft:
		if !pointInRect(msg.X, msg.Y, m.helpRect(m.layout())) {
			m.closeOverlay()
			return m, nil
		}
	}
	return m, nil
}

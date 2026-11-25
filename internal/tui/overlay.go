package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/moneytosms/jot"
)

// isOverlayMode reports whether the mode draws a full-screen overlay in place
// of the calendar and day pane.
func isOverlayMode(m mode) bool {
	switch m {
	case modeSearch, modePalette, modeTodos, modeReminder, modeSettings, modeHelp:
		return true
	}
	return false
}

// closeOverlay returns to the browse screen and releases every overlay input.
// Overlay state is kept so reopening is cheap; only focus is cleared.
func (m *model) closeOverlay() {
	m.mode = modeBrowse
	m.search.input.blur()
	m.palette.filter.blur()
	m.palette.prompt.blur()
	m.reminder.message.blur()
	m.reminder.duration.blur()
	m.settings.editor.blur()
	m.settings.journal.blur()
}

// boxRect is the geometry of an overlay panel in body coordinates: x and y
// are zero-based columns and rows below the header, and w/h include the
// border.
type boxRect struct {
	x, y, w, h int
}

// centerRect centers a panel of the requested size in the body area, clamping
// it to the space actually available. Tiny terminals shrink the panel rather
// than panic.
func (m model) centerRect(lay layout, wantW, wantH int) boxRect {
	w := wantW
	if w > m.width {
		w = m.width
	}
	if w < 1 {
		w = 1
	}
	h := wantH
	if h > lay.paneH {
		h = lay.paneH
	}
	if h < 1 {
		h = 1
	}
	return boxRect{
		x: max((m.width-w)/2, 0),
		y: max((lay.paneH-h)/2, 0),
		w: w,
		h: h,
	}
}

// boxLines draws a bordered panel with the title embedded in the top border.
// Every returned line is exactly rect.w printable columns wide.
func (m model) boxLines(title string, rect boxRect, content []string) []string {
	if rect.w <= 0 || rect.h <= 0 {
		return nil
	}
	border := func(s string) string { return m.theme.dim().Render(s) }
	if rect.w <= 2 {
		lines := make([]string, rect.h)
		for i := range lines {
			lines[i] = border("│")
		}
		if rect.w == 2 && len(lines) > 0 {
			for i := range lines {
				lines[i] = border("│") + border("│")
			}
			lines[0] = border("╭") + border("╮")
			lines[len(lines)-1] = border("╰") + border("╯")
		}
		return lines
	}
	innerW := rect.w - 2
	innerH := max(rect.h-2, 0)
	titleText := " " + strings.TrimSpace(title) + " "
	titleText = ansiTruncate(titleText, innerW)
	fill := max(innerW-ansiWidth(titleText), 0)
	lines := make([]string, 0, rect.h)
	lines = append(lines, border("╭")+m.theme.accent().Bold(true).Render(titleText)+border(strings.Repeat("─", fill))+border("╮"))
	for i := 0; i < innerH; i++ {
		text := ""
		if i < len(content) {
			text = ansiTruncate(content[i], max(innerW-1, 0))
		}
		row := padRight(" "+text, innerW)
		lines = append(lines, border("│")+row+border("│"))
	}
	lines = append(lines, border("╰")+border(strings.Repeat("─", innerW))+border("╯"))
	if len(lines) > rect.h {
		lines = lines[:rect.h]
	}
	return lines
}

// overlayCanvas places a panel in the body area. Body rows before and after
// the panel are blank, and the result is exactly lay.paneH lines so the
// header and footer keep their places.
func (m model) overlayCanvas(lay layout, rect boxRect, panel []string) string {
	rows := make([]string, max(lay.paneH, 1))
	for i, line := range panel {
		y := rect.y + i
		if y < 0 || y >= len(rows) {
			continue
		}
		prefix := ""
		if rect.x > 0 {
			prefix = strings.Repeat(" ", rect.x)
		}
		rows[y] = prefix + ansiTruncate(line, max(m.width-rect.x, 0))
	}
	return strings.Join(rows, "\n")
}

// centeredPanel builds a centered bordered panel and returns the rendered body
// and its geometry.
func (m model) centeredPanel(lay layout, wantW, wantH int, title string, content []string) (string, boxRect) {
	rect := m.centerRect(lay, wantW, wantH)
	return m.overlayCanvas(lay, rect, m.boxLines(title, rect, content)), rect
}

// bodyRow converts a mouse y coordinate to a body row (the header is row 0 on
// screen but outside the body).
func bodyRow(y int) int { return y - 1 }

// pointInRect reports whether a mouse coordinate lands inside rect.
func pointInRect(x, y int, rect boxRect) bool {
	row := bodyRow(y)
	return row >= rect.y && row < rect.y+rect.h && x >= rect.x && x < rect.x+rect.w
}

// contentRow converts a mouse y coordinate to a row inside a panel's content
// area, counting rows below the top border.
func contentRow(y int, rect boxRect) int { return bodyRow(y) - rect.y - 1 }

// firstLine returns the first non-blank line of s, trimmed, for one-line
// previews.
func firstLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(line); t != "" {
			return t
		}
	}
	return ""
}

// prefsSavedMsg reports the result of persisting theme/journal preferences.
type prefsSavedMsg struct {
	kind    string
	theme   string
	journal string
	err     error
}

// savePrefsCmd persists the theme and the open journal without blocking the
// update loop.
func savePrefsCmd(theme, journal, kind string) tea.Cmd {
	return func() tea.Msg {
		_, err := jot.SaveTUIPreferences(theme, journal)
		return prefsSavedMsg{kind: kind, theme: theme, journal: journal, err: err}
	}
}

// cycleTheme advances to the next ThemeNames entry, applies it immediately so
// the user sees it, and persists it in the background.
func (m *model) cycleTheme() tea.Cmd {
	names := ThemeNames()
	idx := -1
	for i, name := range names {
		if name == m.themeName {
			idx = i
			break
		}
	}
	if idx < 0 {
		idx = len(names) - 1 // unknown or empty starts at the first name
	}
	name := names[(idx+1)%len(names)]
	m.themeName = name
	m.theme = Resolve(name)
	return savePrefsCmd(name, m.journal, "theme")
}

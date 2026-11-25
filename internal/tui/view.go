package tui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/moneytosms/jot"
)

// Layout constants. Calendar cells are 3 columns wide so clicks map from
// geometry alone; the sidebar is a fixed width.
const (
	calCellW = 3
	calW     = calCellW * 7
	leftPad  = 1
	sideW    = 20
)

// layout records where everything sits in content coordinates (0-based
// rows/columns of the terminal). It is a pure function of the model, so both
// View and mouse handling agree on what is where.
type layout struct {
	bodyY    int // first row below the header
	leftW    int // total width of the left column
	sideX    int // content x of the sidebar list
	sideY    int // content y of the first journal row
	sideRows int // number of journal rows that fit
	calX     int // content x of the calendar grid
	calY     int // content y of the weekday header row
	paneX    int // content x of the day pane box
	paneW    int // total width of the day pane box
	paneH    int // total height of the day pane box
	innerW   int // text width inside the pane border+padding
	innerH   int // text height inside the pane border
}

// layout computes the screen geometry for the current model state.
func (m model) layout() layout {
	var l layout
	if m.width <= 0 || m.height <= 0 {
		return l
	}
	l.bodyY = 1
	l.leftW = leftPad + calW + leftPad
	l.sideX = leftPad
	l.sideY = l.bodyY + 1
	l.calX = leftPad
	l.calY = l.bodyY + 1
	l.paneX = l.leftW
	l.paneW = max(m.width-l.leftW, 12)
	l.paneH = max(m.height-l.bodyY-2, 1) // header + hints + status lines
	l.innerH = max(l.paneH-2, 1)
	l.innerW = max(l.paneW-4, 1)
	if m.showSidebar {
		// Cap the journal rows so the calendar stays on screen.
		l.sideRows = min(max(l.paneH-4, 1), len(m.journals))
		l.calY = l.sideY + l.sideRows + 1
	}
	return l
}

// View renders the whole screen: header, sidebar+calendar column, day pane,
// hints and status.
func (m model) View() string {
	if m.width <= 0 || m.height <= 0 {
		return "jot" // never panic before the first WindowSizeMsg
	}
	lay := m.layout()
	if lay.paneH <= 2 {
		// Extremely small terminal: show what still fits.
		return ansiTruncate(m.headerText(), m.width) + "\n" + ansiTruncate(m.hintText(), m.width)
	}
	var body string
	if isOverlayMode(m.mode) {
		body = m.overlayView(lay)
	} else {
		body = lipgloss.JoinHorizontal(lipgloss.Top, m.leftColumn(lay), m.paneBox(lay))
	}
	return m.headerText() + "\n" + body + "\n" + m.footerText(lay)
}

// overlayView dispatches to the active overlay's renderer. Each renderer
// returns exactly lay.paneH lines.
func (m model) overlayView(lay layout) string {
	switch m.mode {
	case modeSearch:
		return m.searchView(lay)
	case modePalette:
		return m.paletteView(lay)
	case modeTodos:
		return m.todosView(lay)
	case modeHeatmap:
		return m.heatView(lay)
	case modeReminder:
		return m.reminderView(lay)
	case modeSettings:
		return m.settingsView(lay)
	case modeHelp:
		return m.helpView(lay)
	}
	return ""
}

// Colors as lipgloss helpers.

func (t Theme) fg() lipgloss.Style    { return lipgloss.NewStyle().Foreground(lipgloss.Color(t.Fg)) }
func (t Theme) muted() lipgloss.Style { return lipgloss.NewStyle().Foreground(lipgloss.Color(t.Muted)) }
func (t Theme) dim() lipgloss.Style   { return lipgloss.NewStyle().Foreground(lipgloss.Color(t.Dim)) }
func (t Theme) accent() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(t.Accent))
}
func (t Theme) good() lipgloss.Style { return lipgloss.NewStyle().Foreground(lipgloss.Color(t.Green)) }
func (t Theme) bad() lipgloss.Style  { return lipgloss.NewStyle().Foreground(lipgloss.Color(t.Red)) }
func (t Theme) selStyle() lipgloss.Style {
	return lipgloss.NewStyle().Background(lipgloss.Color(t.SelBg)).Foreground(lipgloss.Color(t.SelFg))
}

// Header.

// headerText renders the top bar: app name, journal, month, quick stats.
func (m model) headerText() string {
	days, entries := m.monthStats()
	parts := []string{
		m.theme.accent().Bold(true).Render("jot"),
		m.theme.fg().Bold(true).Render(m.journal),
		m.theme.fg().Render(m.month.Format("January 2006")),
		m.theme.muted().Render(fmt.Sprintf("%dd %de", days, entries)),
	}
	if m.loading {
		parts = append(parts, m.theme.muted().Render("…"))
	}
	return ansiTruncate(strings.Join(parts, m.theme.muted().Render(" · ")), m.width)
}

// Left column: journal sidebar plus the month calendar.

// leftColumn renders the sidebar (when shown) and the calendar grid, padded
// to the column width so the pane joins flush.
func (m model) leftColumn(lay layout) string {
	var lines []string
	if m.showSidebar {
		lines = append(lines, m.theme.muted().Bold(true).Render(" JOURNALS"))
		for _, j := range m.visibleJournals(lay) {
			lines = append(lines, m.journalRow(j, j.Name == m.journal))
		}
		lines = append(lines, "")
	}
	lines = append(lines, m.calendarLines(lay)...)
	for i, l := range lines {
		lines[i] = " " + padRight(l, lay.leftW-1) // one space of left padding, like calX
	}
	return strings.Join(lines, "\n")
}

// journalRow renders one sidebar row: marker, name and day/entry counts.
func (m model) journalRow(j jot.JournalInfo, active bool) string {
	name := j.Name
	if w := utf8.RuneCountInString(name); w > 10 {
		r := []rune(name)
		name = string(r[:9]) + "…"
	} else if w < 10 {
		name = name + strings.Repeat(" ", 10-w)
	}
	counts := fmt.Sprintf("%2dd %2de", j.Days, j.Entries)
	if j.Last.IsZero() {
		counts = "   —    "
	}
	marker, nameStyle := " ", m.theme.muted()
	if active {
		marker, nameStyle = "▸", m.theme.accent().Bold(true)
	}
	return marker + " " + nameStyle.Render(name) + " " + m.theme.dim().Render(counts)
}

// calendarLines renders the weekday header and the month grid rows.
func (m model) calendarLines(lay layout) []string {
	out := []string{m.theme.dim().Render("Mo Tu We Th Fr Sa Su")}
	for _, week := range m.grid() {
		var b strings.Builder
		for _, cell := range week.Days {
			b.WriteString(m.cellText(cell))
		}
		out = append(out, ansiTruncate(b.String(), calW))
	}
	return out
}

// cellText renders one calendar cell, exactly calCellW columns wide.
func (m model) cellText(cell Cell) string {
	if cell.Day == 0 {
		return "   "
	}
	text := fmt.Sprintf("%2d ", cell.Day)
	st := lipgloss.NewStyle()
	switch {
	case cell.IsSelected:
		st = m.theme.selStyle()
	case cell.Level >= 3:
		st = m.theme.fg().Foreground(lipgloss.Color(m.theme.Yellow))
	case cell.Level >= 1:
		st = m.theme.fg().Foreground(lipgloss.Color(m.theme.Green))
	case cell.Level == 0:
		st = m.theme.muted()
	}
	if cell.IsToday && !cell.IsSelected {
		st = st.Underline(true).Foreground(lipgloss.Color(m.theme.Accent))
	}
	return st.Render(text)
}

// Day pane.

// paneBox wraps the day content in a rounded border, exactly paneW x paneH.
func (m model) paneBox(lay layout) string {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(m.theme.Border)).
		Padding(0, 1).
		Width(lay.innerW + 2). // includes the 1+1 padding columns
		Height(lay.innerH).
		MaxWidth(lay.paneW).
		MaxHeight(lay.paneH).
		Render(m.paneContent(lay))
}

// paneContent builds the pane text: a date heading plus the day's entries,
// sliced to the visible scroll window.
func (m model) paneContent(lay layout) string {
	title := m.theme.accent().Bold(true).Render(m.selected.Format("Monday · 2 January 2006"))
	var body []string
	if strings.TrimSpace(m.dayContent) == "" {
		body = []string{
			"",
			m.theme.muted().Render("No entries on this day."),
			m.theme.muted().Render("Press a to add a note, e to open the editor."),
		}
	} else {
		body = strings.Split(jot.RenderStyled(m.dayContent, lay.innerW, m.theme.mdStyles()), "\n")
	}
	all := append([]string{title, ""}, body...)
	h := lay.innerH
	maxStart := max(len(all)-h, 0)
	start := clamp(m.scroll, 0, maxStart)
	vis := all[start:min(start+h, len(all))]
	for len(vis) < h {
		vis = append(vis, "")
	}
	return strings.Join(vis, "\n")
}

// Footer.

// footerText renders the hint line and the status line (input, toast or
// error).
func (m model) footerText(lay layout) string {
	return ansiTruncate(m.hintText(), m.width) + "\n" + ansiTruncate(m.statusText(lay), m.width)
}

// hintText renders the context key hints for the current mode.
func (m model) hintText() string {
	key := m.theme.fg()
	sep := m.theme.dim().Render(" · ")
	switch m.mode {
	case modeQuickAdd:
		return key.Render("enter") + sep + "save" + sep + key.Render("esc") + " cancel"
	case modeNewJournal:
		return key.Render("enter") + sep + "create" + sep + key.Render("esc") + " cancel"
	case modeRenameJournal:
		return key.Render("enter") + sep + "rename" + sep + key.Render("esc") + " cancel"
	case modeDeleteJournal:
		return key.Render("enter") + sep + "delete" + sep + key.Render("esc") + " cancel"
	case modeSearch:
		return strings.Join([]string{
			key.Render("type") + " search", key.Render("tab") + " scope",
			key.Render("j/k") + " or " + key.Render("arrows") + " move",
			key.Render("enter") + " jump", key.Render("esc") + " close",
		}, sep)
	case modePalette:
		if m.palette.promptKind != "" {
			return key.Render("enter") + sep + "confirm" + sep + key.Render("esc") + " close"
		}
		return strings.Join([]string{
			key.Render("type") + " filter", key.Render("j/k") + " move",
			key.Render("enter") + " run", key.Render("esc") + " close",
		}, sep)
	case modeTodos:
		return strings.Join([]string{
			key.Render("j/k") + " move", key.Render("enter") + " edit day",
			key.Render("r") + " remind", key.Render("tab") + " scope",
			key.Render("esc") + " close", "jot never toggles boxes",
		}, sep)
	case modeReminder:
		return strings.Join([]string{
			key.Render("tab") + " field", key.Render("enter") + " set",
			key.Render("esc") + " cancel",
		}, sep)
	case modeSettings:
		return strings.Join([]string{
			key.Render("tab/up/down") + " row", key.Render("enter") + " save or cycle",
			key.Render("esc") + " close",
		}, sep)
	case modeHelp:
		return strings.Join([]string{
			key.Render("j/k") + " scroll", key.Render("esc") + " close",
		}, sep)
	case modeHeatmap:
		return strings.Join([]string{
			key.Render("h/l") + " year", key.Render("t") + " today",
			key.Render("j/k") + " scroll", key.Render("esc") + " close",
		}, sep)
	default:
		return strings.Join([]string{
			key.Render("/") + " search", key.Render("ctrl+p") + " palette",
			key.Render("T") + " todos", key.Render("y") + " year",
			key.Render("S") + " settings",
			key.Render("r") + " remind", key.Render("?") + " help",
			key.Render("h/l") + " day", key.Render("j/k") + " week",
			key.Render("H/L") + " month", key.Render("t") + " today",
			key.Render("a") + " add", key.Render("e") + " edit",
			key.Render("n") + " new", key.Render("J/K") + " journal",
			key.Render("tab") + " sidebar", key.Render("ctrl+r") + " refresh",
			key.Render("q") + " quit",
		}, sep)
	}
}

// statusText renders the bottom line: the active input, or a toast, or the
// last error.
func (m model) statusText(lay layout) string {
	switch m.mode {
	case modeQuickAdd:
		return m.theme.accent().Render("add "+dayKey(m.selected)+" ") + m.input.view()
	case modeNewJournal:
		return m.theme.accent().Render("new journal ") + m.input.view()
	case modeRenameJournal:
		return m.theme.accent().Render("rename "+m.journal+" to ") + m.input.view()
	case modeDeleteJournal:
		return m.theme.bad().Render("delete "+m.journal+" · type the name: ") + m.input.view()
	case modeSearch:
		if m.search.err != nil {
			return m.theme.bad().Render("✗ " + m.search.err.Error())
		}
		if m.search.loading {
			return m.theme.muted().Render("searching…")
		}
	case modeTodos:
		if m.todos.err != nil {
			return m.theme.bad().Render("✗ " + m.todos.err.Error())
		}
		if m.todos.loading {
			return m.theme.muted().Render("loading todos…")
		}
	case modeReminder:
		if m.reminder.err != "" {
			return m.theme.bad().Render("✗ " + m.reminder.err)
		}
		if m.reminder.pending {
			return m.theme.muted().Render("setting reminder…")
		}
	case modeSettings:
		if m.settings.err != "" {
			return m.theme.bad().Render("✗ " + m.settings.err)
		}
	case modePalette:
		if m.palette.err != "" {
			return m.theme.bad().Render("✗ " + m.palette.err)
		}
	}
	if m.toast != "" {
		return m.theme.good().Render("✓ " + m.toast)
	}
	if m.lastErr != nil {
		return m.theme.bad().Render("✗ " + m.lastErr.Error())
	}
	return ""
}

// ANSI-aware text helpers. Terminal text carries SGR escapes that count as
// zero columns; naive byte slicing would cut them in half.

// ansiTruncate cuts s to at most w printable columns, keeping escape
// sequences intact.
func ansiTruncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	var b strings.Builder
	cells, i := 0, 0
	for i < len(s) && cells < w {
		if s[i] == '\x1b' {
			j := i + 1
			if j < len(s) && s[j] == '[' {
				j++ // skip the CSI introducer before the parameter bytes
				for j < len(s) && (s[j] < 0x40 || s[j] > 0x7e) {
					j++
				}
				if j < len(s) {
					j++ // final byte
				}
			} else if j < len(s) {
				j++
			}
			b.WriteString(s[i:j])
			i = j
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		b.WriteRune(r)
		cells++
		i += size
	}
	return b.String()
}

// padRight appends spaces until s spans w printable columns.
func padRight(s string, w int) string {
	if extra := w - ansiWidth(s); extra > 0 {
		return s + strings.Repeat(" ", extra)
	}
	return s
}

// ansiWidth counts the printable columns of s, skipping ANSI escapes.
func ansiWidth(s string) int {
	cells, i := 0, 0
	for i < len(s) {
		if s[i] == '\x1b' {
			j := i + 1
			if j < len(s) && s[j] == '[' {
				j++ // skip the CSI introducer before the parameter bytes
				for j < len(s) && (s[j] < 0x40 || s[j] > 0x7e) {
					j++
				}
				if j < len(s) {
					j++
				}
			} else if j < len(s) {
				j++
			}
			i = j
			continue
		}
		_, size := utf8.DecodeRuneInString(s[i:])
		cells++
		i += size
	}
	return cells
}

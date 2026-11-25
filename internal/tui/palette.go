package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/moneytosms/jot"
)

// paletteAction is one command in the ctrl+p palette.
type paletteAction struct {
	id    string
	label string
}

// paletteState is the state of the command palette.
type paletteState struct {
	filter     lineInput
	prompt     lineInput
	promptKind string
	items      []paletteAction
	cursor     int
	offset     int
	err        string
	// listFocus lets j/k move the selection; while the filter input is
	// focused printable keys type instead.
	listFocus bool
}

// paletteActions builds the action list for the current model state. Journal
// switches are one action per loaded journal.
func (m model) paletteActions() []paletteAction {
	actions := []paletteAction{
		{id: "today", label: "Go to today"},
		{id: "jump", label: "Jump to a day"},
	}
	for _, j := range m.journals {
		actions = append(actions, paletteAction{id: "journal:" + j.Name, label: "Switch journal: " + j.Name})
	}
	actions = append(actions,
		paletteAction{id: "new-journal", label: "New journal"},
		paletteAction{id: "rename-journal", label: "Rename journal"},
		paletteAction{id: "delete-journal", label: "Delete journal"},
		paletteAction{id: "search", label: "Search entries"},
		paletteAction{id: "todos", label: "Browse todos"},
		paletteAction{id: "heatmap", label: "Year heatmap"},
		paletteAction{id: "reminder", label: "Set a reminder"},
		paletteAction{id: "settings", label: "Settings"},
		paletteAction{id: "theme", label: "Toggle theme"},
		paletteAction{id: "editor", label: "Open day in editor"},
		paletteAction{id: "backup", label: "Backup everything (.tar.gz)"},
		paletteAction{id: "backup-zip", label: "Backup everything (.zip)"},
		paletteAction{id: "export", label: "Export journals"},
		paletteAction{id: "export-html", label: "Export this journal as HTML"},
		paletteAction{id: "export-pdf", label: "Export this journal as PDF"},
		paletteAction{id: "help", label: "Help"},
		paletteAction{id: "quit", label: "Quit"},
	)
	return actions
}

// filterPaletteActions fuzzy-filters the actions, best score first. An empty
// query keeps every action in its natural order.
func filterPaletteActions(query string, actions []paletteAction) []paletteAction {
	q := strings.TrimSpace(query)
	if q == "" {
		return append([]paletteAction(nil), actions...)
	}
	type scored struct {
		action paletteAction
		score  int
		order  int
	}
	var hits []scored
	needle := strings.ToLower(q)
	for i, a := range actions {
		score := jot.FuzzyScore(q, a.label)
		if score <= 0 && jot.FuzzyScore(q, a.id) > 0 {
			score = jot.FuzzyScore(q, a.id)
		}
		if score > 0 {
			// Reward literal matches so "today" offers "Go to today" before
			// actions that merely contain t-o-d-a-y in order.
			label := strings.ToLower(a.label)
			switch {
			case label == needle:
				score += 1000
			case strings.HasPrefix(label, needle):
				score += 500
			case strings.Contains(label, needle):
				score += 250
			}
			hits = append(hits, scored{action: a, score: score, order: i})
		}
	}
	sort.SliceStable(hits, func(i, k int) bool {
		if hits[i].score != hits[k].score {
			return hits[i].score > hits[k].score
		}
		return hits[i].order < hits[k].order
	})
	out := make([]paletteAction, 0, len(hits))
	for _, h := range hits {
		out = append(out, h.action)
	}
	return out
}

// openPalette opens the command palette with an empty filter.
func (m model) openPalette() (tea.Model, tea.Cmd) {
	st := paletteState{}
	st.filter = newLineInput()
	st.filter.placeholder = "type a command"
	st.filter.focus()
	st.listFocus = true
	st.prompt = newLineInput()
	m.palette = st
	m.mode = modePalette
	m.refreshPalette()
	return m, nil
}

// refreshPalette recomputes the filtered actions and clamps the cursor.
func (m *model) refreshPalette() {
	m.palette.items = filterPaletteActions(m.palette.filter.value(), m.paletteActions())
	m.palette.cursor = clamp(m.palette.cursor, 0, max(len(m.palette.items)-1, 0))
	m.palette.offset = 0
	m.ensurePaletteVisible()
}

// paletteRect is the centered palette panel.
func (m model) paletteRect(lay layout) boxRect {
	w := min(max(m.width-6, 1), 60)
	if m.palette.promptKind != "" {
		return m.centerRect(lay, w, 7)
	}
	rows := clamp(len(m.palette.items), 1, 12)
	return m.centerRect(lay, w, rows+5)
}

// paletteVisibleRows is how many action rows fit in the panel.
func (m model) paletteVisibleRows(lay layout) int {
	rect := m.paletteRect(lay)
	return max(rect.h-2-3, 1)
}

// ensurePaletteVisible scrolls the action window to include the cursor.
func (m *model) ensurePaletteVisible() {
	rows := m.paletteVisibleRows(m.layout())
	if m.palette.cursor < m.palette.offset {
		m.palette.offset = m.palette.cursor
	}
	if m.palette.cursor >= m.palette.offset+rows {
		m.palette.offset = m.palette.cursor - rows + 1
	}
	last := max(len(m.palette.items)-rows, 0)
	m.palette.offset = clamp(m.palette.offset, 0, last)
}

// movePalette moves the highlighted action.
func (m *model) movePalette(delta int) {
	if len(m.palette.items) == 0 {
		return
	}
	m.palette.cursor = clamp(m.palette.cursor+delta, 0, len(m.palette.items)-1)
	m.ensurePaletteVisible()
}

// keyPalette feeds keys to the command palette or its inline prompt.
func (m model) keyPalette(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if m.palette.promptKind != "" {
		switch key {
		case "esc":
			m.closeOverlay()
			return m, nil
		case "enter":
			return m.submitPalettePrompt()
		case "tab":
			return m, nil
		}
		if m.palette.prompt.update(msg) {
			m.palette.err = ""
		}
		return m, nil
	}

	switch key {
	case "esc":
		m.closeOverlay()
		return m, nil
	case "enter":
		return m.runSelectedPaletteAction()
	case "up":
		m.palette.listFocus = true
		m.movePalette(-1)
		return m, nil
	case "down":
		m.palette.listFocus = true
		m.movePalette(1)
		return m, nil
	case "k":
		if m.palette.listFocus {
			m.movePalette(-1)
			return m, nil
		}
	case "j":
		if m.palette.listFocus {
			m.movePalette(1)
			return m, nil
		}
	case "pgup", "ctrl+u":
		m.palette.listFocus = true
		m.movePalette(-m.palettePage())
		return m, nil
	case "pgdown", "ctrl+d":
		m.palette.listFocus = true
		m.movePalette(m.palettePage())
		return m, nil
	case "home":
		m.palette.listFocus = true
		m.palette.cursor = 0
		m.ensurePaletteVisible()
		return m, nil
	case "end":
		m.palette.listFocus = true
		m.palette.cursor = max(len(m.palette.items)-1, 0)
		m.ensurePaletteVisible()
		return m, nil
	case "/":
		if m.palette.listFocus {
			m.palette.listFocus = false
			return m, nil
		}
	}
	if m.palette.filter.update(msg) {
		m.palette.listFocus = false
		m.refreshPalette()
	}
	return m, nil
}

// palettePage is the number of actions a page key jumps.
func (m model) palettePage() int {
	return max(m.paletteVisibleRows(m.layout())-1, 1)
}

// submitPalettePrompt validates and runs the inline date/journal prompt.
func (m model) submitPalettePrompt() (tea.Model, tea.Cmd) {
	value := strings.TrimSpace(m.palette.prompt.value())
	switch m.palette.promptKind {
	case "day":
		label, err := jot.ParseDay(value)
		if err != nil {
			m.palette.err = err.Error()
			return m, nil
		}
		day, err := time.ParseInLocation("2006-01-02", label, time.Local)
		if err != nil {
			m.palette.err = err.Error()
			return m, nil
		}
		m.closeOverlay()
		m.selected = day
		m.month = firstOfMonth(day)
		m.scroll = 0
		return m, m.fullLoad()
	case "journal":
		if value == "" {
			m.palette.err = "journal name is empty"
			return m, nil
		}
		name := jot.SanitizeName(value)
		if name == m.journal {
			m.palette.err = "that journal is already open"
			return m, nil
		}
		m.closeOverlay()
		return m, createJournalCmd(m.jot, name)
	}
	return m, nil
}

// runSelectedPaletteAction closes the palette and runs the highlighted action.
func (m model) runSelectedPaletteAction() (tea.Model, tea.Cmd) {
	if len(m.palette.items) == 0 {
		return m, nil
	}
	return m.runPaletteAction(m.palette.items[m.palette.cursor])
}

// runPaletteAction executes one palette action.
func (m model) runPaletteAction(a paletteAction) (tea.Model, tea.Cmd) {
	switch {
	case a.id == "today":
		m.closeOverlay()
		return m.jumpToday()
	case a.id == "jump":
		m.palette.promptKind = "day"
		m.palette.prompt.placeholder = "today, yesterday or YYYY-MM-DD"
		m.palette.prompt.setValue("")
		m.palette.prompt.focus()
		m.palette.err = ""
		return m, nil
	case strings.HasPrefix(a.id, "journal:"):
		name := strings.TrimPrefix(a.id, "journal:")
		m.closeOverlay()
		return m.switchJournal(name)
	case a.id == "new-journal":
		m.palette.promptKind = "journal"
		m.palette.prompt.placeholder = "journal name"
		m.palette.prompt.setValue("")
		m.palette.prompt.focus()
		m.palette.err = ""
		return m, nil
	case a.id == "rename-journal":
		m.closeOverlay()
		return m.startRenameJournal(m.journal)
	case a.id == "delete-journal":
		m.closeOverlay()
		return m.startDeleteJournal(m.journal)
	case a.id == "search":
		m.closeOverlay()
		return m.openSearch()
	case a.id == "todos":
		m.closeOverlay()
		return m.openTodos()
	case a.id == "heatmap":
		m.closeOverlay()
		return m.openHeatmap()
	case a.id == "reminder":
		m.closeOverlay()
		return m.openReminder(m.lastEntryPreview())
	case a.id == "settings":
		m.closeOverlay()
		return m.openSettings()
	case a.id == "theme":
		m.closeOverlay()
		return m, m.cycleTheme()
	case a.id == "editor":
		m.closeOverlay()
		return m, m.editDay()
	case a.id == "backup" || a.id == "backup-zip":
		m.closeOverlay()
		return m, backupWithCmd(m.jot, jot.BackupOptions{Zip: a.id == "backup-zip"})
	case a.id == "export-html" || a.id == "export-pdf":
		m.closeOverlay()
		j, name, format := m.jot, m.journal, strings.TrimPrefix(a.id, "export-")
		return m, func() tea.Msg {
			path, err := j.ExportDocument(name, jot.ExportOptions{Format: format})
			if err != nil {
				return errMsg{err}
			}
			return toastMsg("exported " + path)
		}
	case a.id == "export":
		m.closeOverlay()
		return m, exportCmd(m.jot)
	case a.id == "help":
		m.closeOverlay()
		return m.openHelp()
	case a.id == "quit":
		m.quitting = true
		return m, tea.Quit
	}
	return m, nil
}

// paletteView renders the centered command palette.
func (m model) paletteView(lay layout) string {
	rect := m.paletteRect(lay)
	innerW := max(rect.w-2, 1)
	width := max(innerW-1, 1)
	if m.palette.promptKind != "" {
		label := "Jump to day"
		aux := "today, yesterday or YYYY-MM-DD"
		if m.palette.promptKind == "journal" {
			label = "New journal"
			aux = "letters, digits, - and _"
		}
		content := []string{
			m.theme.muted().Render(label),
			m.palette.prompt.view(),
			m.errLine(),
			m.theme.dim().Render(aux),
		}
		return m.overlayCanvas(lay, rect, m.boxLines("COMMANDS", rect, content))
	}

	content := []string{
		m.palette.filter.view(),
		m.theme.dim().Render(strings.Repeat("─", max(width, 1))),
	}
	rows := m.paletteVisibleRows(lay)
	end := min(m.palette.offset+rows, len(m.palette.items))
	for i := m.palette.offset; i < end; i++ {
		label := ansiTruncate(" "+m.palette.items[i].label, width)
		if i == m.palette.cursor {
			label = m.theme.selStyle().Render(padRight(label, width))
		} else {
			label = padRight(label, width)
		}
		content = append(content, label)
	}
	if len(m.palette.items) == 0 {
		content = append(content, m.theme.muted().Render("No matching commands."))
	}
	for len(content) < rect.h-2-1 {
		content = append(content, "")
	}
	content = append(content, m.theme.dim().Render("enter run · esc close"))
	return m.overlayCanvas(lay, rect, m.boxLines("COMMANDS", rect, content))
}

// errLine renders the inline palette error, if any.
func (m model) errLine() string {
	if m.palette.err == "" {
		return ""
	}
	return m.theme.bad().Render(m.palette.err)
}

// paletteRowAt maps a mouse coordinate to an action index.
func (m model) paletteRowAt(lay layout, x, y int) (int, bool) {
	if m.palette.promptKind != "" {
		return 0, false
	}
	rect := m.paletteRect(lay)
	if !pointInRect(x, y, rect) {
		return 0, false
	}
	row := contentRow(y, rect)
	if row < 2 || row >= rect.h-3 {
		return 0, false
	}
	idx := m.palette.offset + row - 2
	if idx < 0 || idx >= len(m.palette.items) {
		return 0, false
	}
	return idx, true
}

// lastEntryPreview is the first line of the selected day's newest entry, used
// to prefill the reminder form. Empty when the day has no entries.
func (m model) lastEntryPreview() string {
	for i := len(m.dayEntries) - 1; i >= 0; i-- {
		if line := firstLine(m.dayEntries[i].Body); line != "" {
			return line
		}
	}
	return ""
}

// backupDoneMsg reports a backup archive path.
type backupDoneMsg struct {
	path string
	err  error
}

// exportDoneMsg reports how many journal files an export wrote.
type exportDoneMsg struct {
	dir   string
	count int
	err   error
}

// backupWithCmd writes a full backup in the background.
func backupWithCmd(j *jot.Jot, opt jot.BackupOptions) tea.Cmd {
	return func() tea.Msg {
		path, err := j.BackupWith(opt)
		return backupDoneMsg{path: path, err: err}
	}
}

// exportCmd exports one markdown file per journal in the background.
func exportCmd(j *jot.Jot) tea.Cmd {
	return func() tea.Msg {
		paths, err := j.Export("")
		return exportDoneMsg{dir: "jot-export", count: len(paths), err: err}
	}
}

// formatFireTime renders a reminder fire time compactly, including the date
// when it is not today.
func formatFireTime(at time.Time) string {
	now := time.Now()
	if at.Year() == now.Year() && at.YearDay() == now.YearDay() {
		return at.Format("15:04")
	}
	return at.Format("Jan 2 15:04")
}

// backupLabel is a short toast for a backup result.
func backupLabel(path string) string { return "backup: " + path }

// exportLabel is a short toast for an export result.
func exportLabel(msg exportDoneMsg) string {
	if msg.count == 0 {
		return fmt.Sprintf("nothing to export to %s", msg.dir)
	}
	return fmt.Sprintf("exported %d %s to %s", msg.count, plural(msg.count, "journal", "journals"), msg.dir)
}

// handlePaletteMouse scrolls the action list, runs clicked actions and closes
// the panel when the click lands outside it.
func (m model) handlePaletteMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	lay := m.layout()
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		if m.palette.promptKind == "" {
			m.movePalette(-3)
		}
		return m, nil
	case tea.MouseButtonWheelDown:
		if m.palette.promptKind == "" {
			m.movePalette(3)
		}
		return m, nil
	case tea.MouseButtonLeft:
		if !pointInRect(msg.X, msg.Y, m.paletteRect(lay)) {
			m.closeOverlay()
			return m, nil
		}
		if idx, ok := m.paletteRowAt(lay, msg.X, msg.Y); ok {
			m.palette.cursor = idx
			m.ensurePaletteVisible()
			return m.runSelectedPaletteAction()
		}
	}
	return m, nil
}

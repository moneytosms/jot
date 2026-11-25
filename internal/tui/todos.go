package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/moneytosms/jot"
)

// todoResultRow is the content row where todo items start: a scope line and a
// column header come first.
const todoResultRow = 2

// todoState is the state of the aggregated checkbox overlay.
type todoState struct {
	scopeAll bool
	seq      int
	loading  bool
	err      error
	items    []jot.TodoItem
	cursor   int
	offset   int
}

// todosResultMsg carries one todo query response.
type todosResultMsg struct {
	seq     int
	scope   string
	journal string
	items   []jot.TodoItem
	err     error
}

// openTodos opens the todo overlay and starts loading every journal.
func (m model) openTodos() (tea.Model, tea.Cmd) {
	seq := m.todos.seq + 1
	m.todos = todoState{scopeAll: true, seq: seq, loading: true}
	m.mode = modeTodos
	return m, m.todosLoadCmd()
}

// todosLoadCmd reads the aggregated or current-journal todos in the
// background.
func (m model) todosLoadCmd() tea.Cmd {
	seq := m.todos.seq
	scopeAll := m.todos.scopeAll
	journal := m.journal
	j := m.jot
	return func() tea.Msg {
		var (
			items []jot.TodoItem
			err   error
		)
		if scopeAll {
			items, err = j.Todos()
		} else {
			items, err = j.TodosJournal(journal)
		}
		return todosResultMsg{seq: seq, journal: journal, items: items, err: err}
	}
}

// applyTodosResult merges a todo response, dropping stale ones.
func (m model) applyTodosResult(msg todosResultMsg) (tea.Model, tea.Cmd) {
	if m.mode != modeTodos || msg.seq != m.todos.seq {
		return m, nil
	}
	m.todos.loading = false
	m.todos.err = msg.err
	m.todos.cursor = 0
	m.todos.offset = 0
	if msg.err != nil {
		m.todos.items = nil
		return m, nil
	}
	m.todos.items = sortTodosForDisplay(msg.items)
	return m, nil
}

// sortTodosForDisplay keeps open items before done ones while preserving the
// library's newest-day-first order inside each group.
func sortTodosForDisplay(items []jot.TodoItem) []jot.TodoItem {
	out := append([]jot.TodoItem(nil), items...)
	sort.SliceStable(out, func(i, k int) bool {
		return !out[i].Done && out[k].Done
	})
	return out
}

// todosRect is the full-width todo panel.
func (m model) todosRect(lay layout) boxRect {
	return boxRect{x: 0, y: 0, w: max(m.width, 1), h: max(lay.paneH, 1)}
}

// todosVisibleRows is how many todo rows fit, clamped to at least one.
func (m model) todosVisibleRows(lay layout) int {
	rect := m.todosRect(lay)
	innerH := max(rect.h-2, 0)
	return max(innerH-todoResultRow, 1)
}

// keyTodos feeds keys to the todo overlay.
func (m model) keyTodos(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.closeOverlay()
		return m, nil
	case "tab":
		m.todos.scopeAll = !m.todos.scopeAll
		m.todos.seq++
		m.todos.items = nil
		m.todos.cursor = 0
		m.todos.offset = 0
		m.todos.err = nil
		m.todos.loading = true
		return m, m.todosLoadCmd()
	case "enter":
		return m.editSelectedTodo()
	case "r":
		return m.remindSelectedTodo()
	case "up", "k":
		m.moveTodos(-1)
		return m, nil
	case "down", "j":
		m.moveTodos(1)
		return m, nil
	case "pgup", "ctrl+u":
		m.moveTodos(-m.todosPage())
		return m, nil
	case "pgdown", "ctrl+d":
		m.moveTodos(m.todosPage())
		return m, nil
	case "home", "g":
		m.todos.cursor = 0
		m.ensureTodosVisible()
		return m, nil
	case "end", "G":
		m.todos.cursor = max(len(m.todos.items)-1, 0)
		m.ensureTodosVisible()
		return m, nil
	}
	return m, nil
}

// todosPage is the number of rows a page key jumps.
func (m model) todosPage() int {
	return max(m.todosVisibleRows(m.layout())-1, 1)
}

// moveTodos moves the highlighted item and keeps it on screen.
func (m *model) moveTodos(delta int) {
	if len(m.todos.items) == 0 {
		return
	}
	m.todos.cursor = clamp(m.todos.cursor+delta, 0, len(m.todos.items)-1)
	m.ensureTodosVisible()
}

// ensureTodosVisible scrolls the todo window to include the cursor.
func (m *model) ensureTodosVisible() {
	rows := m.todosVisibleRows(m.layout())
	if m.todos.cursor < m.todos.offset {
		m.todos.offset = m.todos.cursor
	}
	if m.todos.cursor >= m.todos.offset+rows {
		m.todos.offset = m.todos.cursor - rows + 1
	}
	last := max(len(m.todos.items)-rows, 0)
	m.todos.offset = clamp(m.todos.offset, 0, last)
}

// scrollTodos moves the todo window by delta rows without losing the cursor.
func (m *model) scrollTodos(delta int) {
	if len(m.todos.items) == 0 {
		return
	}
	rows := m.todosVisibleRows(m.layout())
	last := max(len(m.todos.items)-rows, 0)
	m.todos.offset = clamp(m.todos.offset+delta, 0, last)
	m.todos.cursor = clamp(m.todos.cursor, m.todos.offset, m.todos.offset+rows-1)
	m.todos.cursor = clamp(m.todos.cursor, 0, len(m.todos.items)-1)
}

// editSelectedTodo opens the item's day in the editor, switching journals
// when necessary. Jot never toggles the checkbox itself.
func (m model) editSelectedTodo() (tea.Model, tea.Cmd) {
	if len(m.todos.items) == 0 {
		return m, nil
	}
	item := m.todos.items[m.todos.cursor]
	day, err := time.ParseInLocation("2006-01-02", item.Day, time.Local)
	if err != nil {
		m.lastErr = fmt.Errorf("bad day in todo %q", item.Day)
		return m, nil
	}
	changedJournal := item.Journal != "" && item.Journal != m.journal
	m.closeOverlay()
	m.journal = item.Journal
	m.selected = day
	m.month = firstOfMonth(day)
	m.scroll = 0
	cmd := m.editDay()
	if cmd == nil {
		cmd = m.fullLoad()
	}
	if changedJournal {
		cmd = tea.Batch(cmd, savePrefsCmd(m.themeName, m.journal, "journal"))
	}
	return m, cmd
}

// remindSelectedTodo opens the reminder form prefilled with the item text.
func (m model) remindSelectedTodo() (tea.Model, tea.Cmd) {
	if len(m.todos.items) == 0 {
		return m, nil
	}
	item := m.todos.items[m.todos.cursor]
	text := strings.TrimSpace(item.Text)
	if text == "" {
		text = "todo"
	}
	return m.openReminder(text)
}

// todoLine renders one todo row: checkbox, journal, day, time and text.
func (m model) todoLine(item jot.TodoItem, width int) string {
	box := "[ ]"
	if item.Done {
		box = "[x]"
	}
	at := item.Time
	if at == "" {
		at = "        "
	}
	line := fmt.Sprintf("%s  %-10s %-10s %-8s %s", box, item.Journal, item.Day, at, item.Text)
	return ansiTruncate(line, max(width, 1))
}

// todosView renders the aggregated todo overlay.
func (m model) todosView(lay layout) string {
	rect := m.todosRect(lay)
	innerW := max(rect.w-2, 1)
	width := max(innerW-1, 1)
	scope := "all journals"
	if !m.todos.scopeAll {
		scope = "this journal (" + m.journal + ")"
	}
	open, done := 0, 0
	for _, item := range m.todos.items {
		if item.Done {
			done++
		} else {
			open++
		}
	}
	scopeLine := m.theme.muted().Render(fmt.Sprintf(
		"scope: %s · tab scope · %d open · %d done · jot never toggles boxes, enter edits",
		scope, open, done))
	header := m.theme.dim().Render(fmt.Sprintf("%-4s %-10s %-10s %-8s %s", "BOX", "JOURNAL", "DAY", "TIME", "ITEM"))
	content := []string{scopeLine, header}

	rows := m.todosVisibleRows(lay)
	if len(m.todos.items) == 0 {
		msg := "No checkboxes found. Write - [ ] items in any journal entry."
		if m.todos.loading {
			msg = "Loading todos…"
		}
		if m.todos.err != nil {
			msg = "Could not read todos: " + m.todos.err.Error()
		}
		content = append(content, m.theme.muted().Render(msg))
		for len(content) < rect.h-2 {
			content = append(content, "")
		}
		return m.overlayCanvas(lay, rect, m.boxLines("TODOS", rect, content))
	}

	end := min(m.todos.offset+rows, len(m.todos.items))
	for i := m.todos.offset; i < end; i++ {
		item := m.todos.items[i]
		line := m.todoLine(item, width)
		switch {
		case i == m.todos.cursor:
			line = m.theme.selStyle().Render(padRight(line, width))
		case item.Done:
			line = m.theme.dim().Render(padRight(line, width))
		default:
			line = padRight(line, width)
		}
		content = append(content, line)
	}
	for len(content) < rect.h-2 {
		content = append(content, "")
	}
	return m.overlayCanvas(lay, rect, m.boxLines("TODOS", rect, content))
}

// todoRowAt maps a mouse coordinate to a todo index in the window.
func (m model) todoRowAt(lay layout, x, y int) (int, bool) {
	rect := m.todosRect(lay)
	if x < rect.x || x >= rect.x+rect.w {
		return 0, false
	}
	row := contentRow(y, rect)
	if row < todoResultRow || row >= rect.h-2 {
		return 0, false
	}
	idx := m.todos.offset + row - todoResultRow
	if idx < 0 || idx >= len(m.todos.items) {
		return 0, false
	}
	return idx, true
}

// handleTodosMouse scrolls the list and opens clicked items in the editor.
func (m model) handleTodosMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	lay := m.layout()
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		m.scrollTodos(-3)
		return m, nil
	case tea.MouseButtonWheelDown:
		m.scrollTodos(3)
		return m, nil
	case tea.MouseButtonLeft:
		if msg.Y == 0 || bodyRow(msg.Y) >= lay.paneH {
			m.closeOverlay()
			return m, nil
		}
		if idx, ok := m.todoRowAt(lay, msg.X, msg.Y); ok {
			m.todos.cursor = idx
			m.ensureTodosVisible()
			return m.editSelectedTodo()
		}
	}
	return m, nil
}

package tui

import (
	"fmt"
	"slices"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/moneytosms/jot"
)

// Below-calendar widgets, each one hideable from Settings (tui.hide).
const (
	widgetOnThisDay = "onthisday"
	widgetTodos     = "todos"
	widgetReminders = "reminders"
)

// widgetNames lists the widgets in display order with their settings labels.
var widgetNames = []struct{ id, label string }{
	{widgetOnThisDay, "On this day"},
	{widgetTodos, "Open todos"},
	{widgetReminders, "Reminders"},
}

// memory is an entry written on today's date in an earlier year.
type memory struct {
	day  string
	text string
}

// widgetData is what the widgets show, loaded with every full reload.
type widgetData struct {
	memories  []memory
	todos     []jot.TodoItem // the first few open todos
	openTodos int            // all open todos in the journal
	reminders []jot.Reminder
}

// loadWidgets gathers widget data for a journal. Every source is best
// effort: a failure just leaves that widget empty.
func loadWidgets(j *jot.Jot, journal string, days []string, today time.Time) widgetData {
	var w widgetData
	suffix := today.Format("-01-02")
	for i := len(days) - 1; i >= 0 && len(w.memories) < 3; i-- {
		d := days[i]
		if !strings.HasSuffix(d, suffix) || d >= dayKey(today) {
			continue
		}
		content, err := j.Show(journal, d)
		if err != nil || len(content) == 0 {
			continue
		}
		for _, e := range jot.ParseEntries(content[0]) {
			if line := firstLine(e.Body); line != "" {
				w.memories = append(w.memories, memory{day: d, text: line})
				break
			}
		}
	}
	if items, err := j.TodosJournal(journal); err == nil {
		for _, it := range items {
			if it.Done {
				continue
			}
			w.openTodos++
			if len(w.todos) < 3 {
				w.todos = append(w.todos, it)
			}
		}
	}
	if rs, err := j.Reminders(); err == nil {
		for _, r := range rs {
			if r.At.After(time.Now()) {
				w.reminders = append(w.reminders, r)
			}
		}
		slices.SortFunc(w.reminders, func(a, b jot.Reminder) int { return a.At.Compare(b.At) })
		w.reminders = w.reminders[:min(len(w.reminders), 2)]
	}
	return w
}

// widgetOn reports whether a widget is enabled.
func (m model) widgetOn(id string) bool { return !slices.Contains(m.cfg.TUI.Hide, id) }

// sideLine is one row of the block under the calendar. Clicking it sends key
// (a shortcut) or jumps to day.
type sideLine struct {
	text string
	key  string
	day  string
}

// sideBlock builds the rows under the calendar for room rows: the legend,
// whichever widgets have something to say and fit whole, then the shortcuts.
func (m model) sideBlock(room int) []sideLine {
	width := calW
	out := []sideLine{{text: m.legendLine()}}
	var shortcuts []sideLine
	key := m.theme.accent().Bold(true)
	for _, e := range sideExtras {
		shortcuts = append(shortcuts, sideLine{text: key.Render(padRight(e.key, 2)) + m.theme.muted().Render(e.label), key: e.key})
	}
	left := room - len(out) - 1 - len(shortcuts) // one blank before the shortcuts
	add := func(block []sideLine) {
		if len(block) > 0 && len(block) <= left {
			out = append(out, block...)
			left -= len(block)
		}
	}
	head := func(s string) sideLine { return sideLine{text: m.theme.dim().Bold(true).Render(s)} }
	if m.widgetOn(widgetOnThisDay) && len(m.widgets.memories) > 0 {
		block := []sideLine{{}, head("ON THIS DAY")}
		for _, mem := range m.widgets.memories {
			text := m.theme.accent().Render(mem.day[:4]) + " " + m.theme.muted().Render(mem.text)
			block = append(block, sideLine{text: ansiTruncate(text, width), day: mem.day})
		}
		add(block)
	}
	if m.widgetOn(widgetTodos) && len(m.widgets.todos) > 0 {
		block := []sideLine{{}, {text: m.theme.dim().Bold(true).Render("OPEN TODOS ") + m.theme.dim().Render(fmt.Sprint(m.widgets.openTodos))}}
		for _, it := range m.widgets.todos {
			text := m.theme.muted().Render("☐ ") + m.theme.fg().Render(it.Text)
			block = append(block, sideLine{text: ansiTruncate(text, width), day: it.Day})
		}
		add(block)
	}
	if m.widgetOn(widgetReminders) && len(m.widgets.reminders) > 0 {
		block := []sideLine{{}, head("REMINDERS")}
		for _, r := range m.widgets.reminders {
			at := r.At.Format("15:04")
			if dayKey(r.At) != dayKey(m.today) {
				at = r.At.Format("Mon 15:04")
			}
			msg := r.Message
			if msg == "" {
				msg = "reminder"
			}
			block = append(block, sideLine{text: ansiTruncate(m.theme.accent().Render(at)+" "+m.theme.muted().Render(msg), width)})
		}
		add(block)
	}
	out = append(out, sideLine{})
	return append(out, shortcuts...)
}

// gotoDay selects a "YYYY-MM-DD" day, changing month when needed.
func (m model) gotoDay(day string) (tea.Model, tea.Cmd) {
	t, err := time.ParseInLocation("2006-01-02", day, time.Local)
	if err != nil {
		return m, nil
	}
	m.selected = t
	m.scroll = 0
	if monthKey(t) != monthKey(m.month) {
		m.month = firstOfMonth(t)
		return m, m.fullLoad()
	}
	return m, m.dayLoad()
}

// journalColors are the color names a journal can take, in cycle order;
// "" means the default look.
var journalColors = []string{"", "green", "yellow", "cyan", "magenta", "red", "blue"}

// journalColor returns the hex color chosen for a journal, or "".
func (m model) journalColor(name string) string {
	switch m.cfg.TUI.Colors[name] {
	case "green":
		return m.theme.Green
	case "yellow":
		return m.theme.Yellow
	case "cyan":
		return m.theme.Cyan
	case "magenta":
		return m.theme.Magenta
	case "red":
		return m.theme.Red
	case "blue":
		return m.theme.Blue
	}
	return ""
}

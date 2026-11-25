package jot

import (
	"bufio"
	"path/filepath"
	"slices"
	"strings"
)

// TodoItem is one markdown checkbox item found in a journal, like "- [ ] call
// the dentist". Jot entries are plain markdown, so users may write checkboxes
// by hand and this simply reads what is already on disk.
type TodoItem struct {
	// Journal is the journal the item belongs to.
	Journal string
	// Day is the day label of the item, YYYY-MM-DD.
	Day string
	// Time is the "## HH:MM:SS" heading of the entry containing the item, or
	// empty when the item appears before any entry heading.
	Time string
	// Text is the item text, everything after the checkbox brackets.
	Text string
	// Done reports whether the box is ticked.
	Done bool
}

// Location names where the item lives, "journal/day", for display.
func (t TodoItem) Location() string { return t.Journal + "/" + t.Day }

// Todos returns every checkbox item in every journal, newest day first, then
// entry time, then file order. A missing root means no items, not an error.
func (j *Jot) Todos() ([]TodoItem, error) {
	infos, err := j.List()
	if err != nil {
		return nil, err
	}
	out := make([]TodoItem, 0, len(infos))
	for _, info := range infos {
		items, err := j.TodosJournal(info.Name)
		if err != nil {
			return nil, err
		}
		out = append(out, items...)
	}
	sortTodos(out)
	return out, nil
}

// TodosJournal returns every checkbox item in one journal, newest day first,
// then entry time, then file order. A missing journal wraps ErrNoJournal.
func (j *Jot) TodosJournal(name string) ([]TodoItem, error) {
	journal := j.journalName(name)
	days, err := j.Days(name)
	if err != nil {
		return nil, err
	}
	out := make([]TodoItem, 0, len(days))
	for _, day := range days {
		items, err := scanDayTodos(journal, day, filepath.Join(j.root, journal, day+".md"))
		if err != nil {
			return nil, err
		}
		out = append(out, items...)
	}
	sortTodos(out)
	return out, nil
}

// sortTodos orders items newest day first, then by entry time. The sort is
// stable so items from the same day and time keep their file order.
func sortTodos(items []TodoItem) {
	slices.SortStableFunc(items, func(a, b TodoItem) int {
		if c := strings.Compare(b.Day, a.Day); c != 0 {
			return c
		}
		return strings.Compare(a.Time, b.Time)
	})
}

// scanDayTodos collects checkbox items from one day file, tracking the current
// entry heading so each item can name its time.
func scanDayTodos(journal, day, path string) ([]TodoItem, error) {
	f, err := openDayFile(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	at := ""
	var out []TodoItem
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "## ") {
			at = strings.TrimSpace(strings.TrimPrefix(line, "## "))
			continue
		}
		if item, ok := parseTodo(line); ok {
			item.Journal = journal
			item.Day = day
			item.Time = at
			out = append(out, item)
		}
	}
	return out, scanner.Err()
}

// parseTodo recognizes a checkbox line: optionally indented, starting with "-"
// or "*", then a space, then an empty, "x" or "X" box. As in markdown, the box
// must be followed by a space or the end of the line; everything after that,
// trimmed, is the item text.
func parseTodo(line string) (TodoItem, bool) {
	s := strings.TrimSpace(line)
	if len(s) < 4 || (s[0] != '-' && s[0] != '*') {
		return TodoItem{}, false
	}
	box := strings.TrimLeft(s[1:], " ")
	if len(box) < 3 || box[0] != '[' || box[2] != ']' {
		return TodoItem{}, false
	}
	switch box[1] {
	case ' ':
	case 'x', 'X':
	default:
		return TodoItem{}, false
	}
	rest := box[3:]
	if rest != "" && rest[0] != ' ' {
		return TodoItem{}, false
	}
	return TodoItem{Text: strings.TrimSpace(rest), Done: box[1] != ' '}, true
}

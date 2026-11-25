package jot

import (
	"errors"
	"testing"
)

func TestTodos_Detection(t *testing.T) {
	j := testJot(t)
	writeDay(t, j, "work", "2026-01-02", "Preamble plan:\n"+
		"- [ ] untimed task\n"+
		"* [ ] bullet task\n"+
		"  - [x] indented done\n"+
		"  - [X] indented uppercase\n"+
		"- [x]no space after box\n"+ // not a checkbox in markdown
		"- plain bullet\n"+
		"[ ] not a checkbox\n"+
		"## 09:00:00\n"+
		"- [ ] timed task\n"+
		"- [x]\n")

	items, err := j.TodosJournal("work")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 6 {
		t.Fatalf("expected 6 items, got %d: %+v", len(items), items)
	}
	// Newest day first, then entry time, then file order: untimed items come
	// before the 09:00:00 entry.
	want := []struct {
		time, text string
		done       bool
	}{
		{"", "untimed task", false},
		{"", "bullet task", false},
		{"", "indented done", true},
		{"", "indented uppercase", true},
		{"09:00:00", "timed task", false},
		{"09:00:00", "", true},
	}
	for i, w := range want {
		got := items[i]
		if got.Time != w.time || got.Text != w.text || got.Done != w.done {
			t.Errorf("item %d = %+v, want time %q text %q done %v", i, got, w.time, w.text, w.done)
		}
		if got.Journal != "work" || got.Day != "2026-01-02" {
			t.Errorf("item %d has wrong journal/day: %+v", i, got)
		}
	}
}

func TestTodos_AcrossJournalsAndSortOrder(t *testing.T) {
	j := testJot(t)
	writeDay(t, j, "default", "2026-02-01", "## 08:00:00\n- [ ] newer day\n")
	writeDay(t, j, "default", "2026-01-15", "## 07:00:00\n- [ ] older day\n")
	writeDay(t, j, "work", "2026-02-01", "## 09:00:00\n- [ ] same day later\n## 08:00:00\n- [ ] same day earlier\n")

	items, err := j.Todos()
	if err != nil {
		t.Fatal(err)
	}
	// Sorted by day newest first, then entry time, regardless of journal:
	// the same day mixes journals by time, not by journal.
	want := []struct {
		journal, time, text string
	}{
		{"default", "08:00:00", "newer day"},
		{"work", "08:00:00", "same day earlier"},
		{"work", "09:00:00", "same day later"},
		{"default", "07:00:00", "older day"},
	}
	if len(items) != len(want) {
		t.Fatalf("expected %d items, got %d: %+v", len(want), len(items), items)
	}
	for i, w := range want {
		if items[i].Journal != w.journal || items[i].Time != w.time || items[i].Text != w.text {
			t.Errorf("item %d = %+v, want %s/%s %q", i, items[i], w.journal, w.time, w.text)
		}
	}
	if items[0].Location() != "default/2026-02-01" {
		t.Errorf("unexpected Location: %q", items[0].Location())
	}
}

func TestTodosJournal_ScopesToOneJournal(t *testing.T) {
	j := testJot(t)
	writeDay(t, j, "default", "2026-01-01", "- [ ] default task\n")
	writeDay(t, j, "work", "2026-01-01", "- [ ] work task\n")

	items, err := j.TodosJournal("work")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Journal != "work" || items[0].Text != "work task" {
		t.Fatalf("unexpected scoped items: %+v", items)
	}
}

func TestTodosJournal_MissingJournal(t *testing.T) {
	j := testJot(t)
	if _, err := j.TodosJournal("nope"); !errors.Is(err, ErrNoJournal) {
		t.Fatalf("expected ErrNoJournal, got %v", err)
	}
}

func TestTodosJournal_EmptyJournalIsNotAnError(t *testing.T) {
	j := testJot(t)
	if err := j.Init(); err != nil {
		t.Fatal(err)
	}
	items, err := j.TodosJournal("")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("expected no items, got %+v", items)
	}
}

func TestTodos_EmptyRoot(t *testing.T) {
	j := testJot(t)
	items, err := j.Todos()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("expected no items, got %+v", items)
	}
}

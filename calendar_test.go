package jot

import (
	"errors"
	"testing"
	"time"
)

func TestMonthDays(t *testing.T) {
	j := testJot(t)
	writeDay(t, j, "work", "2026-01-30", "## 08:00:00\nmorning\n\n## 21:00:00\nnight\n")
	writeDay(t, j, "work", "2026-01-31", "## 12:00:00\nlunch\n")
	writeDay(t, j, "work", "2026-02-01", "## 12:00:00\nnext month\n")
	writeDay(t, j, "work", "notes", "## 12:00:00\nnot a date\n")

	days, err := j.MonthDays("work", 2026, time.January)
	if err != nil {
		t.Fatal(err)
	}
	if len(days) != 2 {
		t.Fatalf("expected 2 days, got %d: %+v", len(days), days)
	}
	if days[0].Day != "2026-01-30" || days[1].Day != "2026-01-31" {
		t.Fatalf("days not sorted: %+v", days)
	}
	if days[0].Entries != 2 {
		t.Errorf("expected 2 entries on 2026-01-30, got %d", days[0].Entries)
	}
	if days[1].Entries != 1 {
		t.Errorf("expected 1 entry on 2026-01-31, got %d", days[1].Entries)
	}
	if days[0].Bytes == 0 {
		t.Errorf("expected a non-zero byte count: %+v", days[0])
	}
}

func TestMonthDays_MissingJournal(t *testing.T) {
	j := testJot(t)
	if _, err := j.MonthDays("nope", 2026, time.January); !errors.Is(err, ErrNoJournal) {
		t.Fatalf("expected ErrNoJournal, got %v", err)
	}
}

func TestMonthDays_MonthWithoutFiles(t *testing.T) {
	j := testJot(t)
	writeDay(t, j, "work", "2026-01-02", "## 12:00:00\nnote\n")

	days, err := j.MonthDays("work", 2026, time.March)
	if err != nil {
		t.Fatal(err)
	}
	if len(days) != 0 {
		t.Fatalf("expected no days, got %+v", days)
	}
}

func TestMonths(t *testing.T) {
	j := testJot(t)
	writeDay(t, j, "work", "2026-01-02", "## 12:00:00\nnote\n")
	writeDay(t, j, "work", "2026-01-30", "## 12:00:00\nnote\n")
	writeDay(t, j, "work", "2025-12-31", "## 12:00:00\nnote\n")
	writeDay(t, j, "work", "2026-03-01", "## 12:00:00\nnote\n")
	writeDay(t, j, "work", "scratchpad", "## 12:00:00\nnot a date\n")

	months, err := j.Months("work")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"2026-03", "2026-01", "2025-12"}
	if len(months) != len(want) {
		t.Fatalf("expected %d months, got %d: %+v", len(want), len(months), months)
	}
	for i, w := range want {
		if months[i] != w {
			t.Errorf("month %d = %q, want %q", i, months[i], w)
		}
	}
}

func TestMonths_MissingJournal(t *testing.T) {
	j := testJot(t)
	if _, err := j.Months("nope"); !errors.Is(err, ErrNoJournal) {
		t.Fatalf("expected ErrNoJournal, got %v", err)
	}
}

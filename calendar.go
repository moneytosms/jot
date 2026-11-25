package jot

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// DayCount summarizes one day of a journal for a calendar grid.
type DayCount struct {
	// Day is the day label, YYYY-MM-DD.
	Day string
	// Entries is the number of entries recorded that day.
	Entries int
	// Bytes is the size of the day file on disk.
	Bytes int64
}

// MonthDays counts the entries of every day of a journal in the given month
// that has a file, sorted by day. Files whose name is not a date in that month
// are skipped silently. A missing journal wraps ErrNoJournal.
func (j *Jot) MonthDays(name string, year int, month time.Month) ([]DayCount, error) {
	if err := j.checkJournal(name); err != nil {
		return nil, err
	}
	journal := j.journalName(name)
	dir := filepath.Join(j.root, journal)
	files, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrNoJournal, journal)
		}
		return nil, err
	}
	prefix := fmt.Sprintf("%04d-%02d-", year, int(month))
	out := make([]DayCount, 0, len(files))
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".md") {
			continue
		}
		day := strings.TrimSuffix(f.Name(), ".md")
		if !strings.HasPrefix(day, prefix) {
			continue
		}
		if _, err := time.Parse("2006-01-02", day); err != nil {
			continue
		}
		path := filepath.Join(dir, f.Name())
		info := DayCount{Day: day}
		if st, err := os.Stat(path); err == nil {
			info.Bytes = st.Size()
		}
		if n, err := countEntries(path); err == nil {
			info.Entries = n
		}
		out = append(out, info)
	}
	slices.SortFunc(out, func(a, b DayCount) int {
		return strings.Compare(a.Day, b.Day)
	})
	return out, nil
}

// Months returns the distinct "YYYY-MM" labels of the days recorded in a
// journal, newest first. This drives month-jumping in the TUI.
func (j *Jot) Months(name string) ([]string, error) {
	days, err := j.Days(name)
	if err != nil {
		return nil, err
	}
	// Days comes back sorted, so labels appear in ascending order already.
	var out []string
	for _, day := range days {
		if len(day) < 7 {
			continue
		}
		label := day[:7]
		if _, err := time.Parse("2006-01", label); err != nil {
			continue
		}
		if len(out) == 0 || out[len(out)-1] != label {
			out = append(out, label)
		}
	}
	slices.Reverse(out)
	return out, nil
}

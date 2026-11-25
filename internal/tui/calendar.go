package tui

import "time"

// Cell is one day cell of a month grid. Padding cells outside the month have
// Day 0 and a zero Date.
type Cell struct {
	// Day is the day-of-month, 0 for padding.
	Day int
	// Date is the cell date at midnight local time, zero for padding.
	Date time.Time
	// Level grades how many entries the day has, 0 (none) to 4; see
	// EntryLevel.
	Level int
	// IsToday and IsSelected flag the special cells.
	IsToday, IsSelected bool
}

// Week is one calendar row, Monday first.
type Week struct {
	Days [7]Cell
}

// MonthGrid computes the weeks of the displayed month: seven cells per week,
// Monday first, padded with empty cells at both ends. today and selected set
// the IsToday and IsSelected flags; counts maps "YYYY-MM-DD" labels to entry
// counts. The grid contains only days of the displayed month, so the view
// code can render it straight into the Theme palette.
func MonthGrid(month, today, selected time.Time, counts map[string]int) []Week {
	first := firstOfMonth(month)
	offset := (int(first.Weekday()) + 6) % 7 // Monday-first index
	days := daysInMonth(first.Year(), first.Month())
	nWeeks := (offset + days + 6) / 7
	out := make([]Week, 0, nWeeks)

	selKey, todayKey := dayKey(selected), dayKey(today)
	// day walks the grid cell by cell, starting on the Monday of the first
	// week.
	day := first.AddDate(0, 0, -offset)
	for w := 0; w < nWeeks; w++ {
		var week Week
		for d := 0; d < 7; d++ {
			if day.Month() != first.Month() || day.Year() != first.Year() {
				// Padding before the 1st or after the last day.
				day = day.AddDate(0, 0, 1)
				continue
			}
			key := day.Format("2006-01-02")
			week.Days[d] = Cell{
				Day:        day.Day(),
				Date:       day,
				Level:      EntryLevel(counts[key]),
				IsToday:    key == todayKey,
				IsSelected: key == selKey,
			}
			day = day.AddDate(0, 0, 1)
		}
		out = append(out, week)
	}
	return out
}

// EntryLevel maps an entry count to a 0-4 visual heat level used to color
// calendar cells.
func EntryLevel(entries int) int {
	switch {
	case entries <= 0:
		return 0
	case entries <= 2:
		return 1
	case entries <= 5:
		return 2
	case entries <= 9:
		return 3
	default:
		return 4
	}
}

// firstOfMonth returns the first day of t's month at midnight.
func firstOfMonth(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
}

// daysInMonth returns the number of days in the given month.
func daysInMonth(year int, month time.Month) int {
	return time.Date(year, month, 1, 0, 0, 0, 0, time.Local).AddDate(0, 1, -1).Day()
}

// dayKey formats a date as its "YYYY-MM-DD" journal day label.
func dayKey(t time.Time) string { return t.Format("2006-01-02") }

// monthKey formats a date as its "YYYY-MM" month label.
func monthKey(t time.Time) string { return t.Format("2006-01") }

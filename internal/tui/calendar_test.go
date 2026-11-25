package tui

import (
	"testing"
	"time"
)

// monthsStartingOn covers one month for every Monday-first offset:
// Sun 2024-09, Mon 2025-09, Tue 2025-07, Wed 2025-10, Thu 2025-05,
// Fri 2025-08, Sat 2025-02.
var monthsStartingOn = []struct {
	offset int // Monday-first index of the 1st
	year   int
	month  time.Month
}{
	{6, 2024, time.September},
	{0, 2025, time.September},
	{1, 2025, time.July},
	{2, 2025, time.October},
	{3, 2025, time.May},
	{4, 2025, time.August},
	{5, 2025, time.February},
}

func TestMonthGridWeekdayOffsets(t *testing.T) {
	for _, tc := range monthsStartingOn {
		first := time.Date(tc.year, tc.month, 1, 0, 0, 0, 0, time.Local)
		grid := MonthGrid(first, time.Time{}, time.Time{}, nil)

		if got := (int(first.Weekday()) + 6) % 7; got != tc.offset {
			t.Errorf("%s: offset = %d, want %d", first.Format("2006-01"), got, tc.offset)
		}
		if n := len(grid); n != (tc.offset+daysInMonth(tc.year, tc.month)+6)/7 {
			t.Errorf("%s: weeks = %d, want %d", first.Format("2006-01"), n, (tc.offset+daysInMonth(tc.year, tc.month)+6)/7)
		}
		// The 1st sits in the first week at the offset column; padding before.
		if cell := grid[0].Days[tc.offset]; cell.Day != 1 {
			t.Errorf("%s: first week offset column = day %d, want 1", first.Format("2006-01"), cell.Day)
		}
		for d := 0; d < tc.offset; d++ {
			if cell := grid[0].Days[d]; cell.Day != 0 || !cell.Date.IsZero() {
				t.Errorf("%s: leading padding cell %d = %+v, want empty", first.Format("2006-01"), d, cell)
			}
		}
		// Every week has exactly 7 cells and days advance one per cell.
		prev := 0
		for w, week := range grid {
			if len(week.Days) != 7 {
				t.Fatalf("%s: week %d has %d cells, want 7", first.Format("2006-01"), w, len(week.Days))
			}
			for d, cell := range week.Days {
				if cell.Day == 0 {
					if prev < daysInMonth(tc.year, tc.month) && w == len(grid)-1 {
						continue // trailing padding
					}
					if prev != 0 && prev < daysInMonth(tc.year, tc.month) {
						t.Errorf("%s: empty cell inside month at week %d day %d", first.Format("2006-01"), w, d)
					}
					continue
				}
				if prev != 0 && cell.Day != prev+1 {
					t.Errorf("%s: day %d follows %d at week %d day %d", first.Format("2006-01"), cell.Day, prev, w, d)
				}
				prev = cell.Day
			}
		}
		if prev != daysInMonth(tc.year, tc.month) {
			t.Errorf("%s: last day cell = %d, want %d", first.Format("2006-01"), prev, daysInMonth(tc.year, tc.month))
		}
		// Trailing padding after the last day.
		last := grid[len(grid)-1]
		for d := 6; d >= 0; d-- {
			if last.Days[d].Day != 0 {
				break
			}
			if !last.Days[d].Date.IsZero() {
				t.Errorf("%s: trailing padding cell not empty", first.Format("2006-01"))
			}
		}
	}
}

func TestMonthGridFebruary(t *testing.T) {
	leap := MonthGrid(time.Date(2024, time.February, 1, 0, 0, 0, 0, time.Local), time.Time{}, time.Time{}, nil)
	nonLeap := MonthGrid(time.Date(2025, time.February, 1, 0, 0, 0, 0, time.Local), time.Time{}, time.Time{}, nil)

	if n := len(leap); n != 5 {
		t.Errorf("2024-02 weeks = %d, want 5", n)
	}
	if n := len(nonLeap); n != 5 {
		t.Errorf("2025-02 weeks = %d, want 5", n)
	}
	if !containsDay(leap, 29) {
		t.Errorf("2024-02 grid missing day 29")
	}
	if containsDay(nonLeap, 29) {
		t.Errorf("2025-02 grid should not contain day 29")
	}
}

// containsDay reports whether the grid holds a cell for the given day.
func containsDay(grid []Week, day int) bool {
	for _, week := range grid {
		for _, cell := range week.Days {
			if cell.Day == day {
				return true
			}
		}
	}
	return false
}

func TestMonthGridFlagsAndLevels(t *testing.T) {
	first := time.Date(2025, time.February, 1, 0, 0, 0, 0, time.Local)
	today := time.Date(2025, time.February, 10, 0, 0, 0, 0, time.Local)
	selected := time.Date(2025, time.February, 17, 0, 0, 0, 0, time.Local)
	counts := map[string]int{
		"2025-02-03": 1,  // level 1
		"2025-02-04": 4,  // level 2
		"2025-02-05": 7,  // level 3
		"2025-02-06": 20, // level 4
	}
	grid := MonthGrid(first, today, selected, counts)

	for _, week := range grid {
		for _, cell := range week.Days {
			if cell.Day == 0 {
				continue
			}
			want := EntryLevel(counts[dayKey(cell.Date)])
			if cell.Level != want {
				t.Errorf("day %d level = %d, want %d", cell.Day, cell.Level, want)
			}
			if cell.IsToday != (cell.Day == 10) {
				t.Errorf("day %d IsToday = %v", cell.Day, cell.IsToday)
			}
			if cell.IsSelected != (cell.Day == 17) {
				t.Errorf("day %d IsSelected = %v", cell.Day, cell.IsSelected)
			}
			if !cell.Date.Equal(time.Date(2025, time.February, cell.Day, 0, 0, 0, 0, time.Local)) {
				t.Errorf("day %d date = %v", cell.Day, cell.Date)
			}
		}
	}
	// Days of other months never carry flags, even if their label matches.
	other := MonthGrid(time.Date(2025, time.March, 1, 0, 0, 0, 0, time.Local), today, selected, counts)
	for _, week := range other {
		for _, cell := range week.Days {
			if cell.Day != 0 && (cell.IsToday || cell.IsSelected) {
				t.Errorf("March grid flagged day %d", cell.Day)
			}
		}
	}
}

func TestEntryLevel(t *testing.T) {
	cases := map[int]int{0: 0, 1: 1, 2: 1, 3: 2, 5: 2, 6: 3, 9: 3, 10: 4, 999: 4, -3: 0}
	for n, want := range cases {
		if got := EntryLevel(n); got != want {
			t.Errorf("EntryLevel(%d) = %d, want %d", n, got, want)
		}
	}
}

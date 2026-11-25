package jot

import (
	"fmt"
	"strings"
	"time"
)

// Heat glyphs, one per level 0-3. The ramp runs from a faint dot to a full
// block, so a grid stays readable with and without color.
//
//	·· none   ▄▄ 1-2   ▓▓ 3-5   ██ 6+
const HeatNone = "··"

// HeatGlyphs holds the two-character cell for each heat level, index 0-3.
var HeatGlyphs = [4]string{HeatNone, "▄▄", "▓▓", "██"}

// HeatLevel grades an entry count to a 0-3 heat level: 0 none, 1 one or two
// entries, 2 three to five, 3 six or more. The thresholds match what one
// day of writing usually looks like, not a percentile.
func HeatLevel(entries int) int {
	switch {
	case entries <= 0:
		return 0
	case entries <= 2:
		return 1
	case entries <= 5:
		return 2
	default:
		return 3
	}
}

// HeatPainter paints one two-character heatmap cell. Implementations add
// color; the layout never depends on it.
type HeatPainter interface {
	// Cell renders the cell for a level, 0-3, exactly two characters wide.
	Cell(level int) string
}

// PlainHeatPainter renders the uncolored glyph ramp. It is the painter for
// pipes, NO_COLOR and tests.
type PlainHeatPainter struct{}

// Cell implements HeatPainter.
func (PlainHeatPainter) Cell(level int) string {
	if level < 0 || level >= len(HeatGlyphs) {
		return HeatGlyphs[0]
	}
	return HeatGlyphs[level]
}

const (
	heatLabelW  = 4 // month label column, "Jan " or blanks
	heatCellGap = 1 // space between weekday cells
	heatGutter  = 3 // space between the two half-year grids
)

// HeatmapLines renders one year of entry counts as two side-by-side
// half-year grids: weeks as rows, Monday-first weekdays as columns, and a
// month label on the row where that month begins. Days outside the year are
// blank. Every line is exactly
// 2*heatLabelW+2*(7*2+6*heatCellGap)+heatGutter columns wide.
//
// counts maps "YYYY-MM-DD" day labels to entry counts. p decides the color;
// PlainHeatPainter keeps the output deterministic for tests and pipes.
func HeatmapLines(year int, counts map[string]int, now time.Time, p HeatPainter) []string {
	if p == nil {
		p = PlainHeatPainter{}
	}
	header := "    " + heatWeekHeader() + strings.Repeat(" ", heatGutter) + "    " + heatWeekHeader()
	out := []string{header}
	left := heatHalf(year, time.January, time.June, counts, p)
	right := heatHalf(year, time.July, time.December, counts, p)
	for i := 0; i < max(len(left), len(right)); i++ {
		l, r := "", ""
		if i < len(left) {
			l = left[i]
		} else {
			l = strings.Repeat(" ", heatHalfWidth())
		}
		if i < len(right) {
			r = right[i]
		}
		out = append(out, l+strings.Repeat(" ", heatGutter)+r)
	}
	return out
}

// HeatLegend returns the legend line for a painted grid, explaining the
// glyph ramp.
func HeatLegend(p HeatPainter) string {
	if p == nil {
		p = PlainHeatPainter{}
	}
	return fmt.Sprintf("legend: %s 0 · %s 1-2 · %s 3-5 · %s 6+ entries",
		p.Cell(0), p.Cell(1), p.Cell(2), p.Cell(3))
}

// heatWeekHeader renders the Monday-first weekday labels, matching the cell
// column widths.
func heatWeekHeader() string {
	return "Mo Tu We Th Fr Sa Su"
}

// heatHalfWidth is the printable width of one half-year grid: the month
// label plus seven two-character cells with gaps.
func heatHalfWidth() int {
	return heatLabelW + 7*2 + 6*heatCellGap
}

// heatHalf renders the months from start to end of one year as week rows.
// The row for a week is the month label (when that week contains the first
// of a month in range) followed by seven cells; days outside the half are
// blank. The first row starts on the Monday on or before the 1st, the last
// ends on the Sunday on or after the month's final day.
func heatHalf(year int, start, end time.Month, counts map[string]int, p HeatPainter) []string {
	first := time.Date(year, start, 1, 0, 0, 0, 0, time.Local)
	last := time.Date(year, end, 1, 0, 0, 0, 0, time.Local).AddDate(0, 1, -1)
	rowStart := first.AddDate(0, 0, -((int(first.Weekday()) + 6) % 7)) // back to Monday
	var out []string
	for !rowStart.After(last) {
		label := heatRowLabel(year, start, end, rowStart)
		var cells strings.Builder
		for i := 0; i < 7; i++ {
			if i > 0 {
				cells.WriteString(strings.Repeat(" ", heatCellGap))
			}
			day := rowStart.AddDate(0, 0, i)
			if day.Year() == year && !day.Before(first) && !day.After(last) {
				cells.WriteString(p.Cell(HeatLevel(counts[day.Format("2006-01-02")])))
			} else {
				cells.WriteString("  ")
			}
		}
		out = append(out, label+cells.String())
		rowStart = rowStart.AddDate(0, 0, 7)
	}
	return out
}

// heatRowLabel builds the four-character month label of one week row: the
// abbreviation of the month in range whose 1st falls in this week, else
// blanks.
func heatRowLabel(year int, start, end time.Month, rowStart time.Time) string {
	for m := start; m <= end; m++ {
		first := time.Date(year, m, 1, 0, 0, 0, 0, time.Local)
		if !first.Before(rowStart) && first.Before(rowStart.AddDate(0, 0, 7)) {
			return fmt.Sprintf("%-4s", first.Format("Jan"))
		}
	}
	return strings.Repeat(" ", heatLabelW)
}

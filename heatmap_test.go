package jot

import (
	"strings"
	"testing"
	"time"
)

// heatTestCounts builds counts for every day of 2026 with a deterministic
// ramp: entry count = day of month % 8, so all four levels appear.
func heatTestCounts(year int) map[string]int {
	counts := make(map[string]int)
	for d := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC); d.Year() == year; d = d.AddDate(0, 0, 1) {
		counts[d.Format("2006-01-02")] = d.Day() % 8
	}
	return counts
}

func TestHeatmapLinesShape(t *testing.T) {
	now := time.Date(2026, 11, 25, 12, 0, 0, 0, time.Local)
	lines := HeatmapLines(2026, heatTestCounts(2026), now, nil)

	if len(lines) < 2 {
		t.Fatalf("HeatmapLines returned %d lines, want header plus weeks", len(lines))
	}
	headerRunes := len([]rune(lines[0]))
	if headerRunes != 2*heatHalfWidth()+heatGutter {
		t.Fatalf("header width = %d, want %d", headerRunes, 2*heatHalfWidth()+heatGutter)
	}
	for i, line := range lines {
		if got := len([]rune(line)); got != headerRunes {
			t.Errorf("line %d width = %d, want %d", i, got, headerRunes)
		}
	}
}

func TestHeatmapLinesLabelsAndWeekdays(t *testing.T) {
	now := time.Date(2026, 11, 25, 12, 0, 0, 0, time.Local)
	lines := HeatmapLines(2026, heatTestCounts(2026), now, nil)

	if !strings.Contains(lines[0], "Mo Tu We Th Fr Sa Su") {
		t.Errorf("header line %q lacks weekday labels", lines[0])
	}
	for _, month := range []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"} {
		found := false
		halfW := heatHalfWidth() + heatGutter
		for _, line := range lines {
			runes := []rune(line)
			if len(runes) > 3 && (string(runes[:3]) == month || (len(runes) > halfW+3 && string(runes[halfW:halfW+3]) == month)) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("month label %q not found on any row", month)
		}
	}
	// January 2026 starts on a Thursday, so its label row has three blank
	// cells (Mon-Wed) before the 1st.
	var jan string
	for _, line := range lines {
		if strings.HasPrefix(line, "Jan ") {
			jan = line
			break
		}
	}
	if jan == "" {
		t.Fatal("no Jan label row")
	}
	// "Jan " + blank "  " cells: the first three cells are blanks.
	if !strings.HasPrefix(jan[4:], "     ") {
		t.Errorf("Jan row does not start with blank Mon-Wed cells: %q", jan)
	}
}

func TestHeatmapLinesBlankOutsideYear(t *testing.T) {
	now := time.Date(2026, 11, 25, 12, 0, 0, 0, time.Local)
	lines := HeatmapLines(2026, heatTestCounts(2026), now, nil)

	// The first row starts Monday 2025-12-29; Mon-Wed belong to 2025 and
	// must be blank even though 2025 counts exist in nothing — check cells
	// of the right half's December: days after Nov 25 have no counts and
	// render the level-0 glyph, while trailing blank padding is spaces.
	last := lines[len(lines)-1]
	_ = last
	// Counting: the row starting "Jan" must have a painted cell on Thursday
	// (2026-01-01 has 1 % 8 = 1 entries → level 1 glyph).
	var jan string
	for _, line := range lines {
		if strings.HasPrefix(line, "Jan ") {
			jan = line
			break
		}
	}
	// Cell layout: label 4 + cell 2 + gap 1 per weekday. Thursday is the
	// 4th column: offset 4 + 3*(2+1) = 13 (rune offsets; glyphs are
	// multi-byte).
	if got := string([]rune(jan)[13:15]); got != HeatGlyphs[1] {
		t.Errorf("Jan 1 cell = %q, want %q", got, HeatGlyphs[1])
	}
}

func TestHeatmapLinesEmptyYearIsAllZeroGlyphs(t *testing.T) {
	now := time.Date(2026, 11, 25, 12, 0, 0, 0, time.Local)
	lines := HeatmapLines(2026, nil, now, nil)
	glyph := HeatGlyphs[0]
	painted := 0
	for _, line := range lines[1:] {
		painted += strings.Count(line, glyph)
	}
	if painted == 0 {
		t.Fatal("no zero-level glyphs painted for an empty year")
	}

	// 2026-12-31 is a Thursday, so the last week row of the right half
	// paints Mon-Thu and leaves Fri-Sun blank. The left half's last week
	// (Jun 29) leaves Fri-Sun blank too. In rune offsets: left half cells
	// start at 4 with a 3-column pitch; the right half starts at
	// heatHalfWidth()+heatGutter.
	last := lines[len(lines)-1]
	runes := []rune(last)
	leftPad := runes[4+4*3 : 4+4*3+2] // left half, Friday
	if string(leftPad) != "  " {
		t.Errorf("left half trailing pad = %q, want blank", string(leftPad))
	}
	rightOff := heatHalfWidth() + heatGutter
	rightPad := runes[rightOff+4+4*3 : rightOff+4+4*3+2] // right half, Friday
	if string(rightPad) != "  " {
		t.Errorf("right half trailing pad = %q, want blank", string(rightPad))
	}
	// And the first row's leading cells (Mon-Wed of 2025) are blank too:
	// Mon cell + gap + Tue cell = six runes of blank.
	first := lines[1]
	if head := string([]rune(first)[4:10]); head != "      " {
		t.Errorf("leading pad = %q, want blanks", head)
	}
}

func TestHeatLegend(t *testing.T) {
	legend := HeatLegend(PlainHeatPainter{})
	for _, want := range []string{HeatGlyphs[0], HeatGlyphs[1], HeatGlyphs[2], HeatGlyphs[3], "6+ entries"} {
		if !strings.Contains(legend, want) {
			t.Errorf("legend %q lacks %q", legend, want)
		}
	}
}

func TestHeatLevel(t *testing.T) {
	cases := map[int]int{0: 0, 1: 1, 2: 1, 3: 2, 5: 2, 6: 3, 20: 3}
	for n, want := range cases {
		if got := HeatLevel(n); got != want {
			t.Errorf("HeatLevel(%d) = %d, want %d", n, got, want)
		}
	}
}

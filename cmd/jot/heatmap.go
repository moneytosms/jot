package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/moneytosms/jot"
)

// heatmap ANSI ramps. The dark ramp is tuned for dark terminals; the light
// ramp for JOT_THEME=light. Level 0 uses the plain glyph without color so
// empty days stay quiet.
var (
	heatDarkShades  = [3]string{"#3f5a3f", "#5b8852", "#9ece6a"}
	heatLightShades = [3]string{"#c3d3b3", "#8fb07f", "#2f5f38"}
)

// heatPainter implements jot.HeatPainter with truecolor cells. Color is off
// for pipes and NO_COLOR, where it degrades to the plain glyph ramp.
type heatPainter struct {
	color  bool
	shades [3]string
}

// Cell renders one two-character cell. Levels 1-3 are painted with their
// shade; level 0 stays uncolored so the empty-day dots recede.
func (p heatPainter) Cell(level int) string {
	glyph := jot.HeatGlyphs[level]
	if level <= 0 || level > len(p.shades) || !p.color {
		return glyph
	}
	return "\x1b[38;2;" + p.shades[level-1] + "m" + glyph + "\x1b[0m"
}

// heatmapCmd prints a year heatmap of one journal: two half-year grids,
// weeks as rows, one cell per day shaded by entry count.
func (a *app) heatmapCmd(args []string) error {
	fs := a.flags("heatmap")
	journal := journalFlag(fs)
	year := fs.Int("year", time.Now().Year(), "year to chart")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("heatmap takes no positional arguments; use --year")
	}
	if *year < 1 || *year > 9999 {
		return fmt.Errorf("invalid year %d", *year)
	}

	j, err := a.loadJot()
	if err != nil {
		return err
	}
	name := pickJournal(j, *journal)
	days, err := j.YearDays(name, *year)
	if errors.Is(err, jot.ErrNoJournal) {
		return noJournalErr(name)
	}
	if err != nil {
		return err
	}

	counts := make(map[string]int, len(days))
	dayFiles, entries := 0, 0
	for _, d := range days {
		counts[d.Day] = d.Entries
		if d.Entries > 0 {
			dayFiles++
		}
		entries += d.Entries
	}

	caption := fmt.Sprintf("%d · %s · %dd %de", *year, name, dayFiles, entries)
	if styles, _ := themeStyles(a.out); styles.Dim != "" {
		caption = styles.Dim + caption + "\x1b[0m"
	}
	fmt.Fprintln(a.out, caption)

	painter := heatPainterFor(a.out)
	for _, line := range jot.HeatmapLines(*year, counts, time.Now(), painter) {
		fmt.Fprintln(a.out, line)
	}
	fmt.Fprintln(a.out, jot.HeatLegend(painter))
	return nil
}

// heatPainterFor picks the painter for out: plain for pipes and NO_COLOR,
// truecolor otherwise, with the light ramp when JOT_THEME asks for light.
func heatPainterFor(out io.Writer) heatPainter {
	if os.Getenv("NO_COLOR") != "" || !isTerminalWriter(out) {
		return heatPainter{}
	}
	if strings.Contains(strings.ToLower(os.Getenv("JOT_THEME")), "light") {
		return heatPainter{color: true, shades: heatLightShades}
	}
	return heatPainter{color: true, shades: heatDarkShades}
}

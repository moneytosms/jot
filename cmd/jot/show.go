package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/moneytosms/jot"
)

// renderWidth is the column budget for --render output, the day bar and
// search previews.
const renderWidth = 80

// calendarCurrent is the value a bare --calendar resolves to: the current
// month. The flag package cannot express an optional value, so a valueless
// --calendar is rewritten into this form before parsing.
const calendarCurrent = "current"

func (a *app) showCmd(args []string) error {
	fs := a.flags("show")
	journal := journalFlag(fs)
	date := dateFlag(fs)
	all := fs.Bool("all", false, "show every day")
	fs.BoolVar(all, "a", false, "shorthand for -all")
	render := fs.Bool("render", false, "render markdown for the terminal")
	fs.String("calendar", "", "print a month grid for YYYY-MM (default: current month)")

	args = showCalendarArg(args)
	if err := fs.Parse(args); err != nil {
		return err
	}
	calendar, calendarSet := "", false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "calendar" {
			calendar, calendarSet = f.Value.String(), true
		}
	})
	if fs.NArg() > 0 && *date == "" {
		*date = fs.Arg(0)
	}

	j, err := a.loadJot()
	if err != nil {
		return err
	}
	name := pickJournal(j, *journal)

	if calendarSet {
		return a.showCalendar(j, name, calendar)
	}

	styles, _ := themeStyles(a.out)

	write := func(day, content string, banner bool) {
		content = strings.TrimRight(content, "\n")
		if *render {
			content = jot.RenderStyled(content, renderWidth, styles)
		}
		if banner {
			fmt.Fprintf(a.out, "# %s · %s\n\n", name, day)
		}
		if content != "" {
			fmt.Fprintln(a.out, content)
			fmt.Fprintln(a.out)
		}
	}

	if *all || strings.EqualFold(strings.TrimSpace(*date), "all") {
		files, err := j.ShowDays(name)
		if err != nil {
			if errors.Is(err, jot.ErrNoJournal) {
				return noJournalErr(name)
			}
			return err
		}
		if len(files) == 0 {
			fmt.Fprintf(a.out, "no entries in %s yet\n", name)
			return nil
		}
		for _, file := range files {
			write(file.Day, file.Content, true)
		}
		return nil
	}

	day, err := jot.ParseDay(*date)
	if err != nil {
		return err
	}
	contents, err := j.Show(name, day)
	if err != nil {
		if errors.Is(err, jot.ErrNoJournal) {
			return noJournalErr(name)
		}
		return err
	}
	if len(contents) == 0 {
		fmt.Fprintf(a.out, "no entries in %s for %s\n", name, day)
		return nil
	}

	count := 0
	for _, content := range contents {
		count += len(jot.ParseEntries(content))
	}
	bar := dayHeader(name, day, count, renderWidth)
	if styles.Dim != "" {
		bar = styles.Dim + bar + "\x1b[0m"
	}
	fmt.Fprintf(a.out, "%s\n\n", bar)
	for _, content := range contents {
		write(day, content, false)
	}
	return nil
}

// showCalendar prints the month grid for one journal. month is "YYYY-MM" or
// the sentinel a bare --calendar leaves behind.
func (a *app) showCalendar(j *jot.Jot, name, month string) error {
	year, mon, err := parseCalendarMonth(month, time.Now())
	if err != nil {
		return err
	}
	counts, err := j.MonthDays(name, year, mon)
	if errors.Is(err, jot.ErrNoJournal) {
		return noJournalErr(name)
	}
	if err != nil {
		return err
	}
	for _, line := range monthGrid(year, mon, counts, time.Now()) {
		fmt.Fprintln(a.out, line)
	}
	return nil
}

// parseCalendarMonth parses "YYYY-MM" (or the bare-calendar sentinel) into a
// year and month, defaulting to now.
func parseCalendarMonth(s string, now time.Time) (int, time.Month, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == calendarCurrent {
		return now.Year(), now.Month(), nil
	}
	if t, err := time.ParseInLocation("2006-01", s, time.Local); err == nil {
		return t.Year(), t.Month(), nil
	}
	return 0, 0, fmt.Errorf("invalid month %q: use YYYY-MM", s)
}

// showCalendarArg rewrites a bare --calendar (last argument, or followed by
// another flag) into --calendar=current, because the flag package cannot
// express an optional value.
func showCalendarArg(args []string) []string {
	out := make([]string, 0, len(args))
	for i, arg := range args {
		if (arg == "--calendar" || arg == "-calendar") &&
			(i+1 == len(args) || strings.HasPrefix(args[i+1], "-")) {
			out = append(out, "--calendar="+calendarCurrent)
			continue
		}
		out = append(out, arg)
	}
	return out
}

// dayHeader builds the bar printed above a day's entries: journal, date,
// weekday and entry count, padded with dashes out to width.
func dayHeader(journal, day string, count, width int) string {
	weekday := ""
	if t, err := time.Parse("2006-01-02", day); err == nil {
		weekday = t.Format("Mon")
	}
	label := fmt.Sprintf("-- %s . %s (%s) . %d %s ", journal, day, weekday, count, plural(count, "entry", "entries"))
	if len(label) >= width {
		return strings.TrimRight(label, " ")
	}
	return label + strings.Repeat("-", width-len(label))
}

// monthGrid renders one month of a journal as plain text lines: a title, a
// Monday-first weekday header, one line per week with a density marker per
// day (a dot for 1-2 entries, a star for 3+), today in brackets when it falls
// in the month, and a legend with the month total. It is pure, so the grid is
// deterministic and easy to test.
func monthGrid(year int, month time.Month, counts []jot.DayCount, now time.Time) []string {
	byDay := make(map[string]int, len(counts))
	total := 0
	for _, c := range counts {
		byDay[c.Day] += c.Entries
		total += c.Entries
	}
	first := time.Date(year, month, 1, 0, 0, 0, 0, time.Local)
	offset := (int(first.Weekday()) + 6) % 7 // Monday-first column index
	days := time.Date(year, month+1, 0, 0, 0, 0, 0, time.Local).Day()
	inMonth := now.Year() == year && now.Month() == month

	var lines []string
	lines = append(lines, fmt.Sprintf("%s %d", month, year), "Mo Tu We Th Fr Sa Su")
	var row strings.Builder
	for i := 0; i < offset; i++ {
		row.WriteString("     ")
	}
	for d := 1; d <= days; d++ {
		day := fmt.Sprintf("%04d-%02d-%02d", year, int(month), d)
		marker := byte(' ')
		switch n := byDay[day]; {
		case n >= 3:
			marker = '*'
		case n >= 1:
			marker = '.'
		}
		if inMonth && now.Day() == d {
			fmt.Fprintf(&row, "[%2d]%c", d, marker)
		} else {
			fmt.Fprintf(&row, "%3d%c ", d, marker)
		}
		if (offset+d)%7 == 0 || d == days {
			lines = append(lines, strings.TrimRight(row.String(), " "))
			row.Reset()
		}
	}
	legend := ". 1-2 entries, * 3+ entries"
	if inMonth {
		legend += ", [..] today"
	}
	lines = append(lines, "", "legend: "+legend)
	lines = append(lines, fmt.Sprintf("%d %s in %s %d", total, plural(total, "entry", "entries"), month, year))
	return lines
}

// themeStyles chooses the markdown style set for --render. NO_COLOR or
// non-terminal output gets plain text; otherwise dark styles, unless
// JOT_THEME names a light theme.
func themeStyles(out io.Writer) (jot.MDStyles, bool) {
	if os.Getenv("NO_COLOR") != "" || !isTerminalWriter(out) {
		return jot.PlainStyles(), false
	}
	if strings.Contains(strings.ToLower(os.Getenv("JOT_THEME")), "light") {
		return jot.DefaultLightStyles(), true
	}
	return jot.DefaultDarkStyles(), true
}

// isTerminalWriter reports whether w is an interactive terminal.
func isTerminalWriter(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	st, err := f.Stat()
	return err == nil && st.Mode()&os.ModeCharDevice != 0
}

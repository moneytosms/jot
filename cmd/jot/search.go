package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/moneytosms/jot"
)

func (a *app) searchCmd(args []string) error {
	fs := a.flags("search")
	journal := journalFlag(fs)
	fuzzy := fs.Bool("fuzzy", false, "fuzzy entry-level search with previews")
	limit := fs.Int("limit", 20, "maximum results in fuzzy mode (0 = unlimited)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	// Go's flag package stops at the first non-flag argument, so flags written
	// after the query would silently become part of the query.
	for _, arg := range fs.Args() {
		if strings.HasPrefix(arg, "-") && arg != "-" {
			return fmt.Errorf("flags must come before the query, for example: jot search --fuzzy --limit 10 parser")
		}
	}
	query := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if query == "" {
		return errors.New("usage: jot search <query>")
	}

	j, err := a.loadJot()
	if err != nil {
		return err
	}

	// Only scope to one journal when -j was actually given; otherwise search
	// everything, which is what people expect from a journal search.
	scoped := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "journal" || f.Name == "j" {
			scoped = true
		}
	})

	if *fuzzy || jot.ParseQuery(query).Filtered() { // filters work on entries
		return a.searchFuzzy(j, scoped, *journal, query, *limit)
	}

	var matches []jot.Match
	if scoped {
		name := pickJournal(j, *journal)
		matches, err = j.SearchJournal(name, query)
		if err != nil && errors.Is(err, jot.ErrNoJournal) {
			return noJournalErr(name)
		}
	} else {
		matches, err = j.Search(query)
	}
	if err != nil {
		return err
	}
	if len(matches) == 0 {
		fmt.Fprintf(a.out, "no matches for %q\n", query)
		return nil
	}
	printSearchHits(a.out, matches)
	return nil
}

// printSearchHits prints substring-search hits grouped by journal and day: a
// header line per day with the hit count, every hit with its line number, and
// a final summary.
func printSearchHits(w io.Writer, matches []jot.Match) {
	type group struct {
		journal, day string
		matches      []jot.Match
	}
	var groups []group
	for _, m := range matches {
		if n := len(groups); n > 0 && groups[n-1].journal == m.Journal && groups[n-1].day == m.Day {
			groups[n-1].matches = append(groups[n-1].matches, m)
			continue
		}
		groups = append(groups, group{m.Journal, m.Day, []jot.Match{m}})
	}
	journals := 0
	prev := ""
	for _, g := range groups {
		if g.journal != prev {
			journals++
			prev = g.journal
		}
	}
	for _, g := range groups {
		fmt.Fprintf(w, "%s . %s . %d %s\n", g.journal, g.day, len(g.matches), plural(len(g.matches), "hit", "hits"))
		for _, m := range g.matches {
			fmt.Fprintf(w, "  %s/%s.md:%d: %s\n", m.Journal, m.Day, m.Line, m.Text)
		}
	}
	fmt.Fprintf(w, "\n%d %s in %d %s\n",
		len(matches), plural(len(matches), "match", "matches"),
		journals, plural(journals, "journal", "journals"))
}

// searchFuzzy runs the entry-level fuzzy search and prints one block per hit:
// a "journal . day . time" header, the first three body lines truncated to
// the render width, and a blank line. Hits keep the library's score order;
// only the count is capped at limit (0 means no limit).
func (a *app) searchFuzzy(j *jot.Jot, scoped bool, journal, query string, limit int) error {
	var hits []jot.EntryMatch
	var err error
	if scoped {
		name := pickJournal(j, journal)
		hits, err = j.SearchEntriesJournal(name, query)
		if errors.Is(err, jot.ErrNoJournal) {
			return noJournalErr(name)
		}
	} else {
		hits, err = j.SearchEntries(query)
	}
	if err != nil {
		return err
	}
	if len(hits) == 0 {
		fmt.Fprintf(a.out, "no matches for %q\n", query)
		return nil
	}
	shown := hits
	if limit > 0 && len(hits) > limit {
		shown = hits[:limit]
	}
	for _, h := range shown {
		at := h.Time
		if at == "" {
			at = "-"
		}
		fmt.Fprintf(a.out, "%s . %s . %s\n", h.Journal, h.Day, at)
		for _, line := range previewLines(h.Body, 3, renderWidth) {
			fmt.Fprintln(a.out, line)
		}
		fmt.Fprintln(a.out)
	}
	if len(shown) < len(hits) {
		fmt.Fprintf(a.out, "%d %s, showing first %d\n",
			len(hits), plural(len(hits), "match", "matches"), len(shown))
		return nil
	}
	fmt.Fprintf(a.out, "%d %s\n", len(hits), plural(len(hits), "match", "matches"))
	return nil
}

// previewLines returns up to n lines of body, each truncated to width columns.
func previewLines(body string, n, width int) []string {
	lines := strings.Split(body, "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		out = append(out, truncateCols(line, width))
	}
	return out
}

// truncateCols cuts s to width runes, appending "..." when anything was cut.
func truncateCols(s string, width int) string {
	if width <= 0 || utf8.RuneCountInString(s) <= width {
		return s
	}
	r := []rune(s)
	return string(r[:width]) + "..."
}

func (a *app) backupCmd(args []string) error {
	fs := a.flags("backup")
	dir := fs.String("out", "", "destination directory")
	fs.StringVar(dir, "o", "", "shorthand for -out")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		*dir = fs.Arg(0)
	}

	j, err := a.loadJot()
	if err != nil {
		return err
	}
	path, err := j.Backup(*dir)
	if err != nil {
		return err
	}
	info := ""
	if st, err := os.Stat(path); err == nil {
		info = humanBytes(st.Size())
	}
	if info != "" {
		fmt.Fprintf(a.out, "backup written to %s (%s)\n", path, info)
	} else {
		fmt.Fprintf(a.out, "backup written to %s\n", path)
	}
	return nil
}

func (a *app) exportCmd(args []string) error {
	fs := a.flags("export")
	dir := fs.String("out", "", "destination directory")
	fs.StringVar(dir, "o", "", "shorthand for -out")
	format := fs.String("format", "md", "md, html or pdf")
	journal := journalFlag(fs)
	from := fs.String("from", "", "first day (YYYY-MM-DD, today, yesterday)")
	to := fs.String("to", "", "last day")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		*dir = fs.Arg(0)
	}

	j, err := a.loadJot()
	if err != nil {
		return err
	}
	if *format == "html" || *format == "pdf" {
		opt := jot.ExportOptions{Format: *format, Dir: *dir}
		for _, d := range []struct{ in, out *string }{{from, &opt.From}, {to, &opt.To}} {
			if *d.in != "" {
				if *d.out, err = jot.ParseDay(*d.in); err != nil {
					return err
				}
			}
		}
		names := []string{*journal}
		if *journal == "" {
			infos, err := j.List()
			if err != nil {
				return err
			}
			names = names[:0]
			for _, info := range infos {
				if info.Entries > 0 {
					names = append(names, info.Name)
				}
			}
		}
		if len(names) == 0 {
			fmt.Fprintf(a.out, "nothing to export yet\n")
		}
		for _, n := range names {
			path, err := j.ExportDocument(n, opt)
			if err != nil {
				return err
			}
			fmt.Fprintf(a.out, "wrote %s\n", path)
		}
		return nil
	}
	if *format != "md" {
		return fmt.Errorf("unknown --format %q; use md, html or pdf", *format)
	}
	written, err := j.Export(*dir)
	if err != nil {
		return err
	}
	if len(written) == 0 {
		fmt.Fprintf(a.out, "nothing to export yet\n")
		return nil
	}
	for _, path := range written {
		fmt.Fprintf(a.out, "wrote %s\n", path)
	}
	return nil
}

// tagsCmd lists #tags with the number of entries using each.
func (a *app) tagsCmd(args []string) error {
	fs := a.flags("tags")
	journal := journalFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	j, err := a.loadJot()
	if err != nil {
		return err
	}
	counts, err := j.TagCounts(*journal)
	if errors.Is(err, jot.ErrNoJournal) {
		return noJournalErr(*journal)
	}
	if err != nil {
		return err
	}
	if len(counts) == 0 {
		fmt.Fprintln(a.out, "no #tags yet; write one in any entry, e.g. jot add \"shipped it #work\"")
		return nil
	}
	for _, c := range counts {
		fmt.Fprintf(a.out, "%5d  #%s\n", c.Count, c.Tag)
	}
	return nil
}

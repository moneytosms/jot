package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/moneytosms/jot"
)

func (a *app) showCmd(args []string) error {
	fs := a.flags("show")
	journal := journalFlag(fs)
	date := dateFlag(fs)
	all := fs.Bool("all", false, "show every day")
	fs.BoolVar(all, "a", false, "shorthand for -all")
	render := fs.Bool("render", false, "render markdown for the terminal")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 && *date == "" {
		*date = fs.Arg(0)
	}

	j, err := a.loadJot()
	if err != nil {
		return err
	}
	name := pickJournal(j, *journal)

	write := func(day, content string, banner bool) {
		content = strings.TrimRight(content, "\n")
		if *render {
			content = jot.Render(content)
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
	for _, content := range contents {
		write(day, content, false)
	}
	return nil
}

package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/moneytosms/jot"
)

func (a *app) searchCmd(args []string) error {
	fs := a.flags("search")
	journal := journalFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
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
	for _, m := range matches {
		fmt.Fprintf(a.out, "%s/%s.md:%d: %s\n", m.Journal, m.Day, m.Line, m.Text)
	}
	return nil
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

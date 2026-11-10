package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/moneytosms/jot"
)

func (a *app) editCmd(args []string) error {
	fs := a.flags("edit")
	journal := journalFlag(fs)
	date := dateFlag(fs)
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
	if err := j.Edit(name, *date, a.in, a.out, a.errw); err != nil {
		if errors.Is(err, jot.ErrNoJournal) {
			return noJournalErr(name)
		}
		return err
	}
	return nil
}

func (a *app) initCmd(args []string) error {
	fs := a.flags("init")
	journal := journalFlag(fs)
	root := fs.String("root", "", "root directory to create")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := jot.DefaultConfig()
	if err != nil {
		return err
	}
	if strings.TrimSpace(*root) != "" {
		cfg.Root = *root
	}
	j := jot.New(cfg)
	if err := j.Init(); err != nil {
		return err
	}
	if strings.TrimSpace(*journal) != "" {
		if _, err := j.EnsureJournal(*journal); err != nil {
			return err
		}
	}
	fmt.Fprintf(a.out, "initialized %s\n", j.Root())
	fmt.Fprintf(a.out, "default journal: %s\n", j.DefaultJournal())
	return nil
}

func (a *app) lsCmd(args []string) error {
	fs := a.flags("ls")
	journal := journalFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	name := *journal
	if strings.TrimSpace(name) == "" && fs.NArg() > 0 {
		name = fs.Arg(0)
	}

	j, err := a.loadJot()
	if err != nil {
		return err
	}
	if strings.TrimSpace(name) != "" {
		return a.listDays(j, jot.SanitizeName(name))
	}

	infos, err := j.List()
	if err != nil {
		return err
	}
	if len(infos) == 0 {
		fmt.Fprintf(a.out, "no journals yet; add an entry with `jot add \"...\"`\n")
		return nil
	}
	tw := newTable(a.out)
	fmt.Fprintln(tw, "JOURNAL\tENTRIES\tDAYS\tLAST")
	for _, info := range infos {
		last := "-"
		if !info.Last.IsZero() {
			last = info.Last.Format("2006-01-02 15:04")
		}
		fmt.Fprintf(tw, "%s\t%d\t%d\t%s\n", info.Name, info.Entries, info.Days, last)
	}
	return tw.Flush()
}

func (a *app) listDays(j *jot.Jot, name string) error {
	days, err := j.DaySummaries(name)
	if err != nil {
		if errors.Is(err, jot.ErrNoJournal) {
			return noJournalErr(name)
		}
		return err
	}
	if len(days) == 0 {
		fmt.Fprintf(a.out, "no entries in %s yet\n", name)
		return nil
	}
	tw := newTable(a.out)
	fmt.Fprintln(tw, "DAY\tENTRIES\tSIZE")
	for _, day := range days {
		fmt.Fprintf(tw, "%s\t%d\t%s\n", day.Day, day.Entries, humanBytes(day.Bytes))
	}
	return tw.Flush()
}

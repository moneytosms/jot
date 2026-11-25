package main

import (
	"errors"
	"flag"
	"fmt"

	"github.com/moneytosms/jot"
)

// todoCmd lists markdown checkbox items ("- [ ]" / "- [x]") found in entry
// bodies, newest day first. It is strictly read-only: ticking a box is done
// by editing the day file, never by this command.
func (a *app) todoCmd(args []string) error {
	fs := a.flags("todo")
	journal := journalFlag(fs)
	onlyOpen := fs.Bool("open", false, "list unchecked items only")
	onlyDone := fs.Bool("done", false, "list completed items only")
	fs.Bool("all", false, "list everything (the default)")
	limit := fs.Int("limit", 0, "list at most N items (0 = no limit)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	j, err := a.loadJot()
	if err != nil {
		return err
	}

	// Like search, only scope to one journal when -j was actually given.
	scoped := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "journal" || f.Name == "j" {
			scoped = true
		}
	})

	var items []jot.TodoItem
	if scoped {
		name := pickJournal(j, *journal)
		items, err = j.TodosJournal(name)
		if errors.Is(err, jot.ErrNoJournal) {
			return noJournalErr(name)
		}
	} else {
		items, err = j.Todos()
	}
	if err != nil {
		return err
	}

	openCount, doneCount := 0, 0
	for _, it := range items {
		if it.Done {
			doneCount++
		} else {
			openCount++
		}
	}

	shown := items
	switch {
	case *onlyOpen && !*onlyDone:
		shown = filterTodos(items, false)
	case *onlyDone && !*onlyOpen:
		shown = filterTodos(items, true)
	}
	if *limit > 0 && len(shown) > *limit {
		shown = shown[:*limit]
	}

	if len(shown) == 0 {
		fmt.Fprintln(a.out, "no todo items")
	}
	tw := newTable(a.out)
	for _, it := range shown {
		box := "[ ]"
		if it.Done {
			box = "[x]"
		}
		at := it.Time
		if at == "" {
			at = "-"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", box, it.Journal, it.Day, at, it.Text)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	if len(shown) == 0 {
		fmt.Fprintln(a.out, "toggle with jot edit -d DAY -j JOURNAL")
	} else {
		fmt.Fprintf(a.out, "toggle with jot edit -d %s -j %s\n", shown[0].Day, shown[0].Journal)
	}
	fmt.Fprintf(a.out, "%d open . %d done\n", openCount, doneCount)
	return nil
}

// filterTodos keeps only the items whose ticked state matches done.
func filterTodos(items []jot.TodoItem, done bool) []jot.TodoItem {
	out := make([]jot.TodoItem, 0, len(items))
	for _, it := range items {
		if it.Done == done {
			out = append(out, it)
		}
	}
	return out
}

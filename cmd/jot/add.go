package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/moneytosms/jot"
)

func (a *app) addCmd(args []string) error {
	fs := a.flags("add")
	journal := journalFlag(fs)
	date := dateFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	j, err := a.loadJot()
	if err != nil {
		return err
	}

	day, err := jot.ParseDay(*date)
	if err != nil {
		return err
	}

	body := strings.TrimSpace(strings.Join(fs.Args(), " "))
	fromStdin := body == "-"
	if body == "" {
		if !stdinPiped(a.in) {
			return errors.New("nothing to add: pass text, pipe it in, or use `jot edit`\n\n  jot add \"shipped the release\"\n  echo \"a note\" | jot add")
		}
		fromStdin = true
	}
	if fromStdin {
		body, err = readBody(a.in)
		if err != nil {
			return err
		}
	}
	if strings.TrimSpace(body) == "" {
		return jot.ErrEmptyBody
	}

	name := pickJournal(j, *journal)
	if err := j.Add(jot.Entry{Name: name, Day: day, Body: body}); err != nil {
		return err
	}
	fmt.Fprintf(a.out, "added to %s/%s.md\n", name, day)
	return nil
}

package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/moneytosms/jot"
)

// parseReminderDuration parses a reminder delay. Go-style durations (90s,
// 20m, 2h, 1h30m) are accepted, as is a bare number meaning minutes. It is
// pure so it can be unit tested without touching systemd.
func parseReminderDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if d, err := time.ParseDuration(s); err == nil {
		if d <= 0 {
			return 0, fmt.Errorf("duration must be positive, got %s", d)
		}
		return d, nil
	}
	if n, err := strconv.Atoi(s); err == nil && n > 0 {
		return time.Duration(n) * time.Minute, nil
	}
	return 0, fmt.Errorf("invalid duration %q: use 90s, 20m, 2h or a number of minutes", s)
}

// remindCmd sets a countdown reminder or manages existing ones:
//
//	jot remind MESSAGE DURATION   (the duration may also come first)
//	jot remind list               print active reminders
//	jot remind clear              stop and remove every jot reminder
func (a *app) remindCmd(args []string) error {
	switch {
	case len(args) > 0 && args[0] == "list":
		return a.remindList()
	case len(args) > 0 && args[0] == "clear":
		return a.remindClear()
	}

	fs := a.flags("remind")
	if err := fs.Parse(args); err != nil {
		return err
	}
	rest := fs.Args()
	if len(rest) < 2 {
		return errors.New("usage: jot remind MESSAGE DURATION (e.g. jot remind \"stretch\" 20m)")
	}

	var message, durationArg string
	if _, err := parseReminderDuration(rest[0]); err == nil {
		durationArg = rest[0]
		message = strings.Join(rest[1:], " ")
	} else if _, err := parseReminderDuration(rest[len(rest)-1]); err == nil {
		message = strings.Join(rest[:len(rest)-1], " ")
		durationArg = rest[len(rest)-1]
	} else {
		return fmt.Errorf("cannot parse a duration out of %q or %q: use 90s, 20m, 2h or a number of minutes", rest[0], rest[len(rest)-1])
	}
	message = strings.TrimSpace(message)
	if message == "" {
		return errors.New("reminder message is empty")
	}
	delay, err := parseReminderDuration(durationArg)
	if err != nil {
		return err
	}

	j, err := a.loadJot()
	if err != nil {
		return err
	}
	unit, err := j.Remind(message, delay)
	if err != nil {
		return a.remindError(err)
	}
	fmt.Fprintf(a.out, "reminder set: %s\n  fires in %s via %s\n", message, delay, unit)
	return nil
}

// remindList prints the active reminders with their time remaining, or the
// fire time when that is all that is known.
func (a *app) remindList() error {
	j, err := a.loadJot()
	if err != nil {
		return err
	}
	rems, err := j.Reminders()
	if err != nil {
		return a.remindError(err)
	}
	if len(rems) == 0 {
		fmt.Fprintln(a.out, "no reminders")
		return nil
	}
	now := time.Now()
	for _, r := range rems {
		when := "at an unknown time"
		if !r.At.IsZero() {
			if d := r.At.Sub(now); d > 0 {
				when = "in " + d.Round(time.Second).String()
			} else {
				when = r.At.Format("15:04:05")
			}
		}
		message := r.Message
		if message == "" {
			message = "(message unavailable)"
		}
		fmt.Fprintf(a.out, "%s  %s\n", when, message)
	}
	return nil
}

// remindClear stops and removes every active jot reminder timer.
func (a *app) remindClear() error {
	j, err := a.loadJot()
	if err != nil {
		return err
	}
	cleared, err := j.ClearReminders()
	if err != nil {
		return a.remindError(err)
	}
	if cleared == 0 {
		fmt.Fprintln(a.out, "no reminders to clear")
		return nil
	}
	fmt.Fprintf(a.out, "cleared %d %s\n", cleared, plural(cleared, "reminder", "reminders"))
	return nil
}

// remindError turns the library's unsupported-platform error into a clear
// hint about what is missing, instead of a bare failure.
func (a *app) remindError(err error) error {
	if errors.Is(err, jot.ErrRemindersUnsupported) {
		return fmt.Errorf("%w\n  install one of them, or set \"reminders\" in the config file (run jot doctor to see what is found)", err)
	}
	return err
}

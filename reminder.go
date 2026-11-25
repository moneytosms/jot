package jot

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// ErrRemindersUnsupported is returned when no reminder provider is available:
// transient reminders need either omarchy or a systemd user session with
// notify-send.
var ErrRemindersUnsupported = errors.New("reminders need systemd (user) with notify-send, or omarchy")

// reminderUnitPrefix is the unit-name prefix that marks a jot reminder timer.
// systemctl list-timers glob patterns and the stash file names are derived
// from it.
const reminderUnitPrefix = "jot-reminder-"

// Reminder is an active countdown reminder.
type Reminder struct {
	// Unit is the systemd unit name without the .timer/.service suffix.
	// Omarchy-passthrough reminders have no unit and never appear here.
	Unit string
	// Message is the text shown when the reminder fires. Read from the
	// runtime stash file; empty when the file is gone.
	Message string
	// At is the next elapse time. Zero when it cannot be determined
	// reliably (for example a monotonic-only elapse without a boot time).
	At time.Time
}

// Remind sets a transient countdown reminder that fires the given message
// after in has passed, and returns the unit name or a passthrough label.
//
// Providers, first available wins:
//
//   - omarchy: run "omarchy reminder <minutes> <message>". Omarchy is
//     minute-granular, so durations shorter than one minute are rejected.
//     The returned label is "omarchy-reminder-<minutes>m"; omarchy owns the
//     real timer, so nothing is created under systemd.
//   - systemd: create a transient user timer "jot-reminder-<unix-nano>" via
//     systemd-run; when it fires, the stash file is shown through
//     notify-send and removed.
//
// The message is validated for every provider: empty text and non-positive
// durations are rejected. User text is never interpolated into a shell
// command line; it travels in the stash file only.
func (j *Jot) Remind(message string, in time.Duration) (string, error) {
	if strings.TrimSpace(message) == "" {
		return "", errors.New("reminder message is empty")
	}
	if in <= 0 {
		return "", fmt.Errorf("reminder duration must be positive, got %s", in)
	}
	switch {
	case lookPath("omarchy"):
		minutes, err := omarchyMinutes(in)
		if err != nil {
			return "", err
		}
		_, stderr, err := runCapture(exec.Command("omarchy", "reminder", strconv.Itoa(minutes), message))
		if err != nil {
			return "", fmt.Errorf("omarchy reminder: %s: %w", strings.TrimSpace(stderr), err)
		}
		return fmt.Sprintf("omarchy-reminder-%dm", minutes), nil
	case lookPath("systemd-run") && lookPath("notify-send"):
		return j.remindSystemd(message, in)
	default:
		return "", ErrRemindersUnsupported
	}
}

// omarchyMinutes converts a duration into whole minutes, rounding up, for
// omarchy's minute-granular reminder command. Sub-minute durations are
// rejected because rounding them to one minute would fire early.
func omarchyMinutes(in time.Duration) (int, error) {
	if in <= 0 {
		return 0, fmt.Errorf("reminder duration must be positive, got %s", in)
	}
	if in < time.Minute {
		return 0, fmt.Errorf("omarchy reminders are minute-granular; use at least %s, got %s", time.Minute, in)
	}
	return int((in + time.Minute - 1) / time.Minute), nil
}

// remindSystemd creates a transient systemd user timer that fires once,
// shows the message through notify-send and then removes the stash file.
func (j *Jot) remindSystemd(message string, in time.Duration) (string, error) {
	unit := newReminderUnit(time.Now())
	secs := int((in + time.Second - 1) / time.Second)
	if secs < 1 {
		secs = 1
	}
	dir := reminderStashDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	msgFile, err := writeStash(dir, unit, message)
	if err != nil {
		return "", err
	}
	// The fired command re-reads the message from the stash file, so the
	// only text interpolated here is a path jot built itself.
	notify := fmt.Sprintf(`notify-send -a jot "jot" "$(cat %s)"; rm -f %s`, shQuote(msgFile), shQuote(msgFile))
	_, stderr, err := runCapture(exec.Command("systemd-run",
		"--user", "--collect",
		"--on-active="+strconv.Itoa(secs)+"s",
		"--unit="+unit+".timer",
		"sh", "-c", notify))
	if err != nil {
		os.Remove(msgFile)
		return "", fmt.Errorf("systemd-run: %s: %w", strings.TrimSpace(stderr), err)
	}
	return unit, nil
}

// newReminderUnit mints a unique transient unit name for a reminder set at
// the given instant.
func newReminderUnit(at time.Time) string {
	return fmt.Sprintf("%s%d", reminderUnitPrefix, at.UnixNano())
}

// reminderStashDir is where reminder messages wait for their timer to fire.
// It lives under XDG_RUNTIME_DIR (a tmpfs cleared at logout) and falls back
// to a per-user directory in the system temp dir.
func reminderStashDir() string {
	if dir := strings.TrimSpace(os.Getenv("XDG_RUNTIME_DIR")); dir != "" {
		return filepath.Join(dir, "jot-reminders")
	}
	return filepath.Join(os.TempDir(), "jot-reminders-"+strconv.Itoa(os.Getuid()))
}

// stashPath is the message file for a unit.
func stashPath(dir, unit string) string {
	return filepath.Join(dir, unit+".message")
}

// writeStash stores a reminder message for unit under dir and returns the
// file path. The file is user-private so other local users cannot read the
// text.
func writeStash(dir, unit, message string) (string, error) {
	path := stashPath(dir, unit)
	if err := os.WriteFile(path, []byte(message), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

// readStash reads a stashed reminder message. A missing file is an empty
// message, not an error: the reminder may have fired between listing and
// reading.
func readStash(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

// Reminders lists active jot reminders. Only transient timers named
// jot-reminder-*.timer are enumerated; omarchy-passthrough reminders are
// deliberately left out, because omarchy's own reminder UI owns those units.
func (j *Jot) Reminders() ([]Reminder, error) {
	if !systemctlAvailable() {
		return nil, ErrRemindersUnsupported
	}
	out, stderr, err := runCapture(exec.Command("systemctl", "--user",
		"list-timers", "--all", "--output=json", reminderUnitPrefix+"*.timer"))
	if err != nil {
		return nil, fmt.Errorf("systemctl list-timers: %s: %w", strings.TrimSpace(stderr), err)
	}
	boot, _ := bootTime()
	return parseTimerList([]byte(out), boot, reminderStashDir())
}

// ClearReminders stops and removes every active jot reminder timer, returning
// how many were cleared. Stopping a unit that already went away counts as
// success. Only jot's own timers are touched; omarchy-passthrough reminders
// are managed by omarchy.
func (j *Jot) ClearReminders() (int, error) {
	if !systemctlAvailable() {
		return 0, ErrRemindersUnsupported
	}
	out, stderr, err := runCapture(exec.Command("systemctl", "--user",
		"list-timers", "--all", "--output=json", reminderUnitPrefix+"*.timer"))
	if err != nil {
		return 0, fmt.Errorf("systemctl list-timers: %s: %w", strings.TrimSpace(stderr), err)
	}
	boot, _ := bootTime()
	reminders, err := parseTimerList([]byte(out), boot, "")
	if err != nil {
		return 0, err
	}
	cleared := 0
	var failed []string
	for _, r := range reminders {
		for _, suffix := range []string{".timer", ".service"} {
			if err := stopUnit(r.Unit + suffix); err != nil {
				failed = append(failed, r.Unit+suffix)
			}
		}
		cleared++
	}
	if len(failed) > 0 {
		// reset-failed clears the failed state so the names can be reused.
		_, stderr, err := runCapture(exec.Command("systemctl", "--user", "reset-failed"))
		if err != nil {
			return cleared, fmt.Errorf("systemctl reset-failed: %s: %w", strings.TrimSpace(stderr), err)
		}
	}
	return cleared, nil
}

// stopUnit stops one unit, treating a not-found unit as already stopped.
func stopUnit(unit string) error {
	_, stderr, err := runCapture(exec.Command("systemctl", "--user", "stop", unit))
	if err == nil {
		return nil
	}
	msg := strings.ToLower(stderr)
	if strings.Contains(msg, "not found") || strings.Contains(msg, "not loaded") {
		return nil
	}
	return fmt.Errorf("systemctl stop %s: %s: %w", unit, strings.TrimSpace(stderr), err)
}

// systemctlAvailable reports whether the reminder provider backed by systemd
// can be used at all.
func systemctlAvailable() bool {
	return runtime.GOOS == "linux" && lookPath("systemctl")
}

// RemindersSupported reports whether any reminder provider is available. It
// is a cheap PATH check for `jot doctor`, it runs nothing.
func RemindersSupported() bool {
	if lookPath("omarchy") {
		return true
	}
	return lookPath("systemd-run") && lookPath("notify-send")
}

// systemdTimer is one row of `systemctl list-timers --output=json`. systemd
// versions disagree on the key spelling: the bus property names
// (NextElapseUSecRealtime, NextElapseUSecMonotonic) and, on some versions, a
// short "next" key are all accepted. The values are microseconds.
type systemdTimer struct {
	Unit              string          `json:"unit"`
	NextRealtime      json.RawMessage `json:"NextElapseUSecRealtime"`
	NextMonotonic     json.RawMessage `json:"NextElapseUSecMonotonic"`
	NextShortRealtime json.RawMessage `json:"next"`
}

// parseTimerList turns list-timers JSON into jot reminders, keeping only
// jot-reminder-*.timer units. boot is the reference point for monotonic
// elapse times; a zero boot means monotonic elapses cannot be converted and
// yield a zero At. stashDir is where reminder messages wait; an empty dir
// skips message lookup.
func parseTimerList(data []byte, boot time.Time, stashDir string) ([]Reminder, error) {
	var entries []systemdTimer
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parse systemctl list-timers output: %w", err)
	}
	out := make([]Reminder, 0, len(entries))
	for _, e := range entries {
		if !strings.HasPrefix(e.Unit, reminderUnitPrefix) || !strings.HasSuffix(e.Unit, ".timer") {
			continue
		}
		r := Reminder{Unit: strings.TrimSuffix(e.Unit, ".timer")}
		if usec, ok := decodeUSec(e.NextRealtime); ok && usec > 0 {
			r.At = time.UnixMicro(usec)
		} else if usec, ok := decodeUSec(e.NextShortRealtime); ok && usec > 0 {
			r.At = time.UnixMicro(usec)
		} else if usec, ok := decodeUSec(e.NextMonotonic); ok && usec > 0 && !boot.IsZero() {
			r.At = boot.Add(time.Duration(usec) * time.Microsecond)
		}
		if stashDir != "" {
			r.Message = readStash(stashPath(stashDir, r.Unit))
		}
		out = append(out, r)
	}
	return out, nil
}

// decodeUSec parses a microsecond value that systemd may have serialized as
// a JSON number or a quoted number. Missing or unparseable values are
// reported as absent, not as errors.
func decodeUSec(raw json.RawMessage) (int64, bool) {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" {
		return 0, false
	}
	if strings.HasPrefix(s, `"`) {
		var v string
		if err := json.Unmarshal(raw, &v); err != nil {
			return 0, false
		}
		s = strings.TrimSpace(v)
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		f, ferr := strconv.ParseFloat(s, 64)
		if ferr != nil {
			return 0, false
		}
		n = int64(f)
	}
	if n < 0 {
		return 0, false
	}
	return n, true
}

// bootTime reads the boot time from /proc/stat, used to turn monotonic
// elapse times into wall-clock instants. Anything goes wrong: zero time.
func bootTime() (time.Time, error) {
	if runtime.GOOS != "linux" {
		return time.Time{}, errors.New("boot time unavailable")
	}
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return time.Time{}, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if rest, ok := strings.CutPrefix(line, "btime "); ok {
			sec, err := strconv.ParseInt(strings.TrimSpace(rest), 10, 64)
			if err != nil {
				return time.Time{}, err
			}
			return time.Unix(sec, 0), nil
		}
	}
	return time.Time{}, errors.New("/proc/stat has no btime")
}

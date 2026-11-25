package jot

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestOmarchyMinutes(t *testing.T) {
	tests := []struct {
		in      time.Duration
		want    int
		wantErr bool
	}{
		{in: 0, wantErr: true},
		{in: -time.Minute, wantErr: true},
		{in: 59 * time.Second, wantErr: true}, // sub-minute: rejected, not rounded
		{in: time.Minute, want: 1},
		{in: 61 * time.Second, want: 2}, // rounds up
		{in: 90 * time.Second, want: 2},
		{in: 30 * time.Minute, want: 30},
	}
	for _, tt := range tests {
		got, err := omarchyMinutes(tt.in)
		if tt.wantErr {
			if err == nil {
				t.Errorf("omarchyMinutes(%s) = %d, want error", tt.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("omarchyMinutes(%s): %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("omarchyMinutes(%s) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestNewReminderUnit(t *testing.T) {
	at := time.Unix(1700000000, 123456789)
	unit := newReminderUnit(at)
	if !strings.HasPrefix(unit, reminderUnitPrefix) {
		t.Errorf("unit %q does not start with %q", unit, reminderUnitPrefix)
	}
	if !strings.Contains(unit, strconv.FormatInt(at.UnixNano(), 10)) {
		t.Errorf("unit %q does not embed its nanosecond timestamp", unit)
	}
	if again := newReminderUnit(at.Add(time.Nanosecond)); again == unit {
		t.Errorf("units for distinct instants collide: %q", unit)
	}
}

func TestReminderStashDir(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", filepath.Join(t.TempDir(), "run"))
	if got, want := reminderStashDir(), filepath.Join(os.Getenv("XDG_RUNTIME_DIR"), "jot-reminders"); got != want {
		t.Errorf("reminderStashDir() = %q, want %q", got, want)
	}
	t.Setenv("XDG_RUNTIME_DIR", "  ")
	want := filepath.Join(os.TempDir(), "jot-reminders-"+strconv.Itoa(os.Getuid()))
	if got := reminderStashDir(); got != want {
		t.Errorf("fallback reminderStashDir() = %q, want %q", got, want)
	}
}

func TestStashRoundtrip(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	message := "feed the cat & 'quote' check"
	unit := newReminderUnit(time.Now())
	path, err := writeStash(dir, unit, message)
	if err != nil {
		t.Fatal(err)
	}
	if path != stashPath(dir, unit) {
		t.Errorf("writeStash path = %q, want %q", path, stashPath(dir, unit))
	}
	if got := readStash(path); got != message {
		t.Errorf("readStash = %q, want %q", got, message)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("stash file mode = %o, want 600", perm)
	}
	if got := readStash(filepath.Join(dir, "jot-reminder-absent.message")); got != "" {
		t.Errorf("readStash of missing file = %q, want empty", got)
	}
}

func TestParseTimerListRealtime(t *testing.T) {
	stash := t.TempDir()
	at := time.Unix(1800000000, 0)
	usec := at.UnixMicro()
	data := []byte(`[
		{"unit":"` + reminderUnitPrefix + `123.timer","NextElapseUSecRealtime":` + strconv.FormatInt(usec, 10) + `},
		{"unit":"` + reminderUnitPrefix + `456.timer","NextElapseUSecRealtime":"` + strconv.FormatInt(usec+5, 10) + `"},
		{"unit":"omarchy-calendar-sync.timer","NextElapseUSecRealtime":` + strconv.FormatInt(usec, 10) + `}
	]`)
	if err := os.WriteFile(stashPath(stash, reminderUnitPrefix+"123"), []byte("stand up"), 0o600); err != nil {
		t.Fatal(err)
	}
	reminders, err := parseTimerList(data, time.Time{}, stash)
	if err != nil {
		t.Fatal(err)
	}
	if len(reminders) != 2 {
		t.Fatalf("parseTimerList returned %d reminders, want 2: %+v", len(reminders), reminders)
	}
	first, second := reminders[0], reminders[1]
	if first.Unit != reminderUnitPrefix+"123" || first.Message != "stand up" {
		t.Errorf("first reminder = %+v, want unit %q with stash message", first, reminderUnitPrefix+"123")
	}
	if !first.At.Equal(at) {
		t.Errorf("first At = %v, want %v", first.At, at)
	}
	if second.Unit != reminderUnitPrefix+"456" || second.Message != "" {
		t.Errorf("second reminder = %+v, want unit %q with empty message", second, reminderUnitPrefix+"456")
	}
	if !second.At.Equal(at.Add(5 * time.Microsecond)) {
		t.Errorf("second At = %v, want %v", second.At, at.Add(5*time.Microsecond))
	}
}

func TestParseTimerListMonotonic(t *testing.T) {
	boot := time.Unix(1700000000, 0)
	delay := 90 * time.Second
	data := []byte(`[{"unit":"` + reminderUnitPrefix + `789.timer","NextElapseUSecMonotonic":` +
		strconv.FormatInt(delay.Microseconds(), 10) + `}]`)
	reminders, err := parseTimerList(data, boot, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(reminders) != 1 {
		t.Fatalf("parseTimerList returned %d reminders, want 1", len(reminders))
	}
	if want := boot.Add(delay); !reminders[0].At.Equal(want) {
		t.Errorf("At = %v, want %v", reminders[0].At, want)
	}
}

func TestParseTimerListShortKeyAndFallbacks(t *testing.T) {
	// Older systemctl JSON output uses the short "next" key.
	data := []byte(`[{"unit":"` + reminderUnitPrefix + `1.timer","next":1000}]`)
	reminders, err := parseTimerList(data, time.Time{}, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(reminders) != 1 || !reminders[0].At.Equal(time.UnixMicro(1000)) {
		t.Fatalf("short-key parse failed: %+v", reminders)
	}
	// Monotonic without a boot time yields a zero At, not an error.
	data = []byte(`[{"unit":"` + reminderUnitPrefix + `2.timer","NextElapseUSecMonotonic":1000}]`)
	reminders, err = parseTimerList(data, time.Time{}, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(reminders) != 1 || !reminders[0].At.IsZero() {
		t.Fatalf("monotonic without boot should give zero At: %+v", reminders)
	}
	// Elapsed timers report zero or negative elapse; keep the unit with no At.
	data = []byte(`[{"unit":"` + reminderUnitPrefix + `3.timer","NextElapseUSecRealtime":0},{"unit":"` + reminderUnitPrefix + `4.timer","NextElapseUSecRealtime":-5}]`)
	reminders, err = parseTimerList(data, time.Time{}, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(reminders) != 2 {
		t.Fatalf("elapsed timers should be listed: %+v", reminders)
	}
	for _, r := range reminders {
		if !r.At.IsZero() {
			t.Errorf("elapsed timer %q has non-zero At %v", r.Unit, r.At)
		}
	}
}

func TestParseTimerListBadJSON(t *testing.T) {
	if _, err := parseTimerList([]byte("not json"), time.Time{}, ""); err == nil {
		t.Error("parseTimerList of malformed JSON should fail")
	}
}

func TestRemindRejectsBadInput(t *testing.T) {
	j := New(Config{Root: t.TempDir()})
	for _, tt := range []struct {
		name    string
		message string
		in      time.Duration
	}{
		{name: "empty message", message: "", in: time.Minute},
		{name: "blank message", message: "   \t ", in: time.Minute},
		{name: "zero duration", message: "hello", in: 0},
		{name: "negative duration", message: "hello", in: -time.Minute},
	} {
		if _, err := j.Remind(tt.message, tt.in); err == nil {
			t.Errorf("%s: Remind should fail, got no error", tt.name)
		} else if err == ErrRemindersUnsupported {
			t.Errorf("%s: input validation must run before provider selection", tt.name)
		}
	}
}

// stubPath points PATH at a fresh directory holding the named stub
// executables plus links to the real tools the background script needs, so
// provider selection does not depend on the host.
func stubPath(t *testing.T, stubs map[string]string) string {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("provider stubs are written for Linux")
	}
	dir := t.TempDir()
	for _, tool := range []string{"sh", "sleep", "cat", "rm"} {
		real, err := exec.LookPath(tool)
		if err != nil {
			t.Skipf("%s not found: %v", tool, err)
		}
		if err := os.Symlink(real, filepath.Join(dir, tool)); err != nil {
			t.Fatal(err)
		}
	}
	for name, body := range stubs {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)
	return dir
}

func TestReminderProvider(t *testing.T) {
	all := map[string]string{"omarchy": "", "systemd-run": "", "notify-send": ""}
	tests := []struct {
		name    string
		stubs   map[string]string
		setting string
		in      time.Duration
		want    string
		wantErr error
	}{
		{name: "auto prefers omarchy", stubs: all, setting: "", in: 5 * time.Minute, want: RemindOmarchy},
		{name: "auto skips omarchy under a minute", stubs: all, setting: "auto", in: 30 * time.Second, want: RemindSystemd},
		{name: "auto falls back to background", stubs: map[string]string{"notify-send": ""}, setting: "auto", in: time.Minute, want: RemindBackground},
		{name: "auto with nothing", stubs: nil, setting: "auto", in: time.Minute, wantErr: ErrRemindersUnsupported},
		{name: "pinned provider", stubs: all, setting: "background", in: time.Minute, want: RemindBackground},
		{name: "off", stubs: all, setting: "off", in: time.Minute, wantErr: ErrRemindersOff},
	}
	for _, tt := range tests {
		stubPath(t, tt.stubs)
		got, err := ReminderProvider(tt.setting, tt.in)
		if tt.wantErr != nil {
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("%s: err = %v, want %v", tt.name, err, tt.wantErr)
			}
			continue
		}
		if err != nil || got != tt.want {
			t.Errorf("%s: got %q, %v; want %q", tt.name, got, err, tt.want)
		}
	}

	stubPath(t, map[string]string{"notify-send": ""})
	for _, setting := range []string{"omarchy", "systemd", "bogus"} {
		if p, err := ReminderProvider(setting, time.Minute); err == nil {
			t.Errorf("ReminderProvider(%q) = %q, want an error (pinned provider missing or unknown)", setting, p)
		}
	}
}

func TestBackgroundReminderFiresAndClears(t *testing.T) {
	out := filepath.Join(t.TempDir(), "notified")
	stubPath(t, map[string]string{"notify-send": `printf '%s|' "$@" > ` + shQuote(out)})
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	j := New(Config{Root: t.TempDir(), Reminders: RemindBackground})

	// Cleared before it fires: it must never notify.
	if _, err := j.Remind("cancel me", time.Hour); err != nil {
		t.Fatal(err)
	}
	rems, err := j.Reminders()
	if err != nil || len(rems) != 1 || rems[0].Message != "cancel me" {
		t.Fatalf("Reminders() = %+v, %v; want the one pending reminder", rems, err)
	}
	if n, err := j.ClearReminders(); err != nil || n != 1 {
		t.Fatalf("ClearReminders() = %d, %v; want 1", n, err)
	}
	if rems, _ := j.Reminders(); len(rems) != 0 {
		t.Fatalf("Reminders() after clear = %+v, want none", rems)
	}

	// A message that looks like a flag still arrives as plain text.
	if _, err := j.Remind("-u critical $(rm -rf ~)", time.Second); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		data, err := os.ReadFile(out)
		if err == nil {
			if got, want := string(data), "-a|jot|--|jot|-u critical $(rm -rf ~)|"; got != want {
				t.Fatalf("notify-send args = %q, want %q", got, want)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("background reminder did not fire within 10s")
		}
		time.Sleep(100 * time.Millisecond)
	}
}

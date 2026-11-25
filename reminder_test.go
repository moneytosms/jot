package jot

import (
	"os"
	"path/filepath"
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

func TestRemindersSupported(t *testing.T) {
	RemindersSupported() // cheap PATH probe; its value depends on the host.
}

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/moneytosms/jot"
)

// extOut joins a command's stdout so table assertions read cleanly.
func extOut(t *testing.T, stdin string, args ...string) string {
	t.Helper()
	return mustRun(t, stdin, args...)
}

func TestCLIShowHeaderBar(t *testing.T) {
	testEnv(t)
	mustRun(t, "", "add", "-d", "2026-01-05", "first note")
	mustRun(t, "", "add", "-d", "2026-01-05", "second note")

	out := extOut(t, "", "show", "2026-01-05")
	weekday, _ := time.Parse("2006-01-02", "2026-01-05")
	for _, want := range []string{
		"-- default . 2026-01-05 (" + weekday.Format("Mon") + ") . 2 entries",
		"first note",
		"second note",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("show header missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "(Mon )") || strings.Count(out, "\n\n## ") != 2 {
		t.Errorf("unexpected header or entry count:\n%s", out)
	}

	// An empty day keeps its message and gets no bar.
	if out := extOut(t, "", "show", "2026-02-02"); !strings.Contains(out, "no entries") || strings.Contains(out, "-- default .") {
		t.Errorf("unexpected empty-day output:\n%s", out)
	}
}

func TestCLIShowCalendarGrid(t *testing.T) {
	testEnv(t)
	// January 2026: two entries on the 5th (dot), three on the 6th (star).
	for _, body := range []string{"one", "two"} {
		mustRun(t, "", "add", "-d", "2026-01-05", body)
	}
	for _, body := range []string{"a", "b", "c"} {
		mustRun(t, "", "add", "-d", "2026-01-06", body)
	}

	out := extOut(t, "", "show", "--calendar", "2026-01")
	for _, want := range []string{
		"January 2026",
		"Mo Tu We Th Fr Sa Su",
		"5.", // 1-2 entries -> dot
		"6*", // 3+ entries -> star
		"legend:",
		"5 entries in January 2026",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("calendar missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "[") {
		t.Errorf("past month must not bracket today:\n%s", out)
	}

	// A month with no files still prints a grid with a zero total.
	if out := extOut(t, "", "show", "--calendar", "2026-02"); !strings.Contains(out, "0 entries in February 2026") {
		t.Errorf("unexpected empty-month grid:\n%s", out)
	}

	// -j scopes the grid; plant one work entry on a different day.
	mustRun(t, "", "add", "-j", "work", "-d", "2026-01-07", "work thing")
	out = extOut(t, "", "show", "-j", "work", "--calendar", "2026-01")
	if !strings.Contains(out, "7.") || !strings.Contains(out, "1 entry in January 2026") {
		t.Errorf("journal scoping broke:\n%s", out)
	}
	if strings.Contains(out, "5.") || strings.Contains(out, "6*") {
		t.Errorf("default entries leaked into the work grid:\n%s", out)
	}

	// A missing journal keeps the friendly error.
	code, _, errw := runCLI(t, "", "show", "-j", "ghost", "--calendar", "2026-01")
	if code != 1 || !strings.Contains(errw, "no journal named") {
		t.Errorf("expected a friendly error, got %d %q", code, errw)
	}

	// Invalid month.
	code, _, errw = runCLI(t, "", "show", "--calendar", "2026/01")
	if code != 1 || !strings.Contains(errw, "invalid month") {
		t.Errorf("expected an invalid-month error, got %d %q", code, errw)
	}
}

func TestMonthGridPure(t *testing.T) {
	counts := []jot.DayCount{
		{Day: "2026-01-05", Entries: 2},
		{Day: "2026-01-06", Entries: 3},
		{Day: "2026-01-06", Entries: 1}, // duplicate day entries accumulate
	}
	// A date outside the displayed month: no brackets.
	joined := strings.Join(monthGrid(2026, time.January, counts, time.Date(2026, 2, 10, 12, 0, 0, 0, time.Local)), "\n")
	for _, want := range []string{"January 2026", "Mo Tu We Th Fr Sa Su", "5.", "6*", "6 entries in January 2026"} {
		if !strings.Contains(joined, want) {
			t.Errorf("grid missing %q:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "[") {
		t.Errorf("no brackets expected outside the month:\n%s", joined)
	}

	// Today inside the displayed month is bracketed and keeps its marker.
	joined = strings.Join(monthGrid(2026, time.November, []jot.DayCount{{Day: "2026-11-03", Entries: 4}}, time.Date(2026, 11, 3, 9, 0, 0, 0, time.Local)), "\n")
	if !strings.Contains(joined, "[ 3]*") {
		t.Errorf("today with entries not bracketed:\n%s", joined)
	}
	if !strings.Contains(joined, "legend: . 1-2 entries, * 3+ entries, [..] today") {
		t.Errorf("today legend missing:\n%s", joined)
	}

	// January 2026 starts on a Thursday: three blank cells (5 columns each)
	// before day 1.
	lines := monthGrid(2026, time.January, nil, time.Date(2026, 2, 10, 12, 0, 0, 0, time.Local))
	if want := strings.Repeat(" ", 3*5+2) + "1"; !strings.HasPrefix(lines[2], want) {
		t.Errorf("first week row misaligned: %q", lines[2])
	}
}

func TestDayHeader(t *testing.T) {
	got := dayHeader("work", "2025-11-25", 3, 60)
	if !strings.HasPrefix(got, "-- work . 2025-11-25 (Tue) . 3 entries ") || len(got) != 60 {
		t.Errorf("unexpected header %q (len %d)", got, len(got))
	}
	if got := dayHeader("w", "2025-11-25", 1, 60); !strings.Contains(got, "1 entry ") || strings.Contains(got, "1 entries") {
		t.Errorf("singular broken: %q", got)
	}
}

func TestShowCalendarArgRewrite(t *testing.T) {
	cases := []struct {
		in   []string
		want []string
	}{
		{[]string{"--calendar"}, []string{"--calendar=current"}},
		{[]string{"--calendar", "-a"}, []string{"--calendar=current", "-a"}},
		{[]string{"--calendar", "2026-01"}, []string{"--calendar", "2026-01"}},
		{[]string{"--calendar=2026-01", "x"}, []string{"--calendar=2026-01", "x"}},
	}
	for _, tc := range cases {
		if got := showCalendarArg(tc.in); strings.Join(got, " ") != strings.Join(tc.want, " ") {
			t.Errorf("showCalendarArg(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestParseCalendarMonth(t *testing.T) {
	now := time.Date(2026, 11, 25, 12, 0, 0, 0, time.Local)
	for _, in := range []string{"", "current"} {
		y, m, err := parseCalendarMonth(in, now)
		if err != nil || y != 2026 || m != time.November {
			t.Errorf("parseCalendarMonth(%q) = %d %s %v", in, y, m, err)
		}
	}
	y, m, err := parseCalendarMonth("2026-01", now)
	if err != nil || y != 2026 || m != time.January {
		t.Errorf("parseCalendarMonth(2026-01) = %d %s %v", y, m, err)
	}
	if _, _, err := parseCalendarMonth("2026/01", now); err == nil {
		t.Error("expected an error for 2026/01")
	}
}

func TestCLISearchGrouped(t *testing.T) {
	testEnv(t)
	mustRun(t, "", "add", "-d", "2026-01-05", "gym at noon")
	mustRun(t, "", "add", "-d", "2026-01-05", "gym again")
	mustRun(t, "", "add", "-j", "work", "-d", "2026-01-04", "gym at work")

	out := extOut(t, "", "search", "gym")
	for _, want := range []string{
		"default . 2026-01-05 . 2 hits",
		"  default/2026-01-05.md:3: gym at noon",
		"work . 2026-01-04 . 1 hit",
		"3 matches in 2 journals",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("grouped search missing %q:\n%s", want, out)
		}
	}

	one := extOut(t, "", "search", "-j", "work", "gym")
	if !strings.Contains(one, "1 match in 1 journal") {
		t.Errorf("unexpected single-hit summary:\n%s", one)
	}
}

func TestCLISearchFuzzy(t *testing.T) {
	testEnv(t)
	mustRun(t, "", "add", "-j", "work", "-d", "2026-01-05", "rewrite the parser today")
	mustRun(t, "", "add", "-d", "2026-01-04", "plan the rewrite next week")

	out := extOut(t, "", "search", "--fuzzy", "rewrite")
	for _, want := range []string{
		"work . 2026-01-05 . ",
		"rewrite the parser today",
		"default . 2026-01-04 . ",
		"plan the rewrite next week",
		"2 matches",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("fuzzy search missing %q:\n%s", want, out)
		}
	}

	scoped := extOut(t, "", "search", "--fuzzy", "-j", "work", "rewrite")
	if strings.Contains(scoped, "default . ") {
		t.Errorf("-j leaked across journals:\n%s", scoped)
	}
	if !strings.Contains(scoped, "1 match\n") {
		t.Errorf("unexpected singular summary:\n%s", scoped)
	}

	limited := extOut(t, "", "search", "--fuzzy", "--limit", "1", "rewrite")
	if !strings.Contains(limited, "showing first 1") {
		t.Errorf("limit not reported:\n%s", limited)
	}
}

func TestPreviewLinesTruncate(t *testing.T) {
	got := previewLines("one\ntwo\nthree\nfour\nfive", 3, 80)
	if len(got) != 3 || got[0] != "one" || got[2] != "three" {
		t.Errorf("unexpected preview %v", got)
	}
	long := strings.Repeat("x", 90)
	if got := truncateCols(long, 80); len(got) != 83 || !strings.HasSuffix(got, "...") {
		t.Errorf("unexpected truncation (len %d)", len(got))
	}
	if got := truncateCols("short", 80); got != "short" {
		t.Errorf("short line changed: %q", got)
	}
}

func TestCLITodo(t *testing.T) {
	testEnv(t)
	body := "- [ ] call the dentist\n- [x] send the report"
	mustRun(t, "", "add", "-j", "work", "-d", "2026-01-05", "--", body)
	mustRun(t, "", "add", "-d", "2026-01-06", "--", "- [ ] water the plants")

	out := extOut(t, "", "todo")
	for _, want := range []string{
		"[ ]", "work", "2026-01-05", "call the dentist",
		"[x]", "work", "2026-01-05", "send the report",
		"2026-01-06", "water the plants",
		"toggle with jot edit -d 2026-01-06 -j default",
		"2 open . 1 done",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("todo missing %q:\n%s", want, out)
		}
	}
	// Newest day first: 2026-01-06 lines come before 2026-01-05.
	if strings.Index(out, "water the plants") > strings.Index(out, "call the dentist") {
		t.Errorf("todo not sorted newest first:\n%s", out)
	}

	open := extOut(t, "", "todo", "--open")
	if !strings.Contains(open, "call the dentist") || !strings.Contains(open, "water the plants") {
		t.Errorf("--open lost unchecked items:\n%s", open)
	}
	if strings.Contains(open, "send the report") {
		t.Errorf("--open leaked a done item:\n%s", open)
	}

	done := extOut(t, "", "todo", "--done")
	if !strings.Contains(done, "send the report") || strings.Contains(done, "call the dentist") {
		t.Errorf("--done filter broken:\n%s", done)
	}

	limited := extOut(t, "", "todo", "--limit", "1")
	if n := strings.Count(limited, "[ ]") + strings.Count(limited, "[x]"); n != 1 {
		t.Errorf("--limit 1 showed %d items:\n%s", n, limited)
	}

	// A missing journal keeps the friendly error.
	code, _, errw := runCLI(t, "", "todo", "-j", "ghost")
	if code != 1 || !strings.Contains(errw, "no journal named") {
		t.Errorf("expected a friendly error, got %d %q", code, errw)
	}
}

func TestParseReminderDuration(t *testing.T) {
	cases := []struct {
		in      string
		want    time.Duration
		wantErr bool
	}{
		{"90s", 90 * time.Second, false},
		{"20m", 20 * time.Minute, false},
		{"2h", 2 * time.Hour, false},
		{"1h30m", 90 * time.Minute, false},
		{"90", 90 * time.Minute, false}, // bare number means minutes
		{" 15m ", 15 * time.Minute, false},
		{"0", 0, true},
		{"-5m", 0, true},
		{"abc", 0, true},
		{"", 0, true},
	}
	for _, tc := range cases {
		got, err := parseReminderDuration(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("parseReminderDuration(%q) expected an error, got %s", tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseReminderDuration(%q) failed: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("parseReminderDuration(%q) = %s, want %s", tc.in, got, tc.want)
		}
	}
}

func TestCLIRemindUsageErrors(t *testing.T) {
	testEnv(t)
	// These paths fail before any reminder provider is touched.
	cases := [][]string{
		{"only-a-message"},
		{"hello", "not-a-duration"},
		{"stretch", "20x"},
	}
	for _, args := range cases {
		code, _, errw := runCLI(t, "", append([]string{"remind"}, args...)...)
		if code != 1 {
			t.Errorf("jot remind %v: expected exit 1, got %d (%q)", args, code, errw)
		}
	}
}

func TestCLIJournalNewAndRm(t *testing.T) {
	dir := testEnv(t)
	root := filepath.Join(dir, "jot")

	out := extOut(t, "", "journal", "new", "ideas")
	if want := filepath.Join(root, "ideas"); !strings.Contains(out, want) {
		t.Errorf("journal new did not print %q: %q", want, out)
	}
	if _, err := os.Stat(filepath.Join(root, "ideas")); err != nil {
		t.Fatalf("journal directory missing: %v", err)
	}
	if code, _, errw := runCLI(t, "", "journal", "new", "ideas"); code != 1 || !strings.Contains(errw, "already exists") {
		t.Errorf("expected an already-exists error, got %d %q", code, errw)
	}

	// rm --yes archives into <root>/backups, then deletes the directory.
	out = extOut(t, "", "journal", "rm", "--yes", "ideas")
	if !strings.Contains(out, "archived ideas to") || !strings.Contains(out, "deleted ideas") {
		t.Errorf("unexpected journal rm output:\n%s", out)
	}
	archives, err := os.ReadDir(filepath.Join(root, "backups"))
	if err != nil || len(archives) != 1 || !strings.HasPrefix(archives[0].Name(), "jot-ideas-") {
		t.Fatalf("expected one jot-ideas archive, got %v (%v)", archives, err)
	}
	if _, err := os.Stat(filepath.Join(root, "ideas")); !os.IsNotExist(err) {
		t.Fatalf("ideas still exists: %v", err)
	}
}

func TestCLIJournalRmRefusesDefault(t *testing.T) {
	dir := testEnv(t)
	mustRun(t, "", "add", "keep me")

	code, _, errw := runCLI(t, "", "journal", "rm", "default")
	if code != 1 || !strings.Contains(errw, "default journal cannot be removed") {
		t.Fatalf("expected a default-journal refusal, got %d %q", code, errw)
	}
	if _, err := os.Stat(filepath.Join(dir, "jot", "default")); err != nil {
		t.Fatalf("default journal was touched: %v", err)
	}
}

func TestCLIJournalRmConfirmation(t *testing.T) {
	dir := testEnv(t)
	root := filepath.Join(dir, "jot")
	mustRun(t, "", "journal", "new", "work")
	mustRun(t, "", "add", "-j", "work", "note")

	// A mismatched typed confirmation aborts and leaves the journal intact.
	code, out, errw := runCLI(t, "wrong-name\n", "journal", "rm", "work")
	if code != 0 || !strings.Contains(out, "aborted") {
		t.Fatalf("expected an abort, got %d %q %q", code, out, errw)
	}
	if _, err := os.Stat(filepath.Join(root, "work")); err != nil {
		t.Fatalf("journal deleted despite abort: %v", err)
	}
	// The abort still kept the archive that was written first.
	archives, err := os.ReadDir(filepath.Join(root, "backups"))
	if err != nil || len(archives) != 1 {
		t.Fatalf("expected the pre-delete archive, got %v (%v)", archives, err)
	}

	// Typing the exact name deletes it.
	code, out, errw = runCLI(t, "work\n", "journal", "rm", "work")
	if code != 0 || !strings.Contains(out, "deleted work") {
		t.Fatalf("expected deletion, got %d %q %q", code, out, errw)
	}
	if _, err := os.Stat(filepath.Join(root, "work")); !os.IsNotExist(err) {
		t.Fatalf("work still exists: %v", err)
	}
}

func TestCLIJournalRmNonTerminalStdin(t *testing.T) {
	testEnv(t)
	mustRun(t, "", "journal", "new", "notes")
	// Piped stdin (a real non-terminal file) without --yes is refused. A
	// regular file works as a stand-in: it is an *os.File but not a terminal
	// (unlike /dev/null, which is a character device).
	f, err := os.CreateTemp(t.TempDir(), "stdin")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	var out, errw bytes.Buffer
	code := run([]string{"journal", "rm", "notes"}, f, &out, &errw)
	if code != 1 || !strings.Contains(errw.String(), "--yes") {
		t.Fatalf("expected a --yes refusal, got %d %q", code, errw.String())
	}
}

func TestCLIJournalRenameAndLs(t *testing.T) {
	dir := testEnv(t)
	mustRun(t, "", "journal", "new", "work")

	out := extOut(t, "", "journal", "rename", "work", "deep-work")
	if !strings.Contains(out, "renamed work to deep-work") {
		t.Errorf("unexpected rename output: %q", out)
	}
	if _, err := os.Stat(filepath.Join(dir, "jot", "work")); !os.IsNotExist(err) {
		t.Fatalf("old journal still there: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "jot", "deep-work")); err != nil {
		t.Fatalf("renamed journal missing: %v", err)
	}

	// journal ls matches jot ls.
	ls := extOut(t, "", "journal", "ls")
	viaLs := extOut(t, "", "ls")
	if strings.TrimSpace(ls) != strings.TrimSpace(viaLs) {
		t.Errorf("journal ls differs from ls:\n%s\n---\n%s", ls, viaLs)
	}

	// The default journal cannot be renamed.
	if code, _, errw := runCLI(t, "", "journal", "rename", "default", "x"); code != 1 || !strings.Contains(errw, "cannot be removed or renamed") {
		t.Errorf("expected a default-journal refusal, got %d %q", code, errw)
	}
}

func TestCLINewCommandHelpFlags(t *testing.T) {
	testEnv(t)
	for _, args := range [][]string{
		{"todo", "-h"}, {"todo", "--help"}, {"remind", "-h"},
		{"journal", "-h"}, {"help", "todo"}, {"help", "remind"}, {"help", "journal"},
	} {
		topic := args[0]
		if topic == "help" {
			topic = args[1]
		}
		out := extOut(t, "", args...)
		if !strings.Contains(out, "jot "+topic) {
			t.Errorf("%v did not print a help page:\n%s", args, out)
		}
	}
}

func TestCLIDoctorExtChecks(t *testing.T) {
	testEnv(t)
	out := extOut(t, "", "doctor")
	for _, want := range []string{"reminders", "notify-send", "systemd-run", "vendor"} {
		if !strings.Contains(out, want) {
			t.Errorf("doctor missing the %q check:\n%s", want, out)
		}
	}
	// Extra checks are warnings only: doctor must still exit 0.
	if code, _, errw := runCLI(t, "", "doctor"); code != 0 {
		t.Errorf("doctor exited %d: %q", code, errw)
	}
}

func TestCLIThemeStyles(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	t.Setenv("JOT_THEME", "light")
	var buf bytes.Buffer
	styles, color := themeStyles(&buf)
	if color || styles != jot.PlainStyles() {
		t.Errorf("NO_COLOR must force plain styles, got %+v color=%v", styles, color)
	}
	t.Setenv("NO_COLOR", "")
	styles, color = themeStyles(&buf)
	if color || styles != jot.PlainStyles() {
		t.Errorf("a non-terminal writer must force plain styles, got %+v color=%v", styles, color)
	}
}
func TestCLISearchRejectsFlagsAfterQuery(t *testing.T) {
	testEnv(t)
	mustRun(t, "", "add", "gym session")

	if code, _, errOut := runCLI(t, "", "search", "gym", "--fuzzy"); code == 0 {
		t.Fatalf("search accepted a flag after the query: stderr=%s", errOut)
	} else if !strings.Contains(errOut, "flags must come before the query") {
		t.Fatalf("unexpected error: %s", errOut)
	}

	// The documented order works.
	if code, out, errOut := runCLI(t, "", "search", "--fuzzy", "--limit", "10", "gym"); code != 0 {
		t.Fatalf("search --fuzzy --limit 10 gym failed: %s%s", out, errOut)
	}
}

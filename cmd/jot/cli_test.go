package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/moneytosms/jot"
)

// testEnv points the CLI at a temporary root and config so tests never touch
// the developer's journals.
func testEnv(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv(jot.EnvRoot, filepath.Join(dir, "jot"))
	t.Setenv(jot.EnvConfig, filepath.Join(dir, "config.json"))
	t.Setenv(jot.EnvJournal, "")
	t.Setenv(jot.EnvEditor, "true")
	t.Setenv(jot.EnvInstallDir, filepath.Join(dir, "bin"))
	return dir
}

func runCLI(t *testing.T, stdin string, args ...string) (int, string, string) {
	t.Helper()
	var out, errw bytes.Buffer
	code := run(args, strings.NewReader(stdin), &out, &errw)
	return code, out.String(), errw.String()
}

func mustRun(t *testing.T, stdin string, args ...string) string {
	t.Helper()
	code, out, errw := runCLI(t, stdin, args...)
	if code != 0 {
		t.Fatalf("jot %s failed (%d): %s", strings.Join(args, " "), code, errw)
	}
	return out
}

func TestCLIVersion(t *testing.T) {
	testEnv(t)
	out := mustRun(t, "", "version")
	if !strings.HasPrefix(out, "jot ") {
		t.Fatalf("unexpected version output %q", out)
	}
	if out = mustRun(t, "", "--version"); !strings.HasPrefix(out, "jot ") {
		t.Fatalf("unexpected version output %q", out)
	}
}

func TestCLIHelp(t *testing.T) {
	testEnv(t)
	help := mustRun(t, "")
	for _, want := range []string{"Usage:", "Commands:", "jot add"} {
		if !strings.Contains(help, want) {
			t.Errorf("help missing %q", want)
		}
	}
	cmdHelp := mustRun(t, "", "help", "add")
	if !strings.Contains(cmdHelp, "jot add [flags]") {
		t.Fatalf("unexpected command help: %q", cmdHelp)
	}
	code, _, errw := runCLI(t, "", "help", "nope")
	if code == 0 || !strings.Contains(errw, "no help") {
		t.Fatalf("expected an error for unknown help topic, got %d %q", code, errw)
	}
}

func TestCLIAddShow(t *testing.T) {
	testEnv(t)
	out := mustRun(t, "", "add", "shipped", "the", "parser")
	if !strings.Contains(out, "added to default/") {
		t.Fatalf("unexpected add output %q", out)
	}

	shown := mustRun(t, "", "show")
	if !strings.Contains(shown, "shipped the parser") {
		t.Fatalf("entry not shown: %q", shown)
	}

	// A different journal is separate.
	mustRun(t, "", "add", "-j", "work", "standup")
	work := mustRun(t, "", "show", "-j", "work")
	if !strings.Contains(work, "standup") || strings.Contains(work, "parser") {
		t.Fatalf("journal scoping broke: %q", work)
	}
}

func TestCLIAddFromStdin(t *testing.T) {
	testEnv(t)
	mustRun(t, "a piped note\n", "add")
	shown := mustRun(t, "", "show")
	if !strings.Contains(shown, "a piped note") {
		t.Fatalf("stdin entry not stored: %q", shown)
	}
}

func TestCLIAddRejectsEmpty(t *testing.T) {
	testEnv(t)
	code, _, errw := runCLI(t, "", "add")
	if code != 1 {
		t.Fatalf("expected failure, got %d", code)
	}
	if !strings.Contains(errw, "body is empty") {
		t.Fatalf("unexpected error %q", errw)
	}
}

func TestCLIAddDateAndShowAll(t *testing.T) {
	testEnv(t)
	mustRun(t, "", "add", "-d", "2026-01-05", "monday note")
	mustRun(t, "", "add", "-d", "2026-01-06", "tuesday note")

	shown := mustRun(t, "", "show", "-a")
	for _, want := range []string{"# default · 2026-01-05", "monday note", "# default · 2026-01-06", "tuesday note"} {
		if !strings.Contains(shown, want) {
			t.Errorf("show -a missing %q:\n%s", want, shown)
		}
	}

	day := mustRun(t, "", "show", "2026-01-05")
	if !strings.Contains(day, "monday note") || strings.Contains(day, "tuesday note") {
		t.Fatalf("single day filter broke: %q", day)
	}

	if out := mustRun(t, "", "show", "-d", "2026-02-02"); !strings.Contains(out, "no entries") {
		t.Fatalf("expected an empty-day message, got %q", out)
	}
}

func TestCLILs(t *testing.T) {
	testEnv(t)
	mustRun(t, "", "add", "-j", "work", "one")
	mustRun(t, "", "add", "-j", "ideas", "two")

	list := mustRun(t, "", "ls")
	for _, want := range []string{"JOURNAL", "ENTRIES", "ideas", "work"} {
		if !strings.Contains(list, want) {
			t.Errorf("ls missing %q:\n%s", want, list)
		}
	}

	days := mustRun(t, "", "ls", "work")
	if !strings.Contains(days, "DAY") || !strings.Contains(days, "1") {
		t.Fatalf("unexpected day listing: %q", days)
	}

	code, _, errw := runCLI(t, "", "ls", "ghost")
	if code != 1 || !strings.Contains(errw, "no journal named") {
		t.Fatalf("expected a friendly missing-journal error, got %d %q", code, errw)
	}
}

func TestCLISearch(t *testing.T) {
	testEnv(t)
	mustRun(t, "", "add", "-j", "work", "gym at noon")
	mustRun(t, "", "add", "walked to the gym")

	out := mustRun(t, "", "search", "GYM")
	if !strings.Contains(out, "work/") || !strings.Contains(out, "default/") {
		t.Fatalf("search did not cover every journal: %q", out)
	}

	scoped := mustRun(t, "", "search", "-j", "work", "gym")
	if strings.Contains(scoped, "default/") {
		t.Fatalf("search -j leaked into other journals: %q", scoped)
	}

	if out := mustRun(t, "", "search", "nothing-matches"); !strings.Contains(out, "no matches") {
		t.Fatalf("unexpected empty search output: %q", out)
	}
}

func TestCLIShowMissingJournal(t *testing.T) {
	testEnv(t)
	code, _, errw := runCLI(t, "", "show", "-j", "ghost")
	if code != 1 || !strings.Contains(errw, "no journal named") {
		t.Fatalf("expected a friendly error, got %d %q", code, errw)
	}
}

func TestCLIEditCreatesFile(t *testing.T) {
	dir := testEnv(t)
	mustRun(t, "", "edit")
	day, err := jot.ParseDay("")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "jot", "default", day+".md")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("edit did not create %s: %v", path, err)
	}
}

func TestCLIExportAndBackup(t *testing.T) {
	dir := testEnv(t)
	mustRun(t, "", "add", "-j", "work", "exported")

	exportDir := filepath.Join(dir, "export")
	out := mustRun(t, "", "export", "--out", exportDir)
	if !strings.Contains(out, "wrote") {
		t.Fatalf("unexpected export output %q", out)
	}
	data, err := os.ReadFile(filepath.Join(exportDir, "work.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "exported") {
		t.Fatalf("export content missing: %q", data)
	}

	backupDir := filepath.Join(dir, "backups")
	out = mustRun(t, "", "backup", "--out", backupDir)
	if !strings.Contains(out, "backup written to") {
		t.Fatalf("unexpected backup output %q", out)
	}
	entries, err := os.ReadDir(backupDir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected one archive, got %v (%v)", entries, err)
	}
}

func TestCLIInstall(t *testing.T) {
	dir := testEnv(t)
	bin := filepath.Join(dir, "bin")
	out := mustRun(t, "", "install", "--dir", bin)
	if !strings.Contains(out, bin) {
		t.Fatalf("unexpected install output %q", out)
	}
	if _, err := os.Stat(filepath.Join(bin, "jot")); err != nil {
		t.Fatalf("binary not installed: %v", err)
	}
}

func TestCLIUninstallConfirmation(t *testing.T) {
	dir := testEnv(t)
	mustRun(t, "", "add", "keep me")

	code, _, errw := runCLI(t, "n\n", "uninstall")
	if code != 1 || !strings.Contains(errw, "--yes") {
		t.Fatalf("expected a refusal without confirmation, got %d %q", code, errw)
	}
	if _, err := os.Stat(filepath.Join(dir, "jot")); err != nil {
		t.Fatalf("data was removed despite refusal: %v", err)
	}

	out := mustRun(t, "", "uninstall", "--yes")
	if !strings.Contains(out, "deleted") {
		t.Fatalf("unexpected uninstall output %q", out)
	}
	if _, err := os.Stat(filepath.Join(dir, "jot")); !os.IsNotExist(err) {
		t.Fatalf("root still exists: %v", err)
	}
}

func TestCLIDoctor(t *testing.T) {
	dir := testEnv(t)
	if code, _, errw := runCLI(t, "", "doctor"); code != 0 {
		t.Fatalf("doctor failed: %d %q", code, errw)
	}
	mustRun(t, "", "init")
	out := mustRun(t, "", "doctor")
	if !strings.Contains(out, "[ok]") {
		t.Fatalf("unexpected doctor output %q", out)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatal(err)
	}
}

func TestCLISetupYes(t *testing.T) {
	dir := testEnv(t)
	out := mustRun(t, "", "setup", "--yes", "--no-install", "--root", filepath.Join(dir, "journals"))
	if !strings.Contains(out, "config:") {
		t.Fatalf("unexpected setup output %q", out)
	}
	if _, err := os.Stat(filepath.Join(dir, "config.json")); err != nil {
		t.Fatalf("config not written: %v", err)
	}
}

func TestCLICommandHelpFlag(t *testing.T) {
	testEnv(t)
	out := mustRun(t, "", "add", "--help")
	if !strings.Contains(out, "jot add [flags]") {
		t.Fatalf("jot add --help did not print the command help: %q", out)
	}
	if out = mustRun(t, "", "show", "-h"); !strings.Contains(out, "jot show [flags]") {
		t.Fatalf("jot show -h did not print the command help: %q", out)
	}
}

func TestCLIUnknownCommand(t *testing.T) {
	testEnv(t)
	code, _, errw := runCLI(t, "", "wat")
	if code != 1 || !strings.Contains(errw, "unknown command") {
		t.Fatalf("expected an unknown-command error, got %d %q", code, errw)
	}
}

func TestCLIInvalidDate(t *testing.T) {
	testEnv(t)
	code, _, errw := runCLI(t, "", "add", "-d", "someday", "text")
	if code != 1 || !strings.Contains(errw, "invalid date") {
		t.Fatalf("expected an invalid-date error, got %d %q", code, errw)
	}
}

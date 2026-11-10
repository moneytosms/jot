package jot

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// testJot returns a Jot rooted in a temporary directory, isolated from the
// developer's real config.
func testJot(t *testing.T) *Jot {
	t.Helper()
	dir := t.TempDir()
	t.Setenv(EnvConfig, filepath.Join(dir, "config.json"))
	return New(Config{Root: filepath.Join(dir, "jot"), DefaultJournal: "default"})
}

func TestDefaultConfig_Env(t *testing.T) {
	t.Setenv(EnvConfig, filepath.Join(t.TempDir(), "missing.json"))
	t.Setenv(EnvRoot, "/tmp/jot-test-root")

	c, err := DefaultConfig()
	if err != nil {
		t.Fatal(err)
	}
	if c.Root != "/tmp/jot-test-root" {
		t.Errorf("expected root /tmp/jot-test-root, got %q", c.Root)
	}
}

func TestDefaultConfig_Home(t *testing.T) {
	t.Setenv(EnvConfig, filepath.Join(t.TempDir(), "missing.json"))
	t.Setenv(EnvRoot, "")

	c, err := DefaultConfig()
	if err != nil {
		t.Fatal(err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	if c.Root != filepath.Join(home, ".jot") {
		t.Errorf("expected default root in home, got %q", c.Root)
	}
	if c.DefaultJournal != DefaultJournalName {
		t.Errorf("expected default journal %q, got %q", DefaultJournalName, c.DefaultJournal)
	}
}

func TestConfigFile_RoundTripAndPrecedence(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvConfig, filepath.Join(dir, "jot", "config.json"))
	t.Setenv(EnvRoot, "")
	t.Setenv(EnvJournal, "")
	t.Setenv(EnvEditor, "")

	want := Config{Root: filepath.Join(dir, "journals"), DefaultJournal: "daily", Editor: "nano"}
	path, err := SaveConfig(want)
	if err != nil {
		t.Fatal(err)
	}
	if path != os.Getenv(EnvConfig) {
		t.Fatalf("unexpected config path %q", path)
	}

	loaded, err := LoadConfigFile()
	if err != nil {
		t.Fatal(err)
	}
	if loaded != want {
		t.Fatalf("got %+v, want %+v", loaded, want)
	}

	// The config file supplies defaults; environment variables still win.
	t.Setenv(EnvRoot, "/env/root")
	t.Setenv(EnvJournal, "work")
	got, err := DefaultConfig()
	if err != nil {
		t.Fatal(err)
	}
	if got.Root != "/env/root" || got.DefaultJournal != "work" {
		t.Fatalf("env did not override the config file: %+v", got)
	}
	if got.Editor != "nano" {
		t.Fatalf("expected editor from config file, got %q", got.Editor)
	}
}

func TestConfigFile_Malformed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvConfig, path)

	if _, err := LoadConfigFile(); err == nil {
		t.Fatal("expected an error for malformed config")
	}
	var cfgErr *ConfigError
	if _, err := LoadConfigFile(); !errors.As(err, &cfgErr) {
		t.Fatalf("expected *ConfigError, got %T", err)
	}
}

func TestFormatEntry(t *testing.T) {
	got := FormatEntry("12:00:00", "hello world")
	want := "## 12:00:00\n\nhello world\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRender_Headers(t *testing.T) {
	md := "# Title\n\nsome text\n"
	got := Render(md)
	if !strings.Contains(got, "→ # Title") {
		t.Errorf("expected rendered header, got %q", got)
	}
	if !strings.Contains(got, "  some text") {
		t.Errorf("expected indented body, got %q", got)
	}
}

func TestRender_Lists(t *testing.T) {
	md := "- item one\n* item two\n"
	got := Render(md)
	if !strings.Contains(got, "  • item one") {
		t.Errorf("got %q", got)
	}
	if !strings.Contains(got, "  • item two") {
		t.Errorf("got %q", got)
	}
}

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", "default"},
		{"  ", "default"},
		{"work", "work"},
		{"work stuff!", "work-stuff"},
		{"__foo--bar__", "__foo-bar__"},
		{"123", "123"},
		{"a b c", "a-b-c"},
		{"../../etc/passwd", "etc-passwd"},
	}
	for _, tt := range tests {
		got := SanitizeName(tt.in)
		if got != tt.want {
			t.Errorf("SanitizeName(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestParseDay(t *testing.T) {
	now := time.Now()
	tests := []struct {
		in   string
		want string
	}{
		{"", now.Format("2006-01-02")},
		{"today", now.Format("2006-01-02")},
		{"Yesterday", now.AddDate(0, 0, -1).Format("2006-01-02")},
		{"tomorrow", now.AddDate(0, 0, 1).Format("2006-01-02")},
		{"2026-01-02", "2026-01-02"},
	}
	for _, tt := range tests {
		got, err := ParseDay(tt.in)
		if err != nil {
			t.Fatalf("ParseDay(%q): %v", tt.in, err)
		}
		if got != tt.want {
			t.Errorf("ParseDay(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
	if _, err := ParseDay("last tuesday"); err == nil {
		t.Error("expected an error for an invalid day")
	}
}

func TestAddAndShow(t *testing.T) {
	j := testJot(t)
	if err := j.Add(Entry{Body: "first note"}); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(j.Root(), "default", today()+".md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), "## ") {
		t.Errorf("entry should start with a timestamp heading, got %q", data)
	}
	if !strings.Contains(string(data), "first note") {
		t.Errorf("entry body missing: %q", data)
	}

	contents, err := j.Show("default", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(contents) != 1 || !strings.Contains(contents[0], "first note") {
		t.Fatalf("unexpected show result: %q", contents)
	}
}

func TestAddSanitizesJournalAndHonoursDay(t *testing.T) {
	j := testJot(t)
	if err := j.Add(Entry{Name: "work stuff!", Day: "2026-01-02", Body: "planned"}); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(j.Root(), "work-stuff", "2026-01-02.md")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected %s: %v", path, err)
	}
	if _, err := os.Stat(filepath.Join(j.Root(), "work stuff!")); err == nil {
		t.Fatal("unsanitized journal directory was created")
	}

	contents, err := j.Show("work stuff!", "2026-01-02")
	if err != nil {
		t.Fatal(err)
	}
	if len(contents) != 1 || !strings.Contains(contents[0], "planned") {
		t.Fatalf("unexpected content: %q", contents)
	}
}

func TestAddRejectsEmptyBody(t *testing.T) {
	j := testJot(t)
	err := j.Add(Entry{Body: "  \n\n"})
	if !errors.Is(err, ErrEmptyBody) {
		t.Fatalf("expected ErrEmptyBody, got %v", err)
	}
}

func TestAddRejectsBackupsJournal(t *testing.T) {
	j := testJot(t)
	err := j.Add(Entry{Name: "backups", Body: "hello"})
	if err == nil || !strings.Contains(err.Error(), "reserved for backups") {
		t.Fatalf("expected reserved-name error, got %v", err)
	}
}

func TestShowMissingJournal(t *testing.T) {
	j := testJot(t)
	if _, err := j.Show("nope", "all"); !errors.Is(err, ErrNoJournal) {
		t.Fatalf("expected ErrNoJournal from Show(all), got %v", err)
	}
	if _, err := j.Show("nope", "2026-01-01"); !errors.Is(err, ErrNoJournal) {
		t.Fatalf("expected ErrNoJournal from Show(day), got %v", err)
	}
	if _, err := j.Days("nope"); !errors.Is(err, ErrNoJournal) {
		t.Fatalf("expected ErrNoJournal from Days, got %v", err)
	}
}

func TestShowEmptyDayIsNotAnError(t *testing.T) {
	j := testJot(t)
	if err := j.Init(); err != nil {
		t.Fatal(err)
	}
	contents, err := j.Show("default", "2026-01-01")
	if err != nil {
		t.Fatal(err)
	}
	if len(contents) != 0 {
		t.Fatalf("expected no content, got %q", contents)
	}
}

func TestShowAllDays(t *testing.T) {
	j := testJot(t)
	for _, day := range []string{"2026-01-01", "2026-01-03", "2026-01-02"} {
		if err := j.Add(Entry{Name: "work", Day: day, Body: "on " + day}); err != nil {
			t.Fatal(err)
		}
	}
	files, err := j.ShowDays("work")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 3 {
		t.Fatalf("expected 3 days, got %d", len(files))
	}
	if files[0].Day != "2026-01-01" || files[2].Day != "2026-01-03" {
		t.Fatalf("days not sorted: %+v", files)
	}
	all, err := j.Show("work", "all")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Fatalf("expected Show(all) to return 3 blobs, got %d", len(all))
	}
}

func TestListAndDaySummaries(t *testing.T) {
	j := testJot(t)
	for i := 0; i < 3; i++ {
		if err := j.Add(Entry{Name: "work", Body: "entry"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := j.Add(Entry{Name: "ideas", Day: "2026-02-02", Body: "one"}); err != nil {
		t.Fatal(err)
	}

	infos, err := j.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != 2 {
		t.Fatalf("expected 2 journals, got %d: %+v", len(infos), infos)
	}
	if infos[0].Name != "ideas" || infos[1].Name != "work" {
		t.Fatalf("journals not sorted: %+v", infos)
	}
	if infos[1].Entries != 3 || infos[1].Days != 1 {
		t.Fatalf("unexpected work summary: %+v", infos[1])
	}

	days, err := j.DaySummaries("work")
	if err != nil {
		t.Fatal(err)
	}
	if len(days) != 1 || days[0].Entries != 3 || days[0].Bytes == 0 {
		t.Fatalf("unexpected day summaries: %+v", days)
	}
}

func TestListEmptyRoot(t *testing.T) {
	j := testJot(t)
	infos, err := j.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != 0 {
		t.Fatalf("expected no journals, got %+v", infos)
	}
}

func TestSearch(t *testing.T) {
	j := testJot(t)
	if err := j.Add(Entry{Name: "work", Body: "Gym session at noon"}); err != nil {
		t.Fatal(err)
	}
	if err := j.Add(Entry{Name: "default", Body: "walked to the gym"}); err != nil {
		t.Fatal(err)
	}
	if err := j.Add(Entry{Name: "default", Body: "unrelated"}); err != nil {
		t.Fatal(err)
	}

	matches, err := j.Search("GYM")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d: %+v", len(matches), matches)
	}
	scoped, err := j.SearchJournal("work", "gym")
	if err != nil {
		t.Fatal(err)
	}
	if len(scoped) != 1 || scoped[0].Journal != "work" {
		t.Fatalf("unexpected scoped matches: %+v", scoped)
	}
	if scoped[0].Line != 3 {
		t.Fatalf("expected the body on line 3, got %d", scoped[0].Line)
	}

	if _, err := j.SearchJournal("work", "   "); err == nil {
		t.Fatal("expected an error for an empty query")
	}
}

func TestExport(t *testing.T) {
	j := testJot(t)
	if err := j.Add(Entry{Name: "work", Day: "2026-03-04", Body: "shipped"}); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	written, err := j.Export(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(written) != 1 {
		t.Fatalf("expected one file, got %v", written)
	}
	data, err := os.ReadFile(filepath.Join(dir, "work.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"# work", "## 2026-03-04", "shipped"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("export missing %q:\n%s", want, data)
		}
	}
}

func TestBackup(t *testing.T) {
	j := testJot(t)
	if err := j.Add(Entry{Name: "work", Body: "backup me"}); err != nil {
		t.Fatal(err)
	}

	first, err := j.Backup("")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(first, filepath.Join(j.Root(), backupDirName)) {
		t.Fatalf("backup should default into the root: %s", first)
	}
	names := tarNames(t, first)
	want := "work/" + today() + ".md"
	if !contains(names, want) {
		t.Fatalf("archive does not contain %s: %v", want, names)
	}

	// Sleep so the second archive gets a distinct name, then check that the
	// backups directory itself is never archived.
	time.Sleep(1100 * time.Millisecond)
	if _, err := j.Backup(""); err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		if strings.HasPrefix(name, backupDirName+"/") {
			t.Fatalf("backup archive contains a backup: %v", names)
		}
	}

	// A custom destination works too.
	custom := t.TempDir()
	path, err := j.Backup(custom)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(path) != custom {
		t.Fatalf("expected archive in %s, got %s", custom, path)
	}
}

func tarNames(t *testing.T, archive string) []string {
	t.Helper()
	f, err := os.Open(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	var names []string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return names
		}
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, hdr.Name)
	}
}

func contains(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}

func TestInstall(t *testing.T) {
	j := testJot(t)
	dir := t.TempDir()

	path, installed, err := j.Install(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !installed {
		t.Fatal("expected the binary to be installed")
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Size() == 0 {
		t.Fatal("installed binary is empty")
	}
	if st.Mode().Perm()&0o100 == 0 {
		t.Fatalf("installed binary is not executable: %v", st.Mode())
	}

	_, installedAgain, err := j.Install(dir)
	if err != nil {
		t.Fatal(err)
	}
	if installedAgain {
		t.Fatal("re-installing an identical binary should be a no-op")
	}
}

func TestUninstall(t *testing.T) {
	j := testJot(t)
	if err := j.Add(Entry{Body: "temporary"}); err != nil {
		t.Fatal(err)
	}
	if err := j.Uninstall(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(j.Root()); !os.IsNotExist(err) {
		t.Fatalf("root still exists: %v", err)
	}
	if err := j.Uninstall(); err == nil {
		t.Fatal("expected an error when nothing is left to delete")
	}
}

func TestUninstallRefusesHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory")
	}
	j := New(Config{Root: home})
	if err := j.Uninstall(); err == nil {
		t.Fatal("expected a refusal to delete the home directory")
	}
	if _, err := os.Stat(home); err != nil {
		t.Fatalf("home directory is gone: %v", err)
	}
}

func TestFindModuleRoot(t *testing.T) {
	root, err := FindModuleRoot(".")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatal(err)
	}
	if _, err := FindModuleRoot(t.TempDir()); err == nil {
		t.Fatal("expected an error for a directory that is not a checkout")
	}
}

func TestDoctor(t *testing.T) {
	j := testJot(t)
	if err := j.Init(); err != nil {
		t.Fatal(err)
	}
	checks := j.Doctor()
	if len(checks) == 0 {
		t.Fatal("expected some checks")
	}
	for _, c := range checks {
		switch c.Status {
		case "ok", "warn", "fail":
		default:
			t.Errorf("unexpected status %q for %q", c.Status, c.Name)
		}
		if c.Name == "root" && c.Status != "ok" {
			t.Errorf("root should be ok: %+v", c)
		}
	}
}

func TestSetupNonInteractive(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvConfig, filepath.Join(dir, "config.json"))
	no := false
	result, err := Setup(strings.NewReader(""), io.Discard, SetupOptions{
		Root:       filepath.Join(dir, "journals"),
		Journal:    "daily notes",
		Editor:     "true",
		Yes:        true,
		Install:    &no,
		InstallDir: filepath.Join(dir, "bin"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Config.DefaultJournal != "daily-notes" {
		t.Fatalf("journal not sanitized: %q", result.Config.DefaultJournal)
	}
	if _, err := os.Stat(filepath.Join(result.Config.Root, "daily-notes")); err != nil {
		t.Fatalf("root not created: %v", err)
	}
	loaded, err := LoadConfigFile()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Root != result.Config.Root {
		t.Fatalf("config not saved: %+v", loaded)
	}
}

// lockedBuffer is a concurrency-safe writer for the watch test.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestWatch(t *testing.T) {
	j := testJot(t)
	if err := j.Init(); err != nil {
		t.Fatal(err)
	}

	out := &lockedBuffer{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- j.Watch(ctx, "default", out, 10*time.Millisecond)
	}()

	if err := j.Add(Entry{Body: "live note"}); err != nil {
		cancel()
		t.Fatal(err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for !strings.Contains(out.String(), "live note") {
		if time.Now().After(deadline) {
			cancel()
			t.Fatalf("watch never printed the entry: %q", out.String())
		}
		time.Sleep(10 * time.Millisecond)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("watch did not stop after cancellation")
	}
}

func TestWatchMissingJournal(t *testing.T) {
	j := testJot(t)
	err := j.Watch(context.Background(), "ghost", io.Discard, time.Millisecond)
	if !errors.Is(err, ErrNoJournal) {
		t.Fatalf("expected ErrNoJournal, got %v", err)
	}
}

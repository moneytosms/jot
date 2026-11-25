package tui

import (
	"testing"
	"time"

	"github.com/moneytosms/jot"
)

// newTestJot builds a Jot rooted in a temp directory with one journal and one
// day file so journal existence checks are real.
func newTestJot(t *testing.T) *jot.Jot {
	t.Helper()
	root := t.TempDir()
	t.Setenv("JOT_ROOT", root)
	t.Setenv("JOT_CONFIG", root+"/config.json")
	cfg, err := jot.DefaultConfig()
	if err != nil {
		t.Fatalf("DefaultConfig: %v", err)
	}
	j := jot.New(cfg)
	if _, err := j.EnsureJournal("work"); err != nil {
		t.Fatalf("EnsureJournal: %v", err)
	}
	if err := j.Add(jot.Entry{Name: "work", Day: time.Now().Format("2006-01-02"), Body: "note"}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	return j
}

func TestResolveThemeNamePrecedence(t *testing.T) {
	base := jot.Config{TUI: jot.TUI{Theme: "config-theme"}}
	cases := []struct {
		name string
		opts Options
		env  string
		want string
	}{
		{"flag wins", Options{Cfg: base, Theme: "nord"}, "env-theme", "nord"},
		{"env beats config", Options{Cfg: base}, "env-theme", "env-theme"},
		{"config beats auto", Options{Cfg: base}, "", "config-theme"},
		{"default is auto", Options{}, "", "auto"},
		{"blank flag falls through", Options{Cfg: base, Theme: "  "}, "", "config-theme"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(jot.EnvTheme, tc.env)
			if got := resolveThemeName(tc.opts); got != tc.want {
				t.Fatalf("resolveThemeName = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestResolveJournalPrecedence(t *testing.T) {
	j := newTestJot(t)
	cfg, err := jot.DefaultConfig()
	if err != nil {
		t.Fatalf("DefaultConfig: %v", err)
	}
	cfg.TUI.Journal = "work"
	cfg.DefaultJournal = "work"

	if got := resolveJournal(j, Options{Cfg: cfg, Journal: "default"}); got != "default" {
		t.Fatalf("flag journal = %q, want default", got)
	}
	if got := resolveJournal(j, Options{Cfg: cfg}); got != "work" {
		t.Fatalf("saved journal = %q, want work", got)
	}

	// A remembered journal that no longer exists falls back to the default.
	cfg.TUI.Journal = "gone"
	if got := resolveJournal(jot.New(cfg), Options{Cfg: cfg}); got != "work" {
		t.Fatalf("missing saved journal = %q, want work", got)
	}
}

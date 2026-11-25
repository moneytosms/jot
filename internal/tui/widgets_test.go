package tui

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/moneytosms/jot"
)

func TestStreaks(t *testing.T) {
	today := time.Date(2026, 11, 25, 9, 0, 0, 0, time.Local)
	days := []string{"2026-11-01", "2026-11-02", "2026-11-03", "2026-11-04", "2026-11-23", "2026-11-24"}
	if cur, best := streaks(days, today); cur != 2 || best != 4 {
		t.Fatalf("streaks = %d, %d; want 2, 4 (yesterday keeps the run alive)", cur, best)
	}
	if cur, _ := streaks(days[:4], today); cur != 0 {
		t.Fatalf("broken streak = %d, want 0", cur)
	}
}

func TestWidgetsShowAndToggle(t *testing.T) {
	j, m := newTestModel(t)
	if err := j.Add(jot.Entry{Name: "default", Body: "- [ ] water plants"}); err != nil {
		t.Fatal(err)
	}
	m = send(m, tea.WindowSizeMsg{Width: 100, Height: 50})
	m = drain(m, m.fullLoad())
	plain := ansiRE.ReplaceAllString(m.View(), "")
	if !strings.Contains(plain, "OPEN TODOS 1") || !strings.Contains(plain, "water plants") || !strings.Contains(plain, "streak 1d") {
		t.Fatalf("widgets missing:\n%s", plain)
	}
	if strings.Contains(plain, "ON THIS DAY") {
		t.Fatal("on this day shown without memories")
	}

	// Clicking the todo jumps to its day.
	lay := m.layout()
	for i, l := range m.sideBlock(lay.bodyY + lay.paneH - lay.extraY) {
		if l.day != "" {
			m.selected = m.selected.AddDate(0, 0, -3)
			nm, cmd := m.Update(tea.MouseMsg(tea.MouseEvent{X: 3, Y: lay.extraY + i, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}))
			m = drain(nm.(model), cmd)
			if dayKey(m.selected) != l.day {
				t.Fatalf("clicked todo: selected %s, want %s", dayKey(m.selected), l.day)
			}
			break
		}
	}

	// Settings toggle hides the todos widget and persists it.
	m = press(m, "S")
	for i := 0; i < 7; i++ { // row 7 = Open todos
		m = press(m, "down")
	}
	m = pressRun(m, "enter")
	m = press(m, "esc")
	if strings.Contains(ansiRE.ReplaceAllString(m.View(), ""), "OPEN TODOS") {
		t.Fatal("todos widget still shown after hiding it")
	}
	if cfg, _ := jot.LoadConfigFile(); len(cfg.TUI.Hide) != 1 || cfg.TUI.Hide[0] != widgetTodos {
		t.Fatalf("hide = %v", cfg.TUI.Hide)
	}
}

func TestSettingsColorEncryptAndRestore(t *testing.T) {
	t.Setenv(jot.EnvKey, filepath.Join(t.TempDir(), "key.txt"))
	j, m := newTestModel(t)
	if err := j.Add(jot.Entry{Name: "work", Body: "secret plans"}); err != nil {
		t.Fatal(err)
	}
	m = drain(m, m.fullLoad())
	m = press(m, "S")
	m = press(m, "tab") // journals
	for i, jr := range m.journals {
		if jr.Name == "work" {
			m.settings.jcur = i
		}
	}
	m = pressRun(m, "c")
	if m.cfg.TUI.Colors["work"] != "green" {
		t.Fatalf("colors = %v", m.cfg.TUI.Colors)
	}
	m = press(m, "x")
	if j.IsEncrypted("work") {
		t.Fatal("encrypted without confirmation")
	}
	m = pressRun(m, "x")
	if !m.jot.IsEncrypted("work") {
		t.Fatalf("not encrypted: %q", m.settings.err)
	}
	if got, _ := m.jot.Show("work", ""); len(got) != 1 || !strings.Contains(got[0], "secret plans") {
		t.Fatalf("encrypted journal unreadable: %q", got)
	}

	if _, err := j.ArchiveJournal("work"); err != nil {
		t.Fatal(err)
	}
	if err := j.DeleteJournal("work"); err != nil {
		t.Fatal(err)
	}
	m = pressRun(m, "tab")
	m = pressRun(m, "tab") // archives
	if m.settings.tab != tabArchives || len(m.settings.archives) != 1 {
		t.Fatalf("archives tab: tab %d, %d archives", m.settings.tab, len(m.settings.archives))
	}
	m = pressRun(m, "enter")
	if !j.JournalExists("work") || !strings.Contains(m.toast, "restored work") {
		t.Fatalf("restore failed: %q / %q", m.settings.err, m.toast)
	}
}

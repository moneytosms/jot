package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/moneytosms/jot"
)

// newTestModel builds a model against a temp journal root and runs its
// initial load to completion, all headlessly.
func newTestModel(t *testing.T) (jot.Jot, model) {
	t.Helper()
	// Keep SaveTUIPreferences away from the user's real config file and make
	// toast timers instant so drained commands never sleep.
	t.Setenv(jot.EnvConfig, filepath.Join(t.TempDir(), "config.json"))
	oldDuration := toastDuration
	toastDuration = time.Millisecond
	t.Cleanup(func() { toastDuration = oldDuration })

	root := t.TempDir()
	cfg := jot.Config{Root: root, DefaultJournal: "default"}
	j := jot.New(cfg)
	m := newModel(j, cfg, "default", "default", Resolve("default"))
	m.width, m.height = 100, 30
	m.applySidebar()

	m2 := drain(m, m.Init())
	return *j, m2
}

// send feeds one message to the model.
func send(m model, msg tea.Msg) model {
	nm, _ := m.Update(msg)
	return nm.(model)
}

// drain runs a command (and everything nested in a tea.BatchMsg) to
// completion, feeding every produced message back into the model.
func drain(m model, cmd tea.Cmd) model {
	if cmd == nil {
		return m
	}
	switch v := cmd().(type) {
	case tea.BatchMsg:
		for _, c := range v {
			m = drain(m, c)
		}
		return m
	case nil:
		return m
	default:
		return send(m, v)
	}
}

// keyMsg builds a tea.KeyMsg like the real program would deliver.
func keyMsg(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

// press sends a key. State changes apply immediately; any returned command
// is not executed, so tests stay deterministic and never wait on tick timers.
func press(m model, key string) model {
	nm, _ := m.Update(keyMsg(key))
	return nm.(model)
}

// pressRun sends a key and then runs the returned command to completion,
// feeding every message (loads, creations, toasts) back into the model.
func pressRun(m model, key string) model {
	nm, cmd := m.Update(keyMsg(key))
	return drain(nm.(model), cmd)
}

func dayLabel(y int, mo time.Month, d int) string {
	return time.Date(y, mo, d, 0, 0, 0, 0, time.Local).Format("2006-01-02")
}

func TestMoveKeysShiftSelection(t *testing.T) {
	j, m := newTestModel(t)
	_ = j

	before := m.selected
	m = press(m, "j")
	if got, want := m.selected.Sub(before), 7*24*time.Hour; got != want {
		t.Errorf("after j: selected moved %v, want %v", got, want)
	}
	m = press(m, "k")
	if !m.selected.Equal(before) {
		t.Errorf("after k: selected = %v, want %v", m.selected, before)
	}
	m = press(m, "l")
	if !m.selected.Equal(before.AddDate(0, 0, 1)) {
		t.Errorf("after l: selected = %v, want %v", m.selected, before.AddDate(0, 0, 1))
	}
	m = press(m, "h")
	if !m.selected.Equal(before) {
		t.Errorf("after h: selected = %v, want %v", m.selected, before)
	}
	// Arrows behave the same.
	m2 := press(m, "right")
	if !m2.selected.Equal(before.AddDate(0, 0, 1)) {
		t.Errorf("after right: selected = %v", m2.selected)
	}
	m2 = press(m2, "down")
	if !m2.selected.Equal(before.AddDate(0, 0, 8)) {
		t.Errorf("after down: selected = %v", m2.selected)
	}
}

func TestMoveCrossesMonthBoundary(t *testing.T) {
	_, m := newTestModel(t)
	m.selected = time.Date(2025, 3, 31, 0, 0, 0, 0, time.Local)
	m.month = firstOfMonth(m.selected)

	m = press(m, "l")
	if got := dayKey(m.selected); got != "2025-04-01" {
		t.Errorf("selected = %s, want 2025-04-01", got)
	}
	if got := monthKey(m.month); got != "2025-04" {
		t.Errorf("month = %s, want 2025-04", got)
	}

	m = press(m, "h")
	if got := monthKey(m.month); got != "2025-03" {
		t.Errorf("month = %s, want 2025-03", got)
	}

	// Weeks jump months too.
	m.selected = time.Date(2025, 3, 30, 0, 0, 0, 0, time.Local)
	m.month = firstOfMonth(m.selected)
	m = press(m, "down")
	if got := monthKey(m.month); got != "2025-04" {
		t.Errorf("after week move: month = %s, want 2025-04", got)
	}
}

func TestMonthShiftClampsDay(t *testing.T) {
	_, m := newTestModel(t)
	m.selected = time.Date(2025, 3, 31, 0, 0, 0, 0, time.Local)
	m.month = firstOfMonth(m.selected)

	m = press(m, "H")
	if got := dayKey(m.selected); got != "2025-02-28" {
		t.Errorf("selected = %s, want 2025-02-28", got)
	}
	if got := monthKey(m.month); got != "2025-02" {
		t.Errorf("month = %s, want 2025-02", got)
	}
	m = press(m, "L")
	if got := dayKey(m.selected); got != "2025-03-28" {
		t.Errorf("selected = %s, want 2025-03-28", got)
	}
	// '<' and '>' do the same.
	m2 := press(m, "<")
	if got := monthKey(m2.month); got != "2025-02" {
		t.Errorf("'<' month = %s", got)
	}
	m2 = press(m2, ">")
	if got := monthKey(m2.month); got != "2025-03" {
		t.Errorf("'>' month = %s", got)
	}
}

func TestJumpEdgesUsesJournalDays(t *testing.T) {
	_, m := newTestModel(t)
	m.days = []string{"2025-01-05", "2025-03-20"}
	m.month = firstOfMonth(time.Date(2025, 3, 15, 0, 0, 0, 0, time.Local))
	m.selected = time.Date(2025, 3, 15, 0, 0, 0, 0, time.Local)

	m = press(m, "g")
	if got := dayKey(m.selected); got != "2025-01-05" {
		t.Errorf("after g: selected = %s, want 2025-01-05", got)
	}
	if got := monthKey(m.month); got != "2025-01" {
		t.Errorf("after g: month = %s, want 2025-01", got)
	}
	m = press(m, "G")
	if got := dayKey(m.selected); got != "2025-03-20" {
		t.Errorf("after G: selected = %s, want 2025-03-20", got)
	}

	// No days at all is a toast, not a crash.
	m.days = nil
	m2 := press(m, "g")
	if m2.toast == "" {
		t.Error("expected a toast when the journal is empty")
	}
}

func TestTabTogglesSidebar(t *testing.T) {
	_, m := newTestModel(t)
	if !m.showSidebar {
		t.Fatal("sidebar should start shown at 100 columns")
	}
	m = press(m, "tab")
	if m.showSidebar {
		t.Error("tab did not hide the sidebar")
	}
	m = press(m, "tab")
	if !m.showSidebar {
		t.Error("tab did not re-show the sidebar")
	}
	// Small terminals hide it regardless of the preference.
	m = send(m, tea.WindowSizeMsg{Width: 60, Height: 30})
	if m.showSidebar {
		t.Error("sidebar should hide under 70 columns")
	}
}

func TestQuickAddAppendsEntry(t *testing.T) {
	j, m := newTestModel(t)
	m.selected = time.Date(2025, 3, 15, 0, 0, 0, 0, time.Local)
	m.month = firstOfMonth(m.selected)

	// Open the input with a.
	m = press(m, "a")
	if m.mode != modeQuickAdd {
		t.Fatalf("mode = %v, want modeQuickAdd", m.mode)
	}
	if !m.input.isFocused() {
		t.Error("input is not focused after a")
	}

	// Empty text is ignored.
	m = press(m, "enter")
	if path := filepath.Join(j.Root(), "default", "2025-03-15.md"); fileExists(path) {
		t.Error("empty add created a file")
	}

	// Type and confirm.
	m = send(m, keyMsg("buy milk"))
	m = pressRun(m, "enter")
	if m.mode != modeQuickAdd {
		t.Errorf("mode = %v, want quick add to stay open", m.mode)
	}
	if m.toast == "" {
		t.Error("expected a toast after adding")
	}
	if m.input.value() != "" {
		t.Error("input was not cleared after adding")
	}
	data, err := os.ReadFile(filepath.Join(j.Root(), "default", "2025-03-15.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "buy milk") {
		t.Errorf("day file = %q, want it to contain the note", data)
	}
	if !strings.Contains(string(data), "## ") {
		t.Errorf("day file = %q, want a timestamped entry heading", data)
	}
	// The pane refreshed with the new content.
	if !strings.Contains(m.dayContent, "buy milk") {
		t.Errorf("dayContent = %q, want the new note", m.dayContent)
	}

	// Esc closes the input.
	m = press(m, "esc")
	if m.mode != modeBrowse {
		t.Errorf("mode = %v after esc, want modeBrowse", m.mode)
	}
	if m.input.isFocused() {
		t.Error("input still focused after esc")
	}
}

// TestQuickAddNeverRewrites checks the append-only promise: adding to a day
// that already has entries leaves the old bytes untouched.
func TestQuickAddNeverRewrites(t *testing.T) {
	j, m := newTestModel(t)
	path := filepath.Join(j.Root(), "default", "2025-03-15.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	original := "## 08:00:00\n\nmorning pages\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	m.selected = time.Date(2025, 3, 15, 0, 0, 0, 0, time.Local)
	m.month = firstOfMonth(m.selected)

	m = press(m, "a")
	m = send(m, keyMsg("second note"))
	m = pressRun(m, "enter")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.HasPrefix(s, original) {
		t.Errorf("file was rewritten:\n--- got ---\n%s", s)
	}
	if !strings.Contains(strings.TrimPrefix(s, original), "second note") {
		t.Errorf("appended entry missing:\n%s", s)
	}
}

func TestJournalSwitching(t *testing.T) {
	j, m := newTestModel(t)
	if err := os.MkdirAll(filepath.Join(j.Root(), "work"), 0o700); err != nil {
		t.Fatal(err)
	}
	// Simulate a loaded journal list.
	m.journals = []jot.JournalInfo{{Name: "default"}, {Name: "work"}}

	m = press(m, "J")
	if m.journal != "work" {
		t.Errorf("journal = %q after J, want work", m.journal)
	}
	m = press(m, "K")
	if m.journal != "default" {
		t.Errorf("journal = %q after K, want default", m.journal)
	}

	// Creating a journal with n switches to it.
	m = press(m, "n")
	if m.mode != modeNewJournal {
		t.Fatalf("mode = %v after n, want modeNewJournal", m.mode)
	}
	m = send(m, keyMsg("side"))
	m = pressRun(m, "enter")
	if m.mode != modeBrowse {
		t.Errorf("mode = %v after creating, want modeBrowse", m.mode)
	}
	if m.journal != "side" {
		t.Errorf("journal = %q, want side", m.journal)
	}
	if st, err := os.Stat(filepath.Join(j.Root(), "side")); err != nil || !st.IsDir() {
		t.Error("journal directory side was not created")
	}
}

func TestSidebarClickSwitchesJournal(t *testing.T) {
	j, m := newTestModel(t)
	if err := os.MkdirAll(filepath.Join(j.Root(), "work"), 0o700); err != nil {
		t.Fatal(err)
	}
	m.journals = []jot.JournalInfo{{Name: "default"}, {Name: "work"}}
	m = send(m, tea.WindowSizeMsg{Width: 100, Height: 30})

	lay := m.layout()
	// Click the second sidebar row (work), inside the row text.
	nm, cmd := m.Update(tea.MouseMsg(tea.MouseEvent{
		X: lay.sideX + 3, Y: lay.sideY + 1,
		Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
	}))
	m2 := drain(nm.(model), cmd)
	if m2.journal != "work" {
		t.Errorf("journal = %q after sidebar click, want work", m2.journal)
	}
}

func TestCalendarClickSelectsDay(t *testing.T) {
	_, m := newTestModel(t)
	lay := m.layout()
	weeks := m.grid()

	// Pick day 15 of the displayed month and compute its cell position.
	var row, col int
	target := time.Time{}
	for w, week := range weeks {
		for d, cell := range week.Days {
			if cell.Day == 15 {
				row, col, target = w, d, cell.Date
			}
		}
	}
	x, y := lay.calX+col*calCellW+1, lay.calY+row
	nm, cmd := m.Update(tea.MouseMsg(tea.MouseEvent{
		X: x, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
	}))
	m2 := drain(nm.(model), cmd)
	if !m2.selected.Equal(target) {
		t.Errorf("selected = %v after click at (%d,%d), want %v", m2.selected, x, y, target)
	}
}

func TestQuitKeys(t *testing.T) {
	_, m := newTestModel(t)
	for _, key := range []string{"q", "ctrl+c"} {
		nm, cmd := m.Update(keyMsg(key))
		if nm.(model).quitting != true {
			t.Errorf("quitting not set by %q", key)
		}
		if cmd == nil {
			t.Errorf("no quit command for %q", key)
		}
	}
}

func TestViewNeverPanicsOnTinyTerminals(t *testing.T) {
	_, m := newTestModel(t)
	for _, size := range [][2]int{{0, 0}, {1, 1}, {10, 3}, {30, 5}, {69, 10}, {70, 10}, {200, 50}} {
		m2 := send(m, tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		if view := m2.View(); strings.TrimSpace(view) == "" && size[0] > 0 {
			t.Errorf("empty view at %dx%d", size[0], size[1])
		}
	}
}

func TestLineInputEditing(t *testing.T) {
	var in lineInput
	in.focus()
	in.update(keyMsg("a"))
	in.update(keyMsg("b"))
	in.update(keyMsg("c"))
	if got := in.value(); got != "abc" {
		t.Fatalf("value = %q, want abc", got)
	}
	// Backspace removes before the cursor.
	if !in.update(tea.KeyMsg{Type: tea.KeyBackspace}) {
		t.Error("backspace was ignored")
	}
	if got := in.value(); got != "ab" {
		t.Errorf("value after backspace = %q", got)
	}
	// Cursor moves and mid-string insert.
	if !in.update(tea.KeyMsg{Type: tea.KeyLeft}) {
		t.Error("left was ignored")
	}
	in.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("X")})
	if got := in.value(); got != "aXb" {
		t.Errorf("value = %q, want aXb", got)
	}
	if !in.update(tea.KeyMsg{Type: tea.KeyEnd}) {
		t.Error("end was ignored")
	}
	in.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	if got := in.value(); got != "aXbd" {
		t.Errorf("value = %q, want aXbd", got)
	}
	// Empty input renders the placeholder; reset first.
	in.reset()
	in.placeholder = "say something"
	if v := in.view(); !strings.Contains(v, "say something") {
		t.Errorf("empty view = %q, want placeholder", v)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
func TestJournalRenameAndDelete(t *testing.T) {
	j, m := newTestModel(t)
	workDir := filepath.Join(j.Root(), "work")
	if err := os.MkdirAll(workDir, 0o700); err != nil {
		t.Fatal(err)
	}
	dayFile := filepath.Join(workDir, "2026-01-02.md")
	if err := os.WriteFile(dayFile, []byte("## 09:00:00\n\nkeep me\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	m.journals = []jot.JournalInfo{{Name: "default"}, {Name: "work"}}

	// Switch to work, then rename it.
	m = press(m, "J")
	if m.journal != "work" {
		t.Fatalf("journal = %q, want work", m.journal)
	}
	m = press(m, "R")
	if m.mode != modeRenameJournal {
		t.Fatalf("mode = %v after R, want modeRenameJournal", m.mode)
	}
	for _, r := range "deep-work" {
		m = press(m, string(r))
	}
	m = pressRun(m, "enter")
	if m.mode != modeBrowse {
		t.Fatalf("mode = %v after rename, want browse", m.mode)
	}
	if m.journal != "deep-work" {
		t.Fatalf("journal = %q after rename, want deep-work", m.journal)
	}
	if st, err := os.Stat(filepath.Join(j.Root(), "deep-work")); err != nil || !st.IsDir() {
		t.Fatalf("renamed directory missing: %v", err)
	}

	// A wrong confirmation name must not delete anything.
	m = press(m, "D")
	if m.mode != modeDeleteJournal {
		t.Fatalf("mode = %v after D, want modeDeleteJournal", m.mode)
	}
	m = press(m, "x")
	m = pressRun(m, "enter")
	if m.mode != modeDeleteJournal {
		t.Fatalf("delete closed without the exact name, mode = %v", m.mode)
	}
	if _, err := os.Stat(filepath.Join(j.Root(), "deep-work")); err != nil {
		t.Fatalf("journal deleted without confirmation: %v", err)
	}

	// The exact name deletes and archives.
	m = press(m, "esc")
	m = press(m, "D")
	for _, r := range "deep-work" {
		m = press(m, string(r))
	}
	m = pressRun(m, "enter")
	if m.mode != modeBrowse {
		t.Fatalf("mode = %v after delete, want browse", m.mode)
	}
	if _, err := os.Stat(filepath.Join(j.Root(), "deep-work")); !os.IsNotExist(err) {
		t.Fatalf("journal directory still present: %v", err)
	}
	backups, err := os.ReadDir(filepath.Join(j.Root(), "backups"))
	if err != nil || len(backups) == 0 {
		t.Fatalf("expected an archive in backups: %v", err)
	}
	if m.journal != "default" {
		t.Fatalf("journal = %q after deleting the open journal, want default", m.journal)
	}
}

func TestJournalDeleteProtectedDefault(t *testing.T) {
	_, m := newTestModel(t)
	m = press(m, "D")
	if m.mode == modeDeleteJournal {
		t.Fatal("the default journal opened the delete prompt")
	}
	if m.toast == "" {
		t.Fatal("expected a protection toast")
	}
}

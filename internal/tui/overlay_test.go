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

// ctrlKey builds a control-key message; keyMsg only handles runes and a few
// named keys.
func ctrlKey(t tea.KeyType) tea.KeyMsg { return tea.KeyMsg{Type: t} }

func TestOverlayOpenCloseAndCtrlC(t *testing.T) {
	_, base := newTestModel(t)
	cases := []struct {
		name string
		key  tea.KeyMsg
		want mode
	}{
		{"search", keyMsg("/"), modeSearch},
		{"palette", ctrlKey(tea.KeyCtrlP), modePalette},
		{"todos", keyMsg("T"), modeTodos},
		{"reminder", keyMsg("r"), modeReminder},
		{"settings", keyMsg("S"), modeSettings},
		{"help", keyMsg("?"), modeHelp},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := send(base, tc.key)
			if m.mode != tc.want {
				t.Fatalf("open mode = %v, want %v", m.mode, tc.want)
			}
			m = press(m, "esc")
			if m.mode != modeBrowse {
				t.Fatalf("esc mode = %v, want browse", m.mode)
			}
			// Opening another overlay from browse replaces the mode cleanly.
			m2 := send(m, keyMsg("?"))
			if m2.mode != modeHelp {
				t.Fatalf("after second overlay mode = %v, want modeHelp", m2.mode)
			}

			// ctrl+c must quit from every overlay.
			m = send(base, tc.key)
			nm, cmd := m.Update(keyMsg("ctrl+c"))
			m = nm.(model)
			if !m.quitting || cmd == nil {
				t.Fatalf("ctrl+c did not quit (quitting=%v cmd=%v)", m.quitting, cmd)
			}
			if _, ok := cmd().(tea.QuitMsg); !ok {
				t.Fatalf("ctrl+c cmd = %T, want tea.QuitMsg", cmd())
			}
		})
	}
}

func TestReminderKeyReplacesRefresh(t *testing.T) {
	_, m := newTestModel(t)

	m = press(m, "r")
	if m.mode != modeReminder {
		t.Fatalf("r mode = %v, want reminder", m.mode)
	}
	m = press(m, "esc")
	if m.mode != modeBrowse {
		t.Fatalf("esc mode = %v, want browse", m.mode)
	}

	nm, cmd := m.Update(keyMsg("ctrl+r"))
	m = nm.(model)
	if m.mode != modeBrowse {
		t.Fatalf("ctrl+r mode = %v, want browse", m.mode)
	}
	if cmd == nil {
		t.Fatal("ctrl+r returned no refresh command")
	}
	m = drain(m, cmd)
	if m.lastErr != nil {
		t.Fatalf("ctrl+r refresh error: %v", m.lastErr)
	}
}

func TestReminderPrefill(t *testing.T) {
	_, m := newTestModel(t)
	m.dayEntries = []jot.ParsedEntry{{Body: "call the dentist\nbring insurance"}}
	m = press(m, "r")
	if got := m.reminder.message.value(); got != "call the dentist" {
		t.Fatalf("reminder prefill = %q", got)
	}
	m = press(m, "esc")

	// Opening r from the todo overlay uses the highlighted todo text.
	m = send(m, keyMsg("T"))
	m.todos.seq = 1
	m = send(m, todosResultMsg{seq: 1, items: []jot.TodoItem{
		{Journal: "default", Day: "2025-01-02", Text: "water the plants", Done: false},
	}})
	m = press(m, "r")
	if m.mode != modeReminder {
		t.Fatalf("todo r mode = %v, want reminder", m.mode)
	}
	if got := m.reminder.message.value(); got != "water the plants" {
		t.Fatalf("todo reminder prefill = %q", got)
	}
}

func TestParseReminderDuration(t *testing.T) {
	cases := []struct {
		in   string
		want time.Duration
		ok   bool
	}{
		{"20m", 20 * time.Minute, true},
		{"2h", 2 * time.Hour, true},
		{"90s", 90 * time.Second, true},
		{"1h30m", 90 * time.Minute, true},
		{"15", 15 * time.Minute, true},
		{" 45 ", 45 * time.Minute, true},
		{"", 0, false},
		{"0", 0, false},
		{"0m", 0, false},
		{"-5", 0, false},
		{"-5m", 0, false},
		{"abc", 0, false},
		{"20x", 0, false},
	}
	for _, tc := range cases {
		got, err := parseReminderDuration(tc.in)
		if tc.ok {
			if err != nil {
				t.Errorf("parseReminderDuration(%q) error: %v", tc.in, err)
				continue
			}
			if got != tc.want {
				t.Errorf("parseReminderDuration(%q) = %v, want %v", tc.in, got, tc.want)
			}
		} else if err == nil {
			t.Errorf("parseReminderDuration(%q) = %v, want error", tc.in, got)
		}
	}
}

func TestReminderInvalidDurationKeepsForm(t *testing.T) {
	_, m := newTestModel(t)
	m = press(m, "r")
	m.reminder.duration.setValue("nonsense")
	m = press(m, "enter")
	if m.mode != modeReminder {
		t.Fatalf("invalid duration mode = %v, want reminder", m.mode)
	}
	if m.reminder.err == "" {
		t.Fatal("invalid duration did not set an inline error")
	}
	if m.reminder.pending {
		t.Fatal("invalid duration started a reminder")
	}
}

func TestReminderDoneToast(t *testing.T) {
	_, m := newTestModel(t)
	m = press(m, "r")
	m = send(m, reminderDoneMsg{
		duration: 20 * time.Minute,
		at:       time.Now().Add(20 * time.Minute),
	})
	if m.mode != modeBrowse {
		t.Fatalf("successful reminder mode = %v, want browse", m.mode)
	}
	if !strings.Contains(m.toast, "reminder set for 20m") {
		t.Fatalf("toast = %q", m.toast)
	}
}

func TestSearchStaleResponseDropped(t *testing.T) {
	_, m := newTestModel(t)
	m = send(m, keyMsg("/"))
	m.search.seq = 4

	m = send(m, searchResultMsg{seq: 3, scopeAll: true, journal: m.journal, matches: []jot.EntryMatch{
		{Journal: "default", Day: "2025-01-01", Body: "stale"},
	}})
	if len(m.search.results) != 0 {
		t.Fatalf("stale response applied: %v", m.search.results)
	}

	m = send(m, searchResultMsg{seq: 4, scopeAll: true, journal: m.journal, matches: []jot.EntryMatch{
		{Journal: "default", Day: "2025-01-02", Body: "fresh"},
	}})
	if len(m.search.results) != 1 || m.search.results[0].Body != "fresh" {
		t.Fatalf("fresh response not applied: %v", m.search.results)
	}
}

func TestSearchScopeAndTyping(t *testing.T) {
	_, m := newTestModel(t)
	m = send(m, keyMsg("/"))
	if !m.search.scopeAll {
		t.Fatal("search should start on all journals")
	}
	// j/k must remain typeable in the query.
	m = press(m, "j")
	m = press(m, "k")
	if got := m.search.input.value(); got != "jk" {
		t.Fatalf("typed query = %q, want jk", got)
	}
	m = press(m, "tab")
	if m.search.scopeAll {
		t.Fatal("tab did not switch to the current journal")
	}
}

func TestSearchSelectionJumpsDay(t *testing.T) {
	_, m := newTestModel(t)
	m = send(m, keyMsg("/"))
	m.search.results = []jot.EntryMatch{
		{Journal: "default", Day: "2025-02-03", Time: "09:30:00", Body: "first"},
		{Journal: "default", Day: "2025-02-04", Time: "10:00:00", Body: "second"},
	}
	// Results put the list in focus, so j moves the selection.
	m = press(m, "j")
	if m.search.cursor != 1 {
		t.Fatalf("cursor = %d, want 1", m.search.cursor)
	}
	m = pressRun(m, "enter")
	if m.mode != modeBrowse {
		t.Fatalf("mode = %v, want browse", m.mode)
	}
	if got := dayKey(m.selected); got != "2025-02-04" {
		t.Fatalf("selected day = %s, want 2025-02-04", got)
	}
}

func TestPaletteFilterAndActions(t *testing.T) {
	_, m := newTestModel(t)
	m.selected = time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local)

	m = send(m, ctrlKey(tea.KeyCtrlP))
	if m.mode != modePalette {
		t.Fatalf("palette mode = %v", m.mode)
	}
	for _, r := range "today" {
		m = press(m, string(r))
	}
	if len(m.palette.items) == 0 || m.palette.items[0].id != "today" {
		t.Fatalf("filtered items = %+v", m.palette.items)
	}
	m = pressRun(m, "enter")
	if m.mode != modeBrowse {
		t.Fatalf("after run mode = %v", m.mode)
	}
	if got := dayKey(m.selected); got != dayKey(m.today) {
		t.Fatalf("selected = %s, want today %s", got, dayKey(m.today))
	}

	// The quit action really quits.
	m = send(m, ctrlKey(tea.KeyCtrlP))
	for _, r := range "quit" {
		m = press(m, string(r))
	}
	m = press(m, "enter")
	if !m.quitting {
		t.Fatal("quit action did not set quitting")
	}
}

func TestPaletteJumpPrompt(t *testing.T) {
	_, m := newTestModel(t)
	m = send(m, ctrlKey(tea.KeyCtrlP))
	// The palette starts in list focus; '/' returns to the filter so a query
	// may begin with j.
	m = press(m, "/")
	for _, r := range "jump" {
		m = press(m, string(r))
	}
	if got := m.palette.filter.value(); got != "jump" {
		t.Fatalf("palette filter = %q, want jump", got)
	}
	m = press(m, "enter")
	if m.palette.promptKind != "day" {
		t.Fatalf("promptKind = %q, want day", m.palette.promptKind)
	}
	for _, r := range "yesterday" {
		m = press(m, string(r))
	}
	m = pressRun(m, "enter")
	if m.mode != modeBrowse {
		t.Fatalf("mode = %v, want browse", m.mode)
	}
	want := startOfDay(time.Now()).AddDate(0, 0, -1)
	if !m.selected.Equal(want) {
		t.Fatalf("selected = %v, want %v", m.selected, want)
	}
}

func TestPaletteNewJournalPrompt(t *testing.T) {
	j, m := newTestModel(t)
	_ = j
	m = send(m, ctrlKey(tea.KeyCtrlP))
	for _, r := range "new journal" {
		m = press(m, string(r))
	}
	m = press(m, "enter")
	if m.palette.promptKind != "journal" {
		t.Fatalf("promptKind = %q, want journal", m.palette.promptKind)
	}
	for _, r := range "work" {
		m = press(m, string(r))
	}
	m = pressRun(m, "enter")
	if m.mode != modeBrowse {
		t.Fatalf("mode = %v, want browse", m.mode)
	}
	if m.journal != "work" {
		t.Fatalf("journal = %q, want work", m.journal)
	}
}

func TestTodoSortingAndSelection(t *testing.T) {
	_, m := newTestModel(t)
	m = send(m, keyMsg("T"))
	m.todos.seq = 1
	items := []jot.TodoItem{
		{Journal: "default", Day: "2025-01-03", Text: "done first", Done: true},
		{Journal: "default", Day: "2025-01-02", Text: "open one", Done: false},
		{Journal: "default", Day: "2025-01-01", Text: "done two", Done: true},
		{Journal: "default", Day: "2025-01-04", Text: "open two", Done: false},
	}
	m = send(m, todosResultMsg{seq: 1, items: items})
	if len(m.todos.items) != 4 {
		t.Fatalf("items = %d, want 4", len(m.todos.items))
	}
	if m.todos.items[0].Done || m.todos.items[1].Done {
		t.Fatalf("done items not last: %+v", m.todos.items)
	}
	if m.todos.items[2].Done == false || m.todos.items[3].Done == false {
		t.Fatalf("open items not first: %+v", m.todos.items)
	}
	m = press(m, "j")
	if m.todos.cursor != 1 {
		t.Fatalf("cursor = %d, want 1", m.todos.cursor)
	}
	m = press(m, "G")
	if m.todos.cursor != 3 {
		t.Fatalf("G cursor = %d, want 3", m.todos.cursor)
	}
}

func TestTodoStaleResponseDropped(t *testing.T) {
	_, m := newTestModel(t)
	m = send(m, keyMsg("T"))
	m.todos.seq = 2
	m = send(m, todosResultMsg{seq: 1, items: []jot.TodoItem{{Text: "stale"}}})
	if len(m.todos.items) != 0 {
		t.Fatalf("stale todo response applied: %+v", m.todos.items)
	}
	m = send(m, todosResultMsg{seq: 2, items: []jot.TodoItem{{Text: "fresh"}}})
	if len(m.todos.items) != 1 || m.todos.items[0].Text != "fresh" {
		t.Fatalf("fresh todo response not applied: %+v", m.todos.items)
	}
}

func TestThemeCyclePersists(t *testing.T) {
	_, m := newTestModel(t)
	m = press(m, "S")
	if m.mode != modeSettings {
		t.Fatalf("mode = %v, want settings", m.mode)
	}
	m = pressRun(m, "enter")
	if m.themeName != "light" {
		t.Fatalf("themeName = %q, want light", m.themeName)
	}
	cfg, err := jot.LoadConfigFile()
	if err != nil {
		t.Fatalf("LoadConfigFile: %v", err)
	}
	if cfg.TUI.Theme != "light" {
		t.Fatalf("saved theme = %q, want light", cfg.TUI.Theme)
	}
}

func TestSettingsSavesEditorAndJournal(t *testing.T) {
	_, m := newTestModel(t)
	path := jot.ConfigPath()
	seed := `{"root":"/tmp/jot-root","editor":"nano","tui":{"theme":"nord"}}`
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(seed), 0o600); err != nil {
		t.Fatal(err)
	}

	m = press(m, "S")
	m = press(m, "down") // editor row
	m.settings.editor.setValue("vim -u NONE")
	m = pressRun(m, "enter")

	cfg, err := jot.LoadConfigFile()
	if err != nil {
		t.Fatalf("LoadConfigFile: %v", err)
	}
	if cfg.Editor != "vim -u NONE" {
		t.Fatalf("editor = %q", cfg.Editor)
	}
	if cfg.Root != "/tmp/jot-root" || cfg.TUI.Theme != "nord" {
		t.Fatalf("unrelated settings changed: %+v", cfg)
	}

	m = press(m, "down") // journal row
	m.settings.journal.setValue("work")
	m = pressRun(m, "enter")

	cfg, err = jot.LoadConfigFile()
	if err != nil {
		t.Fatalf("LoadConfigFile: %v", err)
	}
	if cfg.DefaultJournal != "work" {
		t.Fatalf("default journal = %q, want work", cfg.DefaultJournal)
	}
	if cfg.Editor != "vim -u NONE" || cfg.Root != "/tmp/jot-root" {
		t.Fatalf("unrelated settings changed after journal save: %+v", cfg)
	}
}

func TestSettingsSaveFailureToasts(t *testing.T) {
	_, m := newTestModel(t)
	// Point the config path at a directory so writing the file fails.
	t.Setenv(jot.EnvConfig, t.TempDir())
	m = press(m, "S")
	m = press(m, "down")
	m.settings.editor.setValue("vim")
	m = pressRun(m, "enter")
	if m.quitting {
		t.Fatal("settings save failure should not quit")
	}
	if m.settings.err == "" {
		t.Fatal("settings save failure did not set an inline error")
	}
	if m.toast == "" {
		t.Fatal("settings save failure did not toast")
	}
}

func TestHelpTableCompleteness(t *testing.T) {
	wantGroups := []string{"navigation", "calendar", "journals", "notes", "overlays", "settings", "quit"}
	if len(helpSections) != len(wantGroups) {
		t.Fatalf("help sections = %d, want %d", len(helpSections), len(wantGroups))
	}
	for i, want := range wantGroups {
		if helpSections[i].title != want {
			t.Errorf("section %d = %q, want %q", i, helpSections[i].title, want)
		}
		if len(helpSections[i].entries) == 0 {
			t.Errorf("section %q has no entries", want)
		}
	}
	var all strings.Builder
	for _, section := range helpSections {
		for _, entry := range section.entries {
			all.WriteString(entry.keys)
			all.WriteByte(' ')
		}
	}
	text := all.String()
	for _, key := range []string{"h l", "j k", "H L", "a", "e", "n", "J K", "/", "ctrl+p", "T", "S", "?", "r", "ctrl+r", "ctrl+c", "q", "enter", "esc"} {
		if !strings.Contains(text, key) {
			t.Errorf("help table missing key %q", key)
		}
	}
}

func TestOverlayViewsDoNotPanic(t *testing.T) {
	_, base := newTestModel(t)
	openers := []tea.KeyMsg{
		keyMsg("/"), ctrlKey(tea.KeyCtrlP), keyMsg("T"),
		keyMsg("r"), keyMsg("S"), keyMsg("?"),
	}
	sizes := [][2]int{{1, 1}, {1, 8}, {5, 3}, {10, 4}, {20, 6}, {40, 10}, {100, 30}}
	for _, size := range sizes {
		for _, opener := range openers {
			m := send(base, opener)
			m.width, m.height = size[0], size[1]
			m.applySidebar()
			view := m.View()
			if view == "" {
				t.Fatalf("empty view at %dx%d mode %v", size[0], size[1], m.mode)
			}
			for _, line := range strings.Split(view, "\n") {
				if w := ansiWidth(line); w > size[0] {
					t.Fatalf("line too wide at %dx%d mode %v: %d > %d (%q)", size[0], size[1], m.mode, w, size[0], line)
				}
			}
		}
	}
}

func TestOverlayMouseSelects(t *testing.T) {
	_, m := newTestModel(t)
	m.selected = time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local)

	// Palette: click the first action row.
	m = send(m, ctrlKey(tea.KeyCtrlP))
	lay := m.layout()
	rect := m.paletteRect(lay)
	click := tea.MouseMsg{X: rect.x + 2, Y: rect.y + 4, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress}
	nm, _ := m.Update(click)
	m = nm.(model)
	if m.mode != modeBrowse {
		t.Fatalf("palette click mode = %v, want browse", m.mode)
	}
	if got := dayKey(m.selected); got != dayKey(m.today) {
		t.Fatalf("palette click selected = %s, want today", got)
	}

	// Search: click a result row.
	m = send(m, keyMsg("/"))
	m.search.results = []jot.EntryMatch{{Journal: "default", Day: "2025-03-04", Body: "hit"}}
	m.search.seq = 1
	lay = m.layout()
	click = tea.MouseMsg{X: 2, Y: 5, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress}
	nm, _ = m.Update(click)
	m = nm.(model)
	if m.mode != modeBrowse {
		t.Fatalf("search click mode = %v, want browse", m.mode)
	}
	if got := dayKey(m.selected); got != "2025-03-04" {
		t.Fatalf("search click selected = %s", got)
	}
}

func TestOverlayClickOutsideCloses(t *testing.T) {
	_, m := newTestModel(t)
	m = send(m, keyMsg("S"))
	click := tea.MouseMsg{X: 0, Y: 0, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress}
	nm, _ := m.Update(click)
	m = nm.(model)
	if m.mode != modeBrowse {
		t.Fatalf("outside click mode = %v, want browse", m.mode)
	}
}
func TestPaletteJournalRenameAndDeleteActions(t *testing.T) {
	_, m := newTestModel(t)
	m.journal = "work"
	m.journals = []jot.JournalInfo{{Name: "default"}, {Name: "work"}}

	ids := map[string]bool{}
	for _, a := range m.paletteActions() {
		ids[a.id] = true
	}
	if !ids["rename-journal"] || !ids["delete-journal"] {
		t.Fatalf("palette actions missing journal management: %v", ids)
	}

	nm, _ := m.runPaletteAction(paletteAction{id: "rename-journal", label: "Rename journal"})
	if nm.(model).mode != modeRenameJournal {
		t.Fatalf("rename action mode = %v, want modeRenameJournal", nm.(model).mode)
	}
	nm, _ = m.runPaletteAction(paletteAction{id: "delete-journal", label: "Delete journal"})
	if nm.(model).mode != modeDeleteJournal {
		t.Fatalf("delete action mode = %v, want modeDeleteJournal", nm.(model).mode)
	}
}

package tui

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/moneytosms/jot"
)

// mode is which screen the TUI is on. The root Update routes every key press
// through exactly one switch on mode.
type mode int

const (
	modeBrowse        mode = iota // calendar + day pane
	modeQuickAdd                  // one-line note input at the bottom
	modeNewJournal                // journal-name prompt
	modeRenameJournal             // rename the open journal
	modeDeleteJournal             // type the name to delete the open journal
	modeSearch                    // fuzzy entry search overlay
	modePalette                   // command palette overlay
	modeTodos                     // aggregated checkbox overlay
	modeHeatmap                   // year heatmap overlay
	modeReminder                  // reminder form
	modeSettings                  // settings panel
	modeHelp                      // keybinding cheatsheet
	modeOnboard                   // intro tour
)

// model is the root Bubble Tea model.
type model struct {
	onboard int // card shown by the intro tour
	jot     *jot.Jot
	cfg     jot.Config
	journal string

	// Loaded state, refreshed by loadCmd / dayCmd.
	journals   []jot.JournalInfo
	days       []string // day labels with a file in the current journal
	counts     map[string]jot.DayCount
	dayContent string
	dayEntries []jot.ParsedEntry
	widgets    widgetData

	// Display state.
	month    time.Time // first day of the displayed month
	selected time.Time // selected day
	today    time.Time
	scroll   int // entry-pane line offset
	sideTop  int // first visible row of the journal list
	width    int
	height   int

	// Sidebar and modes.
	showSidebar bool
	sidebarPref bool // what the user asked for; showSidebar also honors width
	mode        mode
	input       lineInput
	target      string // journal the rename/delete prompt acts on

	// Status.
	toast    string
	toastSeq int
	lastErr  error
	loading  bool
	quitting bool

	// Theme.
	theme     Theme
	themeName string

	// Best-effort scroll target after jumping from a search result.
	scrollToTime string
	scrollToBody string

	// Overlay state. Exactly one of these is active at a time, matching mode.
	search   searchState
	palette  paletteState
	todos    todoState
	heat     heatState
	reminder reminderState
	settings settingsState
	help     helpState
}

// newModel builds the root model. themeName is the requested theme name (used
// for persistence), theme the resolved palette.
func newModel(j *jot.Jot, cfg jot.Config, journal, themeName string, theme Theme) model {
	ti := newLineInput()
	ti.placeholder = "one-line note…"
	now := time.Now()
	return model{
		jot:         j,
		cfg:         cfg,
		journal:     journal,
		theme:       theme,
		themeName:   themeName,
		month:       firstOfMonth(now),
		selected:    startOfDay(now),
		today:       startOfDay(now),
		counts:      map[string]jot.DayCount{},
		width:       80,
		height:      24,
		sidebarPref: true,
		showSidebar: true,
		mode:        modeBrowse,
		input:       ti,
		loading:     true,
	}
}

// Init issues the first data load.
func (m model) Init() tea.Cmd {
	return m.fullLoad()
}

// Messages.
type (
	// refreshMsg carries a full reload of the journals list, the displayed
	// month and the selected day. journal and month echo the request so stale
	// responses are dropped.
	refreshMsg struct {
		journal  string
		month    string
		day      string
		journals []jot.JournalInfo
		days     []string
		counts   map[string]jot.DayCount
		content  string
		widgets  widgetData
		err      error
	}
	// dayMsg carries one day's content.
	dayMsg struct {
		journal string
		day     string
		content string
		err     error
	}
	// addDoneMsg reports a successful quick add.
	addDoneMsg struct{}
	// journalCreatedMsg reports the result of creating a new journal.
	journalCreatedMsg struct {
		name string
		err  error
	}
	// journalOpMsg reports the result of renaming or deleting a journal.
	journalOpMsg struct {
		kind    string // "rename", "delete" or "archive"
		from    string
		to      string
		archive string
		err     error
	}
	// toastExpireMsg hides the toast with the matching sequence number.
	toastExpireMsg int
	// toastMsg shows a toast from a command.
	toastMsg string
	// errMsg reports a command failure.
	errMsg struct{ err error }
)

// Commands. All disk work happens here, never in View.

// loadCmd reloads journals, the month's day counts and the selected day.
func loadCmd(j *jot.Jot, journal string, month, day time.Time) tea.Cmd {
	return func() tea.Msg { return load(j, journal, month, day) }
}

// load performs the actual reload.
func load(j *jot.Jot, journal string, month, day time.Time) refreshMsg {
	msg := refreshMsg{journal: journal, month: monthKey(month), day: dayKey(day)}
	if infos, err := j.List(); err != nil {
		msg.err = err
	} else {
		msg.journals = infos
	}
	if days, err := j.Days(journal); err == nil {
		msg.days = days
	}
	msg.widgets = loadWidgets(j, journal, msg.days, time.Now())
	if counts, err := j.MonthDays(journal, month.Year(), month.Month()); err == nil {
		msg.counts = make(map[string]jot.DayCount, len(counts))
		for _, c := range counts {
			msg.counts[c.Day] = c
		}
	}
	// A missing journal or day file is a normal empty state, not an error.
	if content, err := j.Show(journal, dayKey(day)); err == nil && len(content) > 0 {
		msg.content = content[0]
	}
	return msg
}

// dayCmd reloads only the selected day's content.
func dayCmd(j *jot.Jot, journal string, day time.Time) tea.Cmd {
	d := dayKey(day)
	journalName := journal
	return func() tea.Msg {
		content, err := j.Show(journalName, d)
		m := dayMsg{journal: journalName, day: d}
		if err == nil && len(content) > 0 {
			m.content = content[0]
		}
		return m
	}
}

// addCmd appends one entry and then reports success. The caller issues the
// follow-up load in parallel.
func addCmd(j *jot.Jot, journal, day, text string) tea.Cmd {
	return func() tea.Msg {
		if err := j.Add(jot.Entry{Name: journal, Day: day, Body: text}); err != nil {
			return errMsg{err}
		}
		return addDoneMsg{}
	}
}

// createJournalCmd creates a journal directory (never deletes anything) and
// reports the result; Update does the switch and persistence.
func createJournalCmd(j *jot.Jot, name string) tea.Cmd {
	return func() tea.Msg {
		_, err := j.EnsureJournal(name)
		return journalCreatedMsg{name: name, err: err}
	}
}

// renameJournalCmd renames a journal and reports the outcome; Update switches
// the view and persists the journal preference.
func renameJournalCmd(j *jot.Jot, from, to string) tea.Cmd {
	return func() tea.Msg {
		err := j.RenameJournal(from, to)
		return journalOpMsg{kind: "rename", from: from, to: to, err: err}
	}
}

// deleteJournalCmd archives a journal to <root>/backups and then removes it.
// The archive is created first, so a failed delete still leaves a backup.
func deleteJournalCmd(j *jot.Jot, name string) tea.Cmd {
	return func() tea.Msg {
		archive, err := j.ArchiveJournal(name)
		if err != nil {
			return journalOpMsg{kind: "delete", from: name, err: err}
		}
		if err := j.DeleteJournal(name); err != nil {
			return journalOpMsg{kind: "delete", from: name, archive: archive, err: err}
		}
		return journalOpMsg{kind: "delete", from: name, archive: archive}
	}
}

// editorCmd opens the selected day in the editor, suspending the TUI with
// tea.ExecProcess, and reloads the day afterwards.
func editorCmd(j *jot.Jot, editor, journal string, day time.Time) tea.Cmd {
	label := dayKey(day)
	return func() tea.Msg {
		if _, err := j.EnsureJournal(journal); err != nil {
			return errMsg{err}
		}
		// EditFile creates (and seeds) the day file first: some editors refuse
		// a path that does not exist; encrypted days open as a private temp file.
		path, finish, err := j.EditFile(journal, label)
		if err != nil {
			return errMsg{err}
		}
		c := shellEditor(editor, path)
		return tea.ExecProcess(c, func(err error) tea.Msg {
			finishErr := finish()
			if err != nil {
				return errMsg{fmt.Errorf("editor: %w", err)}
			}
			if finishErr != nil {
				return errMsg{finishErr}
			}
			return load(j, journal, firstOfMonth(day), day)
		})()
	}
}

// shellEditor builds the editor invocation: "sh -c <editor> '<path>'" so
// editor flags and wrapped commands work.
func shellEditor(editor, path string) *exec.Cmd {
	return exec.Command("sh", "-c", editor+" "+shQuote(path))
}

// shQuote single-quotes s for a POSIX sh command line. Only used on paths the
// TUI built itself.
func shQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// startOfDay truncates t to midnight in its own location.
func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// Update handles every message; disk work stays in commands.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.applySidebar()
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.MouseMsg:
		return m.handleMouse(msg)

	case refreshMsg:
		return m.applyRefresh(msg)

	case dayMsg:
		if msg.journal == m.journal && msg.day == dayKey(m.selected) {
			m.loading = false
			if msg.err != nil {
				m.lastErr = msg.err
				m.scrollToTime = ""
				m.scrollToBody = ""
			} else {
				m.dayContent = msg.content
				m.dayEntries = jot.ParseEntries(msg.content)
				m.applyScrollTarget()
			}
		}
		return m, nil

	case addDoneMsg:
		cmd := m.setToast("added ✓")
		return m, cmd

	case journalCreatedMsg:
		return m.applyJournalCreated(msg)

	case journalOpMsg:
		return m.applyJournalOp(msg)

	case searchTickMsg:
		if m.mode == modeSearch && msg.seq == m.search.seq {
			return m, m.searchLoadCmd()
		}
		return m, nil

	case searchResultMsg:
		return m.applySearchResult(msg)

	case searchTagsMsg:
		m.search.tags = msg.tags
		return m, nil

	case todosResultMsg:
		return m.applyTodosResult(msg)
	case heatResultMsg:
		return m.applyHeatResult(msg)

	case reminderDoneMsg:
		return m.applyReminderDone(msg)

	case settingsSavedMsg:
		return m.applySettingsSaved(msg)

	case rootSavedMsg:
		return m.applyRootSaved(msg)

	case archivesMsg, restoredMsg, encryptedMsg:
		return m.applySettingsAsync(msg)

	case toastMsg:
		return m, m.setToast(string(msg))

	case prefsSavedMsg:
		return m.applyPrefsSaved(msg)

	case backupDoneMsg:
		m.settings.warn = ""
		if msg.err != nil {
			m.lastErr = msg.err
			m.settings.err = msg.err.Error()
			return m, nil
		}
		return m, m.setToast(backupLabel(msg.path))

	case exportDoneMsg:
		if msg.err != nil {
			m.lastErr = msg.err
			return m, nil
		}
		return m, m.setToast(exportLabel(msg))

	case toastExpireMsg:
		if int(msg) == m.toastSeq {
			m.toast = ""
		}
		return m, nil

	case errMsg:
		m.loading = false
		m.lastErr = msg.err
		return m, nil
	}
	return m, nil
}

// handleKey is the single place that routes key presses by mode. ctrl+c is
// handled first so no overlay can swallow the quit key.
func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		m.quitting = true
		return m, tea.Quit
	}
	switch m.mode {
	case modeQuickAdd:
		return m.keyQuickAdd(msg)
	case modeNewJournal:
		return m.keyNewJournal(msg)
	case modeRenameJournal:
		return m.keyRenameJournal(msg)
	case modeDeleteJournal:
		return m.keyDeleteJournal(msg)
	case modeSearch:
		return m.keySearch(msg)
	case modePalette:
		return m.keyPalette(msg)
	case modeTodos:
		return m.keyTodos(msg)
	case modeHeatmap:
		return m.keyHeat(msg)
	case modeReminder:
		return m.keyReminder(msg)
	case modeSettings:
		return m.keySettings(msg)
	case modeHelp:
		return m.keyHelp(msg)
	case modeOnboard:
		return m.keyOnboard(msg)
	default:
		return m.keyBrowse(msg)
	}
}

// keyBrowse handles keys while viewing the calendar and day pane.
func (m model) keyBrowse(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit

	case "up", "k":
		return m.move(-7)
	case "down", "j":
		return m.move(7)
	case "left", "h":
		return m.move(-1)
	case "right", "l":
		return m.move(1)
	case "H", "<":
		return m.shiftMonth(-1)
	case "L", ">":
		return m.shiftMonth(1)
	case "t":
		return m.jumpToday()
	case "g":
		return m.jumpEdge(true)
	case "G":
		return m.jumpEdge(false)
	case "tab":
		m.sidebarPref = !m.sidebarPref
		m.applySidebar()
		return m, nil
	case "J":
		return m.cycleJournal(1)
	case "K":
		return m.cycleJournal(-1)
	case "n":
		return m.startNewJournal()
	case "R":
		return m.startRenameJournal(m.journal)
	case "D":
		return m.startDeleteJournal(m.journal)
	case "a":
		return m.startQuickAdd()
	case "e":
		return m, m.editDay()
	case "/":
		return m.openSearch()
	case "ctrl+p":
		return m.openPalette()
	case "T":
		return m.openTodos()
	case "y":
		return m.openHeatmap()
	case "r":
		return m.openReminder(m.lastEntryPreview())
	case "ctrl+r":
		cmd := m.setToast("refreshed")
		return m, tea.Batch(cmd, m.fullLoad())
	case "S":
		return m.openSettings()
	case "?":
		return m.openHelp()
	}
	return m, nil
}

// keyRenameJournal feeds keys to the rename prompt; Enter renames, Esc closes.
func (m model) keyRenameJournal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "esc":
		m.mode = modeBrowse
		m.input.blur()
		m.input.reset()
		return m, nil
	case "enter":
		to := jot.SanitizeName(m.input.value())
		if to == "" {
			return m, m.setToast("type a journal name")
		}
		if to == m.target {
			return m, m.setToast("that is already the journal name")
		}
		m.input.blur()
		return m, renameJournalCmd(m.jot, m.target, to)
	}
	m.input.update(msg)
	return m, nil
}

// keyDeleteJournal requires the exact journal name before deleting.
func (m model) keyDeleteJournal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "esc":
		m.mode = modeBrowse
		m.input.blur()
		m.input.reset()
		return m, nil
	case "enter":
		if strings.TrimSpace(m.input.value()) != m.target {
			return m, m.setToast("type " + m.target + " exactly to confirm")
		}
		m.input.blur()
		return m, deleteJournalCmd(m.jot, m.target)
	}
	m.input.update(msg)
	return m, nil
}

// keyQuickAdd feeds keys to the note input; Enter appends, Esc closes.
func (m model) keyQuickAdd(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "esc":
		m.mode = modeBrowse
		m.input.blur()
		return m, nil
	case "enter":
		text := strings.TrimSpace(m.input.value())
		if text == "" {
			return m, nil // empty text is ignored
		}
		journal, day := m.journal, dayKey(m.selected)
		m.input.setValue("")
		return m, tea.Batch(
			addCmd(m.jot, journal, day, text),
			loadCmd(m.jot, journal, m.month, m.selected),
		)
	}
	m.input.update(msg)
	return m, nil
}

// keyNewJournal feeds keys to the journal-name prompt.
func (m model) keyNewJournal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "esc":
		m.mode = modeBrowse
		m.input.blur()
		m.input.reset()
		return m, nil
	case "enter":
		name := jot.SanitizeName(m.input.value())
		if name == "" || name == m.journal {
			cmd := m.setToast("pick a different name")
			return m, cmd
		}
		m.input.blur()
		m.input.reset()
		return m, createJournalCmd(m.jot, name)
	}
	m.input.update(msg)
	return m, nil
}

// Movement and navigation.

// move shifts the selection by delta days, following into other months.
func (m model) move(delta int) (tea.Model, tea.Cmd) {
	m.selected = m.selected.AddDate(0, 0, delta)
	m.scroll = 0
	if monthKey(m.selected) != monthKey(m.month) {
		m.month = firstOfMonth(m.selected)
		return m, m.fullLoad()
	}
	return m, m.dayLoad()
}

// shiftMonth changes the displayed month, keeping the selection on the same
// day of the month where possible.
func (m model) shiftMonth(delta int) (tea.Model, tea.Cmd) {
	m.month = m.month.AddDate(0, delta, 0)
	day := m.selected.Day()
	if last := daysInMonth(m.month.Year(), m.month.Month()); day > last {
		day = last
	}
	m.selected = time.Date(m.month.Year(), m.month.Month(), day, 0, 0, 0, 0, time.Local)
	m.scroll = 0
	return m, m.fullLoad()
}

// jumpToday selects today, moving months if needed.
func (m model) jumpToday() (tea.Model, tea.Cmd) {
	m.selected = m.today
	m.scroll = 0
	if monthKey(m.today) != monthKey(m.month) {
		m.month = firstOfMonth(m.today)
		return m, m.fullLoad()
	}
	return m, m.dayLoad()
}

// jumpEdge selects the first (or last) day that has a file in the current
// journal.
func (m model) jumpEdge(first bool) (tea.Model, tea.Cmd) {
	if len(m.days) == 0 {
		cmd := m.setToast("no entries yet")
		return m, cmd
	}
	label := m.days[0]
	if !first {
		label = m.days[len(m.days)-1]
	}
	t, err := time.ParseInLocation("2006-01-02", label, time.Local)
	if err != nil {
		cmd := m.setToast("bad date on disk: " + label)
		return m, cmd
	}
	m.selected = t
	m.scroll = 0
	if monthKey(t) != monthKey(m.month) {
		m.month = firstOfMonth(t)
		return m, m.fullLoad()
	}
	return m, m.dayLoad()
}

// cycleJournal switches to the next (or previous) journal, sorted by name.
func (m model) cycleJournal(step int) (tea.Model, tea.Cmd) {
	if len(m.journals) == 0 {
		cmd := m.setToast("no journals yet — press n to create one")
		return m, cmd
	}
	idx := 0
	for i, j := range m.journals {
		if j.Name == m.journal {
			idx = i
			break
		}
	}
	idx = (idx + step + len(m.journals)) % len(m.journals)
	return m.switchJournal(m.journals[idx].Name)
}

// switchJournal opens another journal and persists the preference.
func (m model) switchJournal(name string) (tea.Model, tea.Cmd) {
	if name == "" || name == m.journal {
		return m, nil
	}
	m.journal = name
	m.scroll = 0
	cmd := m.setToast("switched to " + name)
	return m, tea.Batch(cmd, m.fullLoad(), savePrefsCmd(m.themeName, name, "journal"))
}

// applyJournalCreated handles the result of the new-journal prompt.
func (m model) applyJournalCreated(msg journalCreatedMsg) (tea.Model, tea.Cmd) {
	m.mode = modeBrowse
	m.input.reset()
	if msg.err != nil {
		m.lastErr = msg.err
		return m, nil
	}
	m.journal = msg.name
	m.scroll = 0
	cmd := m.setToast("created " + msg.name)
	return m, tea.Batch(cmd, m.fullLoad(), savePrefsCmd(m.themeName, msg.name, "journal"))
}

// applyJournalOp handles the result of renaming or deleting a journal. The
// view falls back to the default journal when the open one disappears.
func (m model) applyJournalOp(msg journalOpMsg) (tea.Model, tea.Cmd) {
	if msg.kind == "archive" { // runs from settings, which stays open
		if msg.err != nil {
			m.settings.err = msg.err.Error()
			return m, nil
		}
		return m, m.setToast("archived " + msg.from + " to " + msg.archive)
	}
	m.mode = modeBrowse
	m.input.blur()
	m.input.reset()
	if msg.err != nil {
		m.lastErr = msg.err
		return m, nil
	}
	switch msg.kind {
	case "rename":
		if m.journal == msg.from {
			m.journal = msg.to
		}
		m.scroll = 0
		cmd := m.setToast("renamed " + msg.from + " to " + msg.to)
		return m, tea.Batch(cmd, m.fullLoad(), savePrefsCmd(m.themeName, m.journal, "journal"))
	case "delete":
		if m.journal == msg.from {
			m.journal = m.fallbackJournal(msg.from)
		}
		m.scroll = 0
		cmd := m.setToast("deleted " + msg.from + "; archived to " + filepath.Base(filepath.Dir(msg.archive)) + "/")
		return m, tea.Batch(cmd, m.fullLoad(), savePrefsCmd(m.themeName, m.journal, "journal"))
	}
	return m, nil
}

// fallbackJournal picks the journal to show after deleting one: the configured
// default when it still exists, else the first surviving journal, else default.
func (m model) fallbackJournal(deleted string) string {
	def := jot.SanitizeName(m.cfg.DefaultJournal)
	if def != deleted && m.jot.JournalExists(def) {
		return def
	}
	for _, j := range m.journals {
		if j.Name != deleted {
			return j.Name
		}
	}
	return def
}

// applyPrefsSaved merges a background preference save.
func (m model) applyPrefsSaved(msg prefsSavedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.lastErr = msg.err
		return m, m.setToast("preference not saved: " + msg.err.Error())
	}
	switch msg.kind {
	case "theme":
		return m, m.setToast("theme: " + msg.theme)
	case "journal":
		if msg.journal != m.journal {
			return m, nil // a newer switch already owns the preference
		}
		return m, nil
	}
	return m, nil
}

// Mode transitions.

// startRenameJournal opens the rename prompt for a journal.
func (m model) startRenameJournal(name string) (tea.Model, tea.Cmd) {
	if name == m.cfg.DefaultJournal {
		return m, m.setToast("the default journal is protected; change default_journal first")
	}
	m.closeOverlay()
	m.target = name
	m.mode = modeRenameJournal
	m.input.placeholder = "new name for " + name
	m.input.setValue("")
	m.input.focus()
	return m, nil
}

// startDeleteJournal opens the delete confirmation for a journal. The name
// has to be typed in full, mirroring the CLI confirmation.
func (m model) startDeleteJournal(name string) (tea.Model, tea.Cmd) {
	if name == m.cfg.DefaultJournal {
		return m, m.setToast("the default journal is protected; change default_journal first")
	}
	m.closeOverlay()
	m.target = name
	m.mode = modeDeleteJournal
	m.input.placeholder = name
	m.input.setValue("")
	m.input.focus()
	return m, nil
}

// startQuickAdd opens the one-line note input for the selected day.
func (m model) startQuickAdd() (tea.Model, tea.Cmd) {
	m.mode = modeQuickAdd
	m.input.placeholder = "note for " + dayKey(m.selected)
	m.input.setValue("")
	m.input.focus()
	return m, nil
}

// startNewJournal opens the journal-name prompt.
func (m model) startNewJournal() (tea.Model, tea.Cmd) {
	m.mode = modeNewJournal
	m.input.placeholder = "journal name"
	m.input.setValue("")
	m.input.focus()
	return m, nil
}

// editDay returns the command that opens the selected day in the editor.
func (m model) editDay() tea.Cmd {
	editor := strings.TrimSpace(m.cfg.Editor)
	if editor == "" {
		editor = jot.DetectEditor()
	}
	if editor == "" {
		m.lastErr = errors.New("no editor found: set JOT_EDITOR or $EDITOR")
		return nil
	}
	return editorCmd(m.jot, editor, m.journal, m.selected)
}

// Data application.

// applyRefresh merges a full reload, dropping responses that no longer match
// the displayed journal or month.
func (m model) applyRefresh(msg refreshMsg) (tea.Model, tea.Cmd) {
	if msg.journal != m.journal || msg.month != monthKey(m.month) {
		return m, nil // stale, a newer load is in flight
	}
	m.loading = false
	m.lastErr = msg.err
	if msg.err != nil {
		m.scrollToTime = ""
		m.scrollToBody = ""
	}
	m.journals = msg.journals
	m.revealJournal()
	m.days = msg.days
	m.widgets = msg.widgets
	if msg.counts == nil {
		msg.counts = map[string]jot.DayCount{}
	}
	m.counts = msg.counts
	if msg.day == dayKey(m.selected) {
		m.dayContent = msg.content
		m.dayEntries = jot.ParseEntries(msg.content)
		m.applyScrollTarget()
	}
	return m, nil
}

// Toasts.

// toastDuration is how long a toast stays visible. It is a variable so tests
// can shorten it.
var toastDuration = 3 * time.Second

// setToast shows a status message for a few seconds and returns the command
// that hides it. The sequence number keeps overlapping toasts from racing.
func (m *model) setToast(text string) tea.Cmd {
	m.toast = text
	m.toastSeq++
	seq := m.toastSeq
	return tea.Tick(toastDuration, func(time.Time) tea.Msg { return toastExpireMsg(seq) })
}

// Mouse.

// handleMouse maps wheel events to pane scrolling and clicks to calendar
// days, sidebar journals and the header. Overlay modes get their own handler
// so a click can select, activate or dismiss the panel.
func (m model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if msg.Action != tea.MouseActionPress {
		return m, nil
	}
	if isOverlayMode(m.mode) {
		return m.handleOverlayMouse(msg)
	}
	lay := m.layout()
	overList := m.showSidebar && msg.X < lay.leftW && msg.Y >= lay.sideY && msg.Y < lay.sideY+lay.sideRows
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		if overList {
			m.sideTop = clamp(m.sideTop-1, 0, max(len(m.journals)-lay.sideRows, 0))
			return m, nil
		}
		m.scroll = clamp(m.scroll-3, 0, 1<<20)
		return m, nil
	case tea.MouseButtonWheelDown:
		if overList {
			m.sideTop = clamp(m.sideTop+1, 0, max(len(m.journals)-lay.sideRows, 0))
			return m, nil
		}
		m.scroll = clamp(m.scroll+3, 0, 1<<20)
		return m, nil
	case tea.MouseButtonLeft:
		return m.handleClick(msg.X, msg.Y)
	}
	return m, nil
}

// handleOverlayMouse dispatches mouse events to the active overlay.
func (m model) handleOverlayMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case modeSearch:
		return m.handleSearchMouse(msg)
	case modePalette:
		return m.handlePaletteMouse(msg)
	case modeTodos:
		return m.handleTodosMouse(msg)
	case modeReminder:
		return m.handleReminderMouse(msg)
	case modeSettings:
		return m.handleSettingsMouse(msg)
	case modeHelp:
		return m.handleHelpMouse(msg)
	case modeOnboard:
		if msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress {
			return m.moveOnboard(1)
		}
	}
	return m, nil
}

// handleClick selects a clicked calendar day, switches to a clicked sidebar
// journal, or jumps to today when the header is clicked.
func (m model) handleClick(x, y int) (tea.Model, tea.Cmd) {
	lay := m.layout()
	if y == 0 {
		return m.jumpToday() // the header doubles as a "today" button
	}
	if lay.leftW == 0 {
		return m, nil // the left column is hidden
	}
	if y == lay.monthY && x >= lay.calX && x < lay.calX+calW {
		switch {
		case x <= lay.calX+3:
			return m.shiftMonth(-1)
		case x >= lay.calX+calW-4:
			return m.shiftMonth(1)
		}
		return m, nil
	}
	weeks := m.grid()
	if len(weeks) > 0 &&
		y >= lay.calY && y < lay.calY+len(weeks)*lay.calRowH &&
		x >= lay.calX && x < lay.calX+calW {
		cell := weeks[(y-lay.calY)/lay.calRowH].Days[(x-lay.calX)/calCellW]
		if cell.Day > 0 {
			m.selected = cell.Date
			m.scroll = 0
			return m, m.dayLoad()
		}
		return m, nil
	}
	_, journals := m.visibleJournals(lay)
	if m.showSidebar && len(journals) > 0 &&
		x >= lay.sideX && x < lay.sideX+sideW &&
		y >= lay.sideY && y < lay.sideY+len(journals) {
		return m.switchJournal(journals[y-lay.sideY].Name)
	}
	// Rows under the calendar: shortcuts act like their keys, todos and
	// memories jump to their day.
	if lay.extraY > 0 && y >= lay.extraY && x < lay.leftW {
		block := m.sideBlock(lay.bodyY + lay.paneH - lay.extraY)
		if i := y - lay.extraY; i < len(block) {
			switch {
			case block[i].key != "":
				return m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(block[i].key)})
			case block[i].day != "":
				return m.gotoDay(block[i].day)
			}
		}
	}
	return m, nil
}

// Derived helpers.

// applySidebar resolves the sidebar preference against the terminal width.
func (m *model) applySidebar() {
	m.showSidebar = m.sidebarPref && m.width >= 70
}

// fullLoad returns the command that reloads everything for the current view.
func (m model) fullLoad() tea.Cmd {
	m.loading = true
	return loadCmd(m.jot, m.journal, m.month, m.selected)
}

// dayLoad returns the command that reloads the selected day only.
func (m model) dayLoad() tea.Cmd {
	return dayCmd(m.jot, m.journal, m.selected)
}

// grid computes the month grid for the displayed month.
func (m model) grid() []Week {
	counts := make(map[string]int, len(m.counts))
	for k, c := range m.counts {
		counts[k] = c.Entries
	}
	return MonthGrid(m.month, m.today, m.selected, counts)
}

// monthStats returns the days-with-files and entry totals of the displayed
// month.
func (m model) monthStats() (days, entries int) {
	for _, c := range m.counts {
		days++
		entries += c.Entries
	}
	return days, entries
}

// visibleJournals returns the scroll offset and the journal rows that fit in
// the fixed-height sidebar list.
func (m model) visibleJournals(lay layout) (int, []jot.JournalInfo) {
	rows := max(lay.sideRows, 1)
	if rows >= len(m.journals) {
		return 0, m.journals
	}
	start := clamp(m.sideTop, 0, len(m.journals)-rows)
	return start, m.journals[start : start+rows]
}

// revealJournal scrolls the sidebar list so the open journal is visible.
func (m *model) revealJournal() {
	rows := max(m.layout().sideRows, 1)
	for i, j := range m.journals {
		if j.Name != m.journal {
			continue
		}
		if i < m.sideTop {
			m.sideTop = i
		} else if i >= m.sideTop+rows {
			m.sideTop = i - rows + 1
		}
	}
}

// clamp constrains v to [lo, hi]; hi < lo collapses to lo.
func clamp(v, lo, hi int) int {
	if hi < lo {
		return lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

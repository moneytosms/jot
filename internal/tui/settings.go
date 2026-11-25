package tui

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/moneytosms/jot"
)

// Settings tabs.
const (
	tabGeneral = iota
	tabJournals
	tabStorage
	tabArchives
)

var settingsTabs = []string{"General", "Journals", "Storage", "Archives"}

// generalRows is the number of General rows: theme, editor, default journal,
// reminders, git sync, one toggle per widget, then the intro tour.
var generalRows = 6 + len(widgetNames)

// settingsBodyH is the number of content lines each tab body gets, so the
// panel keeps one size across tabs.
const settingsBodyH = 11

// settingsJournalRows is how many journals the Journals tab lists at once;
// the header takes one body line.
const settingsJournalRows = settingsBodyH - 2

// settingsState is the state of the centered settings panel.
type settingsState struct {
	tab        int
	cursor     int // row in the General or Storage tab
	jcur, jtop int // selected journal and first listed one in the Journals tab
	editor     lineInput
	journal    lineInput
	root       lineInput
	configPath string
	remindVia  string // what the reminders setting resolves to, for display
	confirm    bool   // the move-root row is waiting for a second enter
	encConfirm string // journal whose encryption toggle awaits a second x
	archives   []jot.Archive
	acur, atop int // selected and first listed archive
	err        string
	warn       string
}

// settingsSavedMsg reports the result of saving one setting to the config
// file.
type settingsSavedMsg struct {
	kind   string
	value  string
	path   string
	exists bool
	err    error
}

// rootSavedMsg reports a journal root change, with or without moving data.
type rootSavedMsg struct {
	root  string
	moved bool
	err   error
}

// openSettings opens the settings panel with the current values.
func (m model) openSettings() (tea.Model, tea.Cmd) {
	st := settingsState{configPath: jot.ConfigPath(), tab: m.settings.tab}
	st.editor = newLineInput()
	st.editor.prompt = ""
	st.editor.placeholder = "auto-detect"
	st.editor.setValue(strings.TrimSpace(m.cfg.Editor))
	st.journal = newLineInput()
	st.journal.prompt = ""
	st.journal.placeholder = "default"
	journal := strings.TrimSpace(m.cfg.DefaultJournal)
	if journal == "" {
		journal = m.jot.DefaultJournal()
	}
	st.journal.setValue(journal)
	st.root = newLineInput()
	st.root.prompt = ""
	st.root.placeholder = "new journal root, e.g. ~/notes/jot"
	st.remindVia = remindViaText(m.cfg.Reminders)
	for i, j := range m.journals {
		if j.Name == m.journal {
			st.jcur = i
		}
	}
	st.jtop = max(st.jcur-settingsJournalRows+1, 0)
	m.settings = st
	m.mode = modeSettings
	m.applySettingsFocus()
	return m, nil
}

// settingsRows is the number of selectable rows in the active tab.
func (m model) settingsRows() int {
	switch m.settings.tab {
	case tabJournals:
		return len(m.journals)
	case tabStorage:
		return 5
	case tabArchives:
		return len(m.settings.archives)
	}
	return generalRows
}

// applySettingsFocus focuses the line input of the active row, if any.
func (m *model) applySettingsFocus() {
	m.settings.editor.blur()
	m.settings.journal.blur()
	m.settings.root.blur()
	switch {
	case m.settings.tab == tabGeneral && m.settings.cursor == 1:
		m.settings.editor.focus()
	case m.settings.tab == tabGeneral && m.settings.cursor == 2:
		m.settings.journal.focus()
	case m.settings.tab == tabStorage && m.settings.cursor == 0:
		m.settings.root.focus()
	}
}

// switchSettingsTab moves to another tab and resets the row and messages.
func (m *model) switchSettingsTab(tab int) {
	n := len(settingsTabs)
	m.settings.tab = (tab%n + n) % n
	m.settings.cursor = 0
	m.settings.err, m.settings.warn, m.settings.confirm, m.settings.encConfirm = "", "", false, ""
	m.applySettingsFocus()
}

// archivesCmd lists the archives for the Archives tab.
func archivesCmd(j *jot.Jot) tea.Cmd {
	return func() tea.Msg {
		list, err := j.Archives()
		return archivesMsg{list: list, err: err}
	}
}

// archivesMsg carries the archive listing.
type archivesMsg struct {
	list []jot.Archive
	err  error
}

// restoreCmd unpacks an archive into the root.
func restoreCmd(j *jot.Jot, path string) tea.Cmd {
	return func() tea.Msg {
		names, err := j.RestoreArchive(path)
		return restoredMsg{names: names, err: err}
	}
}

// restoredMsg reports an archive restore.
type restoredMsg struct {
	names []string
	err   error
}

// encryptCmd turns a journal's encryption on or off.
func encryptCmd(j *jot.Jot, name string, on bool) tea.Cmd {
	return func() tea.Msg {
		n, err := j.SetEncrypted(name, on)
		return encryptedMsg{name: name, on: on, n: n, err: err}
	}
}

// encryptedMsg reports an encryption toggle.
type encryptedMsg struct {
	name string
	on   bool
	n    int
	err  error
}

// applySettingsAsync merges the Archives, restore and encryption results.
func (m model) applySettingsAsync(msg tea.Msg) (tea.Model, tea.Cmd) {
	var syncErr *jot.SyncError
	switch msg := msg.(type) {
	case archivesMsg:
		m.settings.archives = msg.list
		m.settings.acur = clamp(m.settings.acur, 0, max(len(msg.list)-1, 0))
		if msg.err != nil {
			m.settings.err = msg.err.Error()
		}
	case restoredMsg:
		if msg.err != nil && !errors.As(msg.err, &syncErr) {
			m.settings.err = msg.err.Error()
			return m, nil
		}
		return m, tea.Batch(m.setToast("restored "+strings.Join(msg.names, ", ")), m.fullLoad())
	case encryptedMsg:
		if msg.err != nil && !errors.As(msg.err, &syncErr) {
			m.settings.err = msg.err.Error()
			return m, nil
		}
		text := fmt.Sprintf("encrypted %s (%d %s); back up %s", msg.name, msg.n, plural(msg.n, "day", "days"), jot.KeyPath())
		if !msg.on {
			text = fmt.Sprintf("decrypted %s (%d %s)", msg.name, msg.n, plural(msg.n, "day", "days"))
		}
		return m, tea.Batch(m.setToast(text), m.fullLoad())
	}
	return m, nil
}

// moveSettingsCursor moves the selected row of the active tab by delta.
func (m *model) moveSettingsCursor(delta int) {
	n := m.settingsRows()
	if n == 0 {
		return
	}
	m.settings.confirm = false
	m.settings.encConfirm = ""
	if m.settings.tab == tabJournals || m.settings.tab == tabArchives {
		cur, top := &m.settings.jcur, &m.settings.jtop
		if m.settings.tab == tabArchives {
			cur, top = &m.settings.acur, &m.settings.atop
		}
		*cur = clamp(*cur+delta, 0, n-1)
		if *cur < *top {
			*top = *cur
		} else if *cur >= *top+settingsJournalRows {
			*top = *cur - settingsJournalRows + 1
		}
		return
	}
	m.settings.cursor = (m.settings.cursor + delta + n) % n
	m.applySettingsFocus()
}

// keySettings feeds keys to the settings panel.
func (m model) keySettings(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "esc":
		m.closeOverlay()
		return m, nil
	case "tab", "shift+tab":
		step := 1
		if key == "shift+tab" {
			step = -1
		}
		m.switchSettingsTab(m.settings.tab + step)
		if m.settings.tab == tabArchives {
			return m, archivesCmd(m.jot)
		}
		return m, nil
	case "down":
		m.moveSettingsCursor(1)
		return m, nil
	case "up":
		m.moveSettingsCursor(-1)
		return m, nil
	}
	switch m.settings.tab {
	case tabJournals:
		return m.keySettingsJournals(key)
	case tabStorage:
		return m.keySettingsStorage(msg)
	case tabArchives:
		switch key {
		case "j":
			m.moveSettingsCursor(1)
		case "k":
			m.moveSettingsCursor(-1)
		case "enter":
			if len(m.settings.archives) > 0 {
				m.settings.err = ""
				return m, restoreCmd(m.jot, m.settings.archives[m.settings.acur].Path)
			}
		}
		return m, nil
	}
	return m.keySettingsGeneral(msg)
}

// keySettingsGeneral handles the General tab rows.
func (m model) keySettingsGeneral(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "enter":
		return m.activateSettingsRow()
	case "left", "right":
		switch m.settings.cursor {
		case 0:
			return m, m.cycleTheme()
		case 3:
			return m.cycleReminders(key == "left")
		case 4:
			return m.cycleGit(key == "left")
		}
	}
	var in *lineInput
	switch m.settings.cursor {
	case 1:
		in = &m.settings.editor
	case 2:
		in = &m.settings.journal
	default:
		return m, nil
	}
	if in.update(msg) {
		m.settings.err = ""
		m.settings.warn = ""
	}
	return m, nil
}

// keySettingsJournals handles the Journals tab: per-journal actions on the
// highlighted row.
func (m model) keySettingsJournals(key string) (tea.Model, tea.Cmd) {
	if key == "n" {
		m.closeOverlay()
		return m.startNewJournal()
	}
	if len(m.journals) == 0 {
		return m, nil
	}
	name := m.journals[clamp(m.settings.jcur, 0, len(m.journals)-1)].Name
	switch key {
	case "j":
		m.moveSettingsCursor(1)
	case "k":
		m.moveSettingsCursor(-1)
	case "enter", "o":
		m.closeOverlay()
		return m.switchJournal(name)
	case "r":
		return m.startRenameJournal(name)
	case "d":
		return m.startDeleteJournal(name)
	case "a":
		m.settings.err = ""
		return m, archiveJournalCmd(m.jot, name)
	case "x":
		on := !m.jot.IsEncrypted(name)
		if m.settings.encConfirm != name {
			m.settings.encConfirm = name
			if on {
				m.settings.warn = "x again to encrypt " + name + "; the key is " + jot.KeyPath() + " - back it up"
			} else {
				m.settings.warn = "x again to decrypt " + name + " back to plain markdown"
			}
			return m, nil
		}
		m.settings.encConfirm, m.settings.warn, m.settings.err = "", "", ""
		return m, encryptCmd(m.jot, name, on)
	case "c":
		cur := 0
		for i, c := range journalColors {
			if c == m.cfg.TUI.Colors[name] {
				cur = i
			}
		}
		return m, saveSettingCmd(m.jot, "color", name+"="+journalColors[(cur+1)%len(journalColors)])
	case "t":
		editor := strings.TrimSpace(m.cfg.Editor)
		if editor == "" {
			editor = jot.DetectEditor()
		}
		if editor == "" {
			m.settings.err = "no editor found: set JOT_EDITOR or $EDITOR"
			return m, nil
		}
		m.closeOverlay()
		path := m.jot.TemplatePath(name)
		return m, tea.ExecProcess(shellEditor(editor, path), func(err error) tea.Msg {
			if err != nil {
				return errMsg{fmt.Errorf("editor: %w", err)}
			}
			return toastMsg("template saved: " + path)
		})
	case "s":
		m.settings.err = ""
		m.settings.journal.setValue(name)
		return m, saveSettingCmd(m.jot, "journal", name)
	}
	return m, nil
}

// keySettingsStorage handles the Storage tab: the root path input and the
// move / point-only rows.
func (m model) keySettingsStorage(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() != "enter" {
		if m.settings.cursor == 0 && m.settings.root.update(msg) {
			m.settings.err, m.settings.warn, m.settings.confirm = "", "", false
		}
		return m, nil
	}
	if m.settings.cursor == 0 {
		m.moveSettingsCursor(1)
		return m, nil
	}
	if m.settings.cursor >= 3 { // backup rows: .tar.gz, .zip
		m.settings.err, m.settings.warn = "", "backing up…"
		return m, backupWithCmd(m.jot, jot.BackupOptions{Zip: m.settings.cursor == 4})
	}
	m.settings.err, m.settings.warn = "", ""
	if os.Getenv(jot.EnvRoot) != "" {
		m.settings.err = "JOT_ROOT is set and overrides the config; change it in your shell"
		return m, nil
	}
	dst, err := jot.ResolveRoot(m.settings.root.value())
	if err != nil {
		m.settings.err = "type a new root path first"
		m.settings.cursor = 0
		m.applySettingsFocus()
		return m, nil
	}
	if dst == m.jot.Root() {
		m.settings.err = "that is already the journal root"
		return m, nil
	}
	move := m.settings.cursor == 1
	if move && !m.settings.confirm {
		m.settings.confirm = true
		m.settings.warn = "enter again to move every journal to " + dst
		return m, nil
	}
	m.settings.confirm = false
	m.settings.warn = "working…"
	return m, setRootCmd(m.jot, dst, move)
}

// activateSettingsRow runs the current General row: cycle the theme or save
// a line input.
func (m model) activateSettingsRow() (tea.Model, tea.Cmd) {
	switch m.settings.cursor {
	case 0:
		return m, m.cycleTheme()
	case 1:
		value := strings.TrimSpace(m.settings.editor.value())
		m.settings.err = ""
		return m, saveSettingCmd(m.jot, "editor", value)
	case 2:
		raw := strings.TrimSpace(m.settings.journal.value())
		if raw == "" {
			m.settings.err = "default journal is empty"
			return m, nil
		}
		name := jot.SanitizeName(raw)
		if name == "" {
			m.settings.err = "default journal name has no usable characters"
			return m, nil
		}
		m.settings.journal.setValue(name)
		m.settings.err = ""
		return m, saveSettingCmd(m.jot, "journal", name)
	case 3:
		return m.cycleReminders(false)
	case 4:
		return m.cycleGit(false)
	}
	if i := m.settings.cursor - 5; i >= 0 && i < len(widgetNames) {
		return m, saveSettingCmd(m.jot, "hide", widgetNames[i].id)
	}
	if m.settings.cursor == generalRows-1 {
		return m.startOnboard()
	}
	return m, nil
}

// cycleGit moves the git sync setting to the next (or previous) value.
func (m model) cycleGit(back bool) (tea.Model, tea.Cmd) {
	opts := jot.GitSettings
	cur := 0
	for i, o := range opts {
		if o == m.cfg.Git {
			cur = i
		}
	}
	step := 1
	if back {
		step = len(opts) - 1
	}
	m.settings.err = ""
	return m, saveSettingCmd(m.jot, "git", opts[(cur+step)%len(opts)])
}

// cycleReminders moves the reminders setting to the next (or previous)
// value and saves it.
func (m model) cycleReminders(back bool) (tea.Model, tea.Cmd) {
	opts := jot.ReminderSettings
	cur := 0
	for i, o := range opts {
		if o == m.cfg.Reminders {
			cur = i
		}
	}
	step := 1
	if back {
		step = len(opts) - 1
	}
	next := opts[(cur+step)%len(opts)]
	m.settings.err = ""
	return m, saveSettingCmd(m.jot, "reminders", next)
}

// remindViaText describes where a reminder would go under setting.
func remindViaText(setting string) string {
	p, err := jot.ReminderProvider(setting, 0)
	if err != nil {
		return "unavailable"
	}
	return "via " + p
}

// saveSettingCmd loads the config file, changes one field and saves it so
// unrelated settings are preserved. All disk work stays in the command.
func saveSettingCmd(j *jot.Jot, kind, value string) tea.Cmd {
	return func() tea.Msg {
		file, err := jot.LoadConfigFile()
		if err != nil {
			return settingsSavedMsg{kind: kind, value: value, err: err}
		}
		switch kind {
		case "editor":
			file.Editor = value
		case "journal":
			file.DefaultJournal = jot.SanitizeName(value)
			value = file.DefaultJournal
		case "reminders":
			file.Reminders = value
		case "git":
			file.Git = value
		case "hide":
			if i := slices.Index(file.TUI.Hide, value); i >= 0 {
				file.TUI.Hide = slices.Delete(file.TUI.Hide, i, i+1)
			} else {
				file.TUI.Hide = append(file.TUI.Hide, value)
			}
		case "onboarded":
			file.TUI.Onboarded = true
		case "color":
			name, color, _ := strings.Cut(value, "=")
			if file.TUI.Colors == nil {
				file.TUI.Colors = map[string]string{}
			}
			if color == "" {
				delete(file.TUI.Colors, name)
			} else {
				file.TUI.Colors[name] = color
			}
		}
		path, err := jot.SaveConfig(file)
		if kind == "hide" {
			value = strings.Join(file.TUI.Hide, ",")
		}
		msg := settingsSavedMsg{kind: kind, value: value, path: path, exists: true, err: err}
		if kind == "journal" {
			msg.exists = j.JournalExists(value)
		}
		return msg
	}
}

// archiveJournalCmd writes a journal backup without touching the journal.
func archiveJournalCmd(j *jot.Jot, name string) tea.Cmd {
	return func() tea.Msg {
		archive, err := j.ArchiveJournal(name)
		return journalOpMsg{kind: "archive", from: name, archive: archive, err: err}
	}
}

// setRootCmd points the config at a new journal root, moving the data there
// first when move is set. A failed config save moves the data back so the
// config and the files never disagree.
func setRootCmd(j *jot.Jot, dst string, move bool) tea.Cmd {
	return func() tea.Msg {
		file, err := jot.LoadConfigFile()
		if err != nil {
			return rootSavedMsg{err: err}
		}
		if move {
			if err := j.MoveRoot(dst); err != nil {
				return rootSavedMsg{err: err}
			}
		}
		file.Root = dst
		if _, err := jot.SaveConfig(file); err != nil {
			if move {
				if back := jot.New(jot.Config{Root: dst, DefaultJournal: j.DefaultJournal()}).MoveRoot(j.Root()); back != nil {
					err = fmt.Errorf("%w; journals are now in %s (moving back failed: %v)", err, dst, back)
				}
			}
			return rootSavedMsg{err: err}
		}
		return rootSavedMsg{root: dst, moved: move}
	}
}

// applyRootSaved switches the open handle to the new root and reloads.
func (m model) applyRootSaved(msg rootSavedMsg) (tea.Model, tea.Cmd) {
	m.settings.warn = ""
	if msg.err != nil {
		m.settings.err = msg.err.Error()
		m.lastErr = msg.err
		return m, m.setToast("root not changed")
	}
	m.cfg.Root = msg.root
	m.jot = jot.New(m.cfg)
	m.settings.root.setValue("")
	text := "journal root: " + msg.root
	if msg.moved {
		text = "moved journals to " + msg.root
	}
	return m, tea.Batch(m.setToast(text), m.fullLoad())
}

// applySettingsSaved merges a config save result.
func (m model) applySettingsSaved(msg settingsSavedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.settings.err = msg.err.Error()
		m.lastErr = msg.err
		return m, m.setToast("not saved: " + msg.err.Error())
	}
	switch msg.kind {
	case "editor":
		m.cfg.Editor = msg.value
		m.settings.err = ""
		m.settings.warn = ""
		return m, m.setToast("editor saved")
	case "journal":
		m.cfg.DefaultJournal = msg.value
		m.settings.journal.setValue(msg.value)
		m.settings.err = ""
		if msg.exists {
			m.settings.warn = ""
			return m, m.setToast("default journal: " + msg.value)
		}
		m.settings.warn = "journal " + msg.value + " does not exist yet"
		return m, m.setToast("default journal saved (does not exist yet)")
	case "reminders":
		m.cfg.Reminders = msg.value
		m.settings.remindVia = remindViaText(msg.value)
		return m, m.setToast("reminders: " + msg.value)
	case "git":
		m.cfg.Git = msg.value
		m.jot = jot.New(m.cfg)
		return m, m.setToast("git sync: " + msg.value)
	case "onboarded":
		m.cfg.TUI.Onboarded = true
		return m, nil
	case "hide":
		m.cfg.TUI.Hide = nil
		if msg.value != "" {
			m.cfg.TUI.Hide = strings.Split(msg.value, ",")
		}
		return m, nil
	case "color":
		name, color, _ := strings.Cut(msg.value, "=")
		colors := maps.Clone(m.cfg.TUI.Colors)
		if colors == nil {
			colors = map[string]string{}
		}
		if color == "" {
			delete(colors, name)
		} else {
			colors[name] = color
		}
		m.cfg.TUI.Colors = colors
		return m, nil
	}
	return m, nil
}

// settingsRect is the centered settings panel: tabs, a blank line, the tab
// body, the key hints and the status line inside the border.
func (m model) settingsRect(lay layout) boxRect {
	w := min(max(m.width-8, 1), 86)
	return m.centerRect(lay, w, settingsBodyH+6)
}

// settingsRow renders one labeled settings row.
func (m model) settingsRow(label, value string, active bool) string {
	marker := "  "
	labelStyle := m.theme.muted()
	if active {
		marker = m.theme.accent().Bold(true).Render("▸ ")
		labelStyle = m.theme.accent().Bold(true)
	}
	return marker + labelStyle.Render(padRight(label, 16)) + value
}

// tabLabel is how one tab name is drawn in the tab bar; its width is the
// same in every state so clicks map from geometry alone.
func tabLabel(name string) string { return " " + name + " " }

// settingsTabBar renders the tab names, the active one highlighted.
func (m model) settingsTabBar() string {
	var b strings.Builder
	for i, name := range settingsTabs {
		if i > 0 {
			b.WriteString(m.theme.dim().Render("│"))
		}
		if i == m.settings.tab {
			b.WriteString(m.theme.selStyle().Bold(true).Render(tabLabel(name)))
		} else {
			b.WriteString(m.theme.muted().Render(tabLabel(name)))
		}
	}
	return b.String()
}

// settingsView renders the settings panel.
func (m model) settingsView(lay layout) string {
	rect := m.settingsRect(lay)
	var body []string
	var hint string
	switch m.settings.tab {
	case tabGeneral:
		body, hint = m.settingsGeneralBody()
	case tabJournals:
		body, hint = m.settingsJournalsBody(rect.w - 4)
	case tabStorage:
		body, hint = m.settingsStorageBody()
	case tabArchives:
		body, hint = m.settingsArchivesBody()
	}
	for len(body) < settingsBodyH {
		body = append(body, "")
	}
	content := append([]string{m.settingsTabBar(), ""}, body[:settingsBodyH]...)
	content = append(content, m.theme.dim().Render(hint))
	status := ""
	switch {
	case m.settings.err != "":
		status = m.theme.bad().Render(m.settings.err)
	case m.settings.warn != "":
		status = m.theme.muted().Render(m.settings.warn)
	}
	content = append(content, status)
	return m.overlayCanvas(lay, rect, m.boxLines("SETTINGS", rect, content))
}

// settingsGeneralBody renders the General tab.
func (m model) settingsGeneralBody() ([]string, string) {
	cur := m.settings.cursor
	themeValue := m.theme.fg().Render(m.themeName) + m.theme.dim().Render("  enter cycles")
	git := m.cfg.Git
	if git == "" {
		git = jot.GitOff
	}
	out := []string{
		m.settingsRow("Theme", themeValue, cur == 0),
		m.settingsRow("Editor command", m.settings.editor.view(), cur == 1),
		m.settingsRow("Default journal", m.settings.journal.view(), cur == 2),
		m.settingsRow("Reminders", m.theme.fg().Render(reminderSettingName(m.cfg.Reminders))+
			m.theme.dim().Render("  "+m.settings.remindVia+" · enter cycles"), cur == 3),
		m.settingsRow("Git sync", m.theme.fg().Render(git)+m.theme.dim().Render("  off · commit · push (if a remote is set)"), cur == 4),
	}
	for i, w := range widgetNames {
		state := m.theme.good().Render("● shown")
		if !m.widgetOn(w.id) {
			state = m.theme.dim().Render("○ hidden")
		}
		out = append(out, m.settingsRow("  "+w.label, state+m.theme.dim().Render("  under the calendar"), cur == 5+i))
	}
	out = append(out, m.settingsRow("Intro tour", m.theme.dim().Render("enter to replay what each part does"), cur == generalRows-1))
	return append(out, "", m.settingsRow("Config file", m.theme.fg().Render(m.settings.configPath), false)),
		"tab next tab · up/down row · enter save, cycle or toggle · esc close"
}

// settingsJournalsBody renders the Journals tab: a scrolling list with the
// selected journal's actions in the hint line.
func (m model) settingsJournalsBody(width int) ([]string, string) {
	hint := "x encrypt · t daily template · c colour · n new journal"
	if len(m.journals) == 0 {
		return []string{m.theme.muted().Render("  No journals yet. Press n to create one.")}, hint
	}
	head := fmt.Sprintf("  %-18s %5s %6s  %-10s", "journal", "days", "notes", "last write")
	if len(m.journals) > settingsJournalRows {
		head += fmt.Sprintf("  %d-%d/%d", m.settings.jtop+1, min(m.settings.jtop+settingsJournalRows, len(m.journals)), len(m.journals))
	}
	out := []string{m.theme.dim().Render(ansiTruncate(head, width))}
	end := min(m.settings.jtop+settingsJournalRows, len(m.journals))
	for i := m.settings.jtop; i < end; i++ {
		j := m.journals[i]
		last := "—"
		if !j.Last.IsZero() {
			last = j.Last.Format("2006-01-02")
		}
		name := j.Name
		if r := []rune(name); len(r) > 18 {
			name = string(r[:17]) + "…"
		}
		var tags []string
		if j.Name == m.journal {
			tags = append(tags, "open")
		}
		if j.Name == m.cfg.DefaultJournal {
			tags = append(tags, "default")
		}
		if m.jot.IsEncrypted(j.Name) {
			tags = append(tags, "encrypted")
		}
		if c := m.cfg.TUI.Colors[j.Name]; c != "" {
			tags = append(tags, c)
		}
		row := fmt.Sprintf("%-18s %5d %6d  %-10s", name, j.Days, j.Entries, last)
		tag := ""
		if len(tags) > 0 {
			tag = "  " + m.theme.accent().Render(strings.Join(tags, " · "))
		}
		if i == m.settings.jcur {
			out = append(out, m.theme.accent().Bold(true).Render("▸ ")+m.theme.selStyle().Render(row)+tag)
		} else {
			out = append(out, "  "+m.theme.fg().Render(row)+tag)
		}
	}
	for len(out) < settingsBodyH-1 {
		out = append(out, "")
	}
	return append(out, m.theme.dim().Render("⏎ open · r rename · a archive · d delete · s make default")), hint
}

// settingsStorageBody renders the Storage tab.
func (m model) settingsStorageBody() ([]string, string) {
	cur := m.settings.cursor
	moveLabel := "move every journal there, then use it"
	if m.settings.confirm {
		moveLabel = m.theme.bad().Render("enter again to confirm the move")
	}
	out := []string{
		m.settingsRow("New root", m.settings.root.view(), cur == 0),
		m.settingsRow("Move journals", m.theme.fg().Render(moveLabel), cur == 1),
		m.settingsRow("Use path only", m.theme.fg().Render("point jot there, leave files where they are"), cur == 2),
		m.settingsRow("Back up .tar.gz", m.theme.fg().Render("journals, templates and config into Backups"), cur == 3),
		m.settingsRow("Back up .zip", m.theme.fg().Render("the same, as a zip"), cur == 4),
		"",
		m.settingsRow("Journal root", m.theme.fg().Render(m.jot.Root()), false),
		m.settingsRow("Backups", m.theme.fg().Render(filepath.Join(m.jot.Root(), "backups")), false),
		m.settingsRow("Config file", m.theme.fg().Render(m.settings.configPath), false),
	}
	if os.Getenv(jot.EnvRoot) != "" {
		out = append(out, "", m.theme.muted().Render("  JOT_ROOT is set; it overrides the config root"))
	}
	return out, "tab next tab · up/down row · enter run row · esc close"
}

// settingsArchivesBody renders the Archives tab: archives and backups in
// <root>/backups, newest first.
func (m model) settingsArchivesBody() ([]string, string) {
	hint := "⏎ restore the highlighted archive (refuses journals that exist) · esc close"
	if len(m.settings.archives) == 0 {
		return []string{m.theme.muted().Render("  No archives yet. Archive a journal with a in the Journals tab.")}, hint
	}
	head := fmt.Sprintf("  %-50s %8s  %s", "archive", "size", "written")
	out := []string{m.theme.dim().Render(head)}
	end := min(m.settings.atop+settingsJournalRows, len(m.settings.archives))
	for i := m.settings.atop; i < end; i++ {
		a := m.settings.archives[i]
		name := a.Name
		if r := []rune(name); len(r) > 50 {
			name = string(r[:49]) + "…"
		}
		row := fmt.Sprintf("%-50s %7.1fK  %s", name, float64(a.Size)/1024, a.Modified.Format("2006-01-02 15:04"))
		if i == m.settings.acur {
			out = append(out, m.theme.accent().Bold(true).Render("▸ ")+m.theme.selStyle().Render(row))
		} else {
			out = append(out, "  "+m.theme.fg().Render(row))
		}
	}
	return out, hint
}

// reminderSettingName shows the empty setting as the "auto" it means.
func reminderSettingName(s string) string {
	if s == "" {
		return jot.RemindAuto
	}
	return s
}

// handleSettingsMouse switches tabs, focuses or runs clicked rows, scrolls
// the journal list and closes the panel when the click lands outside it.
func (m model) handleSettingsMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	lay := m.layout()
	rect := m.settingsRect(lay)
	switch msg.Button {
	case tea.MouseButtonWheelUp, tea.MouseButtonWheelDown:
		if m.settings.tab == tabJournals {
			delta := 1
			if msg.Button == tea.MouseButtonWheelUp {
				delta = -1
			}
			m.moveSettingsCursor(delta)
		}
		return m, nil
	case tea.MouseButtonLeft:
	default:
		return m, nil
	}
	if !pointInRect(msg.X, msg.Y, rect) {
		m.closeOverlay()
		return m, nil
	}
	row := contentRow(msg.Y, rect)
	if row == 0 {
		x := msg.X - rect.x - 2 // border plus the content's leading space
		for i, name := range settingsTabs {
			w := len(tabLabel(name))
			if x >= 0 && x < w {
				m.switchSettingsTab(i)
				if i == tabArchives {
					return m, archivesCmd(m.jot)
				}
				return m, nil
			}
			x -= w + 1 // the separator
		}
		return m, nil
	}
	row -= 2 // tab bar and blank line
	switch m.settings.tab {
	case tabJournals:
		i := m.settings.jtop + row - 1 // the list header takes one row
		if row >= 1 && i < min(m.settings.jtop+settingsJournalRows, len(m.journals)) {
			m.settings.jcur = i
		}
		return m, nil
	case tabStorage:
		if row >= 0 && row < 5 {
			m.settings.cursor = row
			m.applySettingsFocus()
		}
		return m, nil
	case tabArchives:
		i := m.settings.atop + row - 1
		if row >= 1 && i < min(m.settings.atop+settingsJournalRows, len(m.settings.archives)) {
			m.settings.acur = i
		}
		return m, nil
	}
	if row < 0 || row >= generalRows {
		return m, nil
	}
	m.settings.cursor = row
	m.applySettingsFocus()
	switch row {
	case 0:
		return m, m.cycleTheme()
	case 3, 4:
		return m.activateSettingsRow()
	}
	if row >= 5 {
		return m.activateSettingsRow()
	}
	return m, nil
}

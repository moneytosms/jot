package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/moneytosms/jot"
)

// settingsState is the state of the centered settings panel.
type settingsState struct {
	cursor     int
	editor     lineInput
	journal    lineInput
	configPath string
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

// openSettings opens the settings panel with the current values.
func (m model) openSettings() (tea.Model, tea.Cmd) {
	st := settingsState{configPath: jot.ConfigPath()}
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
	m.settings = st
	m.mode = modeSettings
	m.applySettingsFocus()
	return m, nil
}

// applySettingsFocus focuses the line input of the active row, if any.
func (m *model) applySettingsFocus() {
	m.settings.editor.blur()
	m.settings.journal.blur()
	switch m.settings.cursor {
	case 1:
		m.settings.editor.focus()
	case 2:
		m.settings.journal.focus()
	}
}

// keySettings feeds keys to the settings panel.
func (m model) keySettings(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "esc":
		m.closeOverlay()
		return m, nil
	case "tab", "down":
		m.settings.cursor = (m.settings.cursor + 1) % 3
		m.applySettingsFocus()
		return m, nil
	case "shift+tab", "up":
		m.settings.cursor = (m.settings.cursor + 2) % 3
		m.applySettingsFocus()
		return m, nil
	case "enter":
		return m.activateSettingsRow()
	case "left", "right":
		if m.settings.cursor == 0 {
			return m, m.cycleTheme()
		}
	case "home", "end":
		if m.settings.cursor != 0 {
			break
		}
	}
	if m.settings.cursor == 1 {
		if m.settings.editor.update(msg) {
			m.settings.err = ""
			m.settings.warn = ""
		}
		return m, nil
	}
	if m.settings.cursor == 2 {
		if m.settings.journal.update(msg) {
			m.settings.err = ""
			m.settings.warn = ""
		}
	}
	return m, nil
}

// activateSettingsRow runs the current row: cycle the theme or save a line
// input.
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
	}
	return m, nil
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
		}
		path, err := jot.SaveConfig(file)
		msg := settingsSavedMsg{kind: kind, value: value, path: path, exists: true, err: err}
		if kind == "journal" {
			msg.exists = j.JournalExists(value)
		}
		return msg
	}
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
	}
	return m, nil
}

// settingsRect is the centered settings panel.
func (m model) settingsRect(lay layout) boxRect {
	w := min(max(m.width-8, 1), 74)
	return m.centerRect(lay, w, 11)
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

// settingsView renders the settings panel.
func (m model) settingsView(lay layout) string {
	rect := m.settingsRect(lay)
	themeValue := m.theme.fg().Render(m.themeName) + m.theme.dim().Render("  enter cycles")
	content := []string{
		m.settingsRow("Theme", themeValue, m.settings.cursor == 0),
		m.settingsRow("Editor command", m.settings.editor.view(), m.settings.cursor == 1),
		m.settingsRow("Default journal", m.settings.journal.view(), m.settings.cursor == 2),
		"",
		m.settingsRow("Config file", m.theme.fg().Render(m.settings.configPath), false),
		m.settingsRow("Journal root", m.theme.fg().Render(m.jot.Root()), false),
		"",
		m.theme.dim().Render("tab/up/down row · enter save or cycle · esc close"),
	}
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

// settingsRowAt maps a mouse coordinate to a settings row (0-2).
func (m model) settingsRowAt(lay layout, x, y int) (int, bool) {
	rect := m.settingsRect(lay)
	if !pointInRect(x, y, rect) {
		return 0, false
	}
	row := contentRow(y, rect)
	if row < 0 || row > 2 {
		return 0, false
	}
	return row, true
}

// handleSettingsMouse focuses a clicked row, cycles the theme row and closes
// the panel when the click lands outside it.
func (m model) handleSettingsMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if msg.Button != tea.MouseButtonLeft {
		return m, nil
	}
	lay := m.layout()
	if !pointInRect(msg.X, msg.Y, m.settingsRect(lay)) {
		m.closeOverlay()
		return m, nil
	}
	row, ok := m.settingsRowAt(lay, msg.X, msg.Y)
	if !ok {
		return m, nil
	}
	m.settings.cursor = row
	m.applySettingsFocus()
	if row == 0 {
		return m, m.cycleTheme()
	}
	return m, nil
}

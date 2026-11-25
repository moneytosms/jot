package tui

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/moneytosms/jot"
)

// reminderState is the state of the centered reminder form.
type reminderState struct {
	message  lineInput
	duration lineInput
	focus    int
	err      string
	pending  bool
}

// reminderDoneMsg carries the result of jot.Remind.
type reminderDoneMsg struct {
	label    string
	duration time.Duration
	at       time.Time
	err      error
}

// openReminder opens the form with the message prefilled and a default
// duration.
func (m model) openReminder(prefill string) (tea.Model, tea.Cmd) {
	st := reminderState{}
	st.message = newLineInput()
	st.message.prompt = ""
	st.message.placeholder = "what should jot remind you about?"
	st.message.setValue(strings.TrimSpace(prefill))
	st.duration = newLineInput()
	st.duration.prompt = ""
	st.duration.placeholder = "20m, 2h, 90s, 1h30m or minutes"
	st.duration.setValue("20m")
	m.reminder = st
	m.mode = modeReminder
	m.applyReminderFocus()
	return m, nil
}

// applyReminderFocus focuses the active field.
func (m *model) applyReminderFocus() {
	m.reminder.message.blur()
	m.reminder.duration.blur()
	if m.reminder.focus == 1 {
		m.reminder.duration.focus()
	} else {
		m.reminder.message.focus()
	}
}

// keyReminder feeds keys to the reminder form.
func (m model) keyReminder(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.closeOverlay()
		return m, nil
	case "enter":
		if m.reminder.pending {
			return m, nil
		}
		return m.submitReminder()
	case "tab", "up", "down":
		m.reminder.focus = 1 - m.reminder.focus
		m.applyReminderFocus()
		return m, nil
	}
	if m.reminder.focus == 1 {
		if m.reminder.duration.update(msg) {
			m.reminder.err = ""
		}
		return m, nil
	}
	if m.reminder.message.update(msg) {
		m.reminder.err = ""
	}
	return m, nil
}

// submitReminder validates the form and starts the reminder in the
// background. Invalid durations keep the form open with an inline error.
func (m model) submitReminder() (tea.Model, tea.Cmd) {
	message := strings.TrimSpace(m.reminder.message.value())
	if message == "" {
		m.reminder.err = "reminder message is empty"
		return m, nil
	}
	d, err := parseReminderDuration(m.reminder.duration.value())
	if err != nil {
		m.reminder.err = err.Error()
		return m, nil
	}
	m.reminder.err = ""
	m.reminder.pending = true
	return m, remindCmd(m.jot, m.cfg.Reminders, message, d)
}

// remindCmd sets the reminder off the update loop, through the provider the
// "reminders" setting picks, and reports the fire time.
func remindCmd(j *jot.Jot, setting, message string, d time.Duration) tea.Cmd {
	return func() tea.Msg {
		at := time.Now().Add(d)
		label, err := j.RemindVia(setting, message, d)
		return reminderDoneMsg{label: label, duration: d, at: at, err: err}
	}
}

// applyReminderDone closes the form on success and reports the fire time.
func (m model) applyReminderDone(msg reminderDoneMsg) (tea.Model, tea.Cmd) {
	if m.mode == modeReminder {
		m.reminder.pending = false
	}
	if msg.err != nil {
		if m.mode == modeReminder {
			m.reminder.err = msg.err.Error()
		} else {
			m.lastErr = msg.err
		}
		return m, nil
	}
	if m.mode == modeReminder {
		m.closeOverlay()
	}
	text := fmt.Sprintf("reminder set for %s at %s", formatReminderDuration(msg.duration), formatFireTime(msg.at))
	cmd := m.setToast(text)
	return m, cmd
}

// parseReminderDuration accepts Go duration syntax (20m, 2h, 90s, 1h30m) or a
// bare number of minutes.
func parseReminderDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, errors.New("duration is empty")
	}
	if isDigits(s) {
		n, err := strconv.Atoi(s)
		if err != nil {
			return 0, fmt.Errorf("invalid duration %q", s)
		}
		if n <= 0 {
			return 0, fmt.Errorf("duration must be positive, got %q", s)
		}
		return time.Duration(n) * time.Minute, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q: use 20m, 2h, 90s, 1h30m or minutes", s)
	}
	if d <= 0 {
		return 0, fmt.Errorf("duration must be positive, got %q", s)
	}
	return d, nil
}

// isDigits reports whether s is one or more ASCII digits.
func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// formatReminderDuration renders a duration without a trailing zero seconds
// component.
func formatReminderDuration(d time.Duration) string {
	s := d.String()
	if strings.HasSuffix(s, "m0s") {
		s = strings.TrimSuffix(s, "0s")
	}
	return s
}

// reminderRect is the centered reminder panel.
func (m model) reminderRect(lay layout) boxRect {
	w := min(max(m.width-8, 1), 64)
	return m.centerRect(lay, w, 9)
}

// reminderView renders the reminder form.
func (m model) reminderView(lay layout) string {
	rect := m.reminderRect(lay)
	focusMark := func(field int) string {
		if m.reminder.focus == field {
			return m.theme.accent().Bold(true).Render("▸ ")
		}
		return "  "
	}
	content := []string{
		focusMark(0) + m.theme.muted().Render("MESSAGE"),
		"  " + m.reminder.message.view(),
		"",
		focusMark(1) + m.theme.muted().Render("DURATION") + m.theme.dim().Render("  20m, 2h, 90s, 1h30m or minutes"),
		"  " + m.reminder.duration.view(),
	}
	errLine := ""
	if m.reminder.err != "" {
		errLine = m.theme.bad().Render(m.reminder.err)
	} else if m.reminder.pending {
		errLine = m.theme.muted().Render("setting reminder…")
	}
	content = append(content, errLine, m.theme.dim().Render("enter set · tab field · esc cancel"))
	return m.overlayCanvas(lay, rect, m.boxLines("REMINDER", rect, content))
}

// reminderRowAt maps a mouse coordinate to a form field; it returns 0 for the
// message and 1 for the duration.
func (m model) reminderRowAt(lay layout, x, y int) (int, bool) {
	rect := m.reminderRect(lay)
	if !pointInRect(x, y, rect) {
		return 0, false
	}
	row := contentRow(y, rect)
	switch row {
	case 1:
		return 0, true
	case 4:
		return 1, true
	}
	return 0, false
}

// handleReminderMouse focuses a clicked field and closes the form when the
// click is outside it.
func (m model) handleReminderMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if msg.Button != tea.MouseButtonLeft {
		return m, nil
	}
	lay := m.layout()
	if !pointInRect(msg.X, msg.Y, m.reminderRect(lay)) {
		m.closeOverlay()
		return m, nil
	}
	if field, ok := m.reminderRowAt(lay, msg.X, msg.Y); ok {
		m.reminder.focus = field
		m.applyReminderFocus()
	}
	return m, nil
}

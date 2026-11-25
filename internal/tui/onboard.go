package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// onboardSteps is the intro tour: one card per part of the screen, each with
// the keys to try. It opens on the first run and from Settings or the palette.
var onboardSteps = []struct{ title, body, keys string }{
	{"Welcome to jot", "A journal that is only files: one Markdown file per day, in a folder per journal. Nothing to sync, nothing to sign in to. This takes a minute.", "enter next · esc skip"},
	{"The calendar", "Left: the month. Coloured days have entries, green for a few and yellow for a busy day. Today is underlined. Under it: open todos, reminders and notes from this day in past years.", "h/l day · j/k week · H/L month · t today · y year"},
	{"The day", "Right: the selected day, rendered from its file. Add a line without leaving, or open the whole day in your editor; jot never rewrites what you wrote.", "a quick add · e edit in your editor"},
	{"Journals", "Keep separate journals, like work and ideas. Each is a folder of day files. Tab hides the sidebar when you want the room.", "J/K switch · n new · R rename · D delete"},
	{"Finding things", "Search every entry, typos welcome. Narrow it with #tag, is:open or after:7d. T collects every checkbox; the palette runs any command by name.", "/ search · T todos · ctrl+p palette"},
	{"Reminders and export", "Set a reminder that pops up as a notification. Export a day or a whole journal as HTML or PDF from the palette.", "r reminder · ctrl+p export"},
	{"Settings and help", "Theme, editor, reminders, git sync, encryption and backups live in Settings, where this tour is too. ? lists every key; type to filter it.", "S settings · ? keys · q quit"},
}

// startOnboard opens the tour at its first card.
func (m model) startOnboard() (tea.Model, tea.Cmd) {
	m.closeOverlay()
	m.onboard = 0
	m.mode = modeOnboard
	return m, nil
}

// keyOnboard pages through the tour.
func (m model) keyOnboard(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", "right", "l", " ", "n":
		return m.moveOnboard(1)
	case "left", "h", "p":
		return m.moveOnboard(-1)
	case "esc", "q":
		return m.finishOnboard()
	}
	return m, nil
}

// moveOnboard steps the tour; stepping past the last card finishes it.
func (m model) moveOnboard(delta int) (tea.Model, tea.Cmd) {
	if m.onboard+delta >= len(onboardSteps) {
		return m.finishOnboard()
	}
	m.onboard = max(m.onboard+delta, 0)
	return m, nil
}

// finishOnboard closes the tour and remembers it was seen.
func (m model) finishOnboard() (tea.Model, tea.Cmd) {
	m.closeOverlay()
	if m.cfg.TUI.Onboarded {
		return m, nil
	}
	return m, saveSettingCmd(m.jot, "onboarded", "true")
}

// onboardView renders the current card.
func (m model) onboardView(lay layout) string {
	rect := m.centerRect(lay, min(max(m.width-8, 1), 66), 13)
	step := onboardSteps[m.onboard]
	dots := ""
	for i := range onboardSteps {
		if i == m.onboard {
			dots += m.theme.accent().Render("■ ")
		} else {
			dots += m.theme.dim().Render("□ ")
		}
	}
	body := lipgloss.NewStyle().Width(max(rect.w-6, 10)).Render(step.body)
	content := []string{dots, "", m.theme.accent().Bold(true).Render(step.title), ""}
	for _, l := range strings.Split(body, "\n") {
		content = append(content, m.theme.fg().Render(l))
	}
	content = append(content, "", m.theme.muted().Render("try  ")+m.theme.fg().Render(step.keys))
	for len(content) < rect.h-2 {
		content = append(content, "")
	}
	return m.overlayCanvas(lay, rect, m.boxLines("INTRO", rect, content[:rect.h-2]))
}

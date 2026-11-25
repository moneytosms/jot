package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// inputLimit caps how many runes the line editor accepts.
const inputLimit = 512

// lineInput is a minimal single-line text editor used by the quick-add and
// new-journal prompts. It supports just what those prompts need: typing,
// backspace/delete, cursor moves and home/end, with a reverse-video cursor.
//
// It deliberately replaces bubbles/textinput, whose transitive dependency
// (atotto/clipboard) has no go.sum entry in this module.
type lineInput struct {
	prompt      string
	placeholder string
	text        []rune
	pos         int
	focused     bool
}

// newLineInput returns an unfocused editor with the standard prompt.
func newLineInput() lineInput {
	return lineInput{prompt: "› "}
}

// value returns the current text.
func (l *lineInput) value() string { return string(l.text) }

// setValue replaces the text and moves the cursor to the end.
func (l *lineInput) setValue(s string) {
	l.text = []rune(s)
	l.pos = len(l.text)
}

// reset clears the text and cursor.
func (l *lineInput) reset() {
	l.text = nil
	l.pos = 0
}

// focus enables keyboard capture and the cursor.
func (l *lineInput) focus() { l.focused = true }

// blur disables the cursor.
func (l *lineInput) blur() { l.focused = false }

// isFocused reports whether the editor has focus.
func (l lineInput) isFocused() bool { return l.focused }

// update applies one key press and reports whether it changed the text or
// cursor.
func (l *lineInput) update(msg tea.KeyMsg) bool {
	switch msg.Type {
	case tea.KeyRunes, tea.KeySpace:
		if len(msg.Runes) == 0 {
			return false
		}
		if len(l.text)+len(msg.Runes) > inputLimit {
			return false
		}
		l.insert(msg.Runes)
		return true
	case tea.KeyBackspace:
		if l.pos > 0 {
			l.text = append(l.text[:l.pos-1], l.text[l.pos:]...)
			l.pos--
			return true
		}
	case tea.KeyDelete:
		if l.pos < len(l.text) {
			l.text = append(l.text[:l.pos], l.text[l.pos+1:]...)
			return true
		}
	case tea.KeyLeft:
		if l.pos > 0 {
			l.pos--
			return true
		}
	case tea.KeyRight:
		if l.pos < len(l.text) {
			l.pos++
			return true
		}
	case tea.KeyHome, tea.KeyCtrlA:
		if l.pos != 0 {
			l.pos = 0
			return true
		}
	case tea.KeyEnd, tea.KeyCtrlE:
		if l.pos != len(l.text) {
			l.pos = len(l.text)
			return true
		}
	}
	return false
}

// insert splices runes at the cursor without aliasing the backing array.
func (l *lineInput) insert(rs []rune) {
	out := make([]rune, 0, len(l.text)+len(rs))
	out = append(out, l.text[:l.pos]...)
	out = append(out, rs...)
	out = append(out, l.text[l.pos:]...)
	l.text = out
	l.pos += len(rs)
}

// view renders the prompt, text and cursor (or the placeholder when empty).
func (l lineInput) view() string {
	if len(l.text) == 0 && l.placeholder != "" {
		return l.prompt + lipgloss.NewStyle().Faint(true).Render(l.placeholder)
	}
	var b strings.Builder
	b.WriteString(l.prompt)
	b.WriteString(string(l.text[:l.pos]))
	if !l.focused {
		b.WriteString(string(l.text[l.pos:]))
		return b.String()
	}
	if l.pos < len(l.text) {
		b.WriteString(lipgloss.NewStyle().Reverse(true).Render(string(l.text[l.pos : l.pos+1])))
		b.WriteString(string(l.text[l.pos+1:]))
	} else {
		b.WriteString(lipgloss.NewStyle().Reverse(true).Render(" "))
	}
	return b.String()
}

// Package tui is the interactive terminal interface of jot: a calendar of
// journal days, a rendered day pane, quick one-line adds and an editor
// handoff, all read-mostly over plain markdown files.
//
// The stable entry point is Run, which blocks until the user quits.
package tui

import (
	"io"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/moneytosms/jot"
)

// Options configures a TUI run. Zero fields fall back to the config file,
// environment and built-in defaults.
type Options struct {
	// Cfg is the resolved jot configuration (root, default journal, editor).
	Cfg jot.Config
	// Journal is the journal to open; empty means Cfg.TUI.Journal when that
	// journal exists, else Cfg.DefaultJournal.
	Journal string
	// Theme is the theme name; empty means JOT_THEME, then Cfg.TUI.Theme,
	// else auto.
	Theme string
	// In replaces stdin for the program (useful in tests). Nil means stdin.
	In io.Reader
	// Out replaces stdout for the program (useful in tests). Nil means
	// stdout.
	Out io.Writer
}

// Run builds the Bubble Tea program and blocks until the user quits.
func Run(opts Options) error {
	j := jot.New(opts.Cfg)
	journal := resolveJournal(j, opts)
	themeName := resolveThemeName(opts)
	theme := Resolve(themeName)

	popts := []tea.ProgramOption{tea.WithAltScreen(), tea.WithMouseCellMotion()}
	if opts.Out != nil {
		popts = append(popts, tea.WithOutput(opts.Out))
	}
	if opts.In != nil {
		popts = append(popts, tea.WithInput(opts.In))
	}

	p := tea.NewProgram(newModel(j, opts.Cfg, journal, themeName, theme), popts...)
	_, err := p.Run()
	return err
}

// resolveJournal picks the journal to open: the flag, then the journal last
// opened in the TUI when it still exists, then the configured default.
func resolveJournal(j *jot.Jot, opts Options) string {
	if journal := strings.TrimSpace(opts.Journal); journal != "" {
		return jot.SanitizeName(journal)
	}
	if journal := strings.TrimSpace(opts.Cfg.TUI.Journal); j.JournalExists(journal) {
		return jot.SanitizeName(journal)
	}
	return jot.SanitizeName(j.DefaultJournal())
}

// resolveThemeName picks the theme: the flag, then JOT_THEME, then the saved
// preference, then auto. The order matches the other JOT_* overrides.
func resolveThemeName(opts Options) string {
	if theme := strings.TrimSpace(opts.Theme); theme != "" {
		return theme
	}
	if theme := strings.TrimSpace(os.Getenv(jot.EnvTheme)); theme != "" {
		return theme
	}
	if theme := strings.TrimSpace(opts.Cfg.TUI.Theme); theme != "" {
		return theme
	}
	return "auto"
}

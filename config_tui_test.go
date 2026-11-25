package jot

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveTUIPreferencesKeepsUserSettings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	t.Setenv(EnvConfig, path)
	t.Setenv(EnvRoot, filepath.Join(dir, "jot"))

	if _, err := SaveConfig(Config{Root: "/tmp/journal", DefaultJournal: "work", Editor: "vim"}); err != nil {
		t.Fatalf("save config: %v", err)
	}

	written, err := SaveTUIPreferences("nord", "ideas")
	if err != nil {
		t.Fatalf("save tui preferences: %v", err)
	}
	if written != path {
		t.Fatalf("wrote %q, want %q", written, path)
	}

	file, err := LoadConfigFile()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if file.Root != "/tmp/journal" || file.DefaultJournal != "work" || file.Editor != "vim" {
		t.Fatalf("user settings were lost: %+v", file)
	}
	if file.TUI.Theme != "nord" || file.TUI.Journal != "ideas" {
		t.Fatalf("tui settings not stored: %+v", file.TUI)
	}

	// The file must stay private and readable as plain JSON.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat config: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("config mode = %v, want 0600", mode)
	}
}
func TestSetupPreservesTUIPreferences(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvConfig, filepath.Join(dir, "config.json"))
	if _, err := SaveTUIPreferences("nord", "work"); err != nil {
		t.Fatalf("SaveTUIPreferences: %v", err)
	}

	no := false
	result, err := Setup(strings.NewReader(""), io.Discard, SetupOptions{
		Root:       filepath.Join(dir, "journals"),
		Journal:    "daily",
		Editor:     "true",
		Yes:        true,
		Install:    &no,
		InstallDir: filepath.Join(dir, "bin"),
	})
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	if result.Config.TUI.Theme != "nord" || result.Config.TUI.Journal != "work" {
		t.Fatalf("TUI preferences dropped: %+v", result.Config.TUI)
	}
	loaded, err := LoadConfigFile()
	if err != nil {
		t.Fatalf("LoadConfigFile: %v", err)
	}
	if loaded.TUI.Theme != "nord" || loaded.TUI.Journal != "work" {
		t.Fatalf("saved TUI preferences = %+v", loaded.TUI)
	}
}

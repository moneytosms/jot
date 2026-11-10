package jot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Environment variables understood by jot.
const (
	// EnvRoot overrides the journal root directory.
	EnvRoot = "JOT_ROOT"
	// EnvJournal sets the default journal name.
	EnvJournal = "JOT_JOURNAL"
	// EnvEditor sets the editor used by `jot edit`.
	EnvEditor = "JOT_EDITOR"
	// EnvConfig points at an alternative config file.
	EnvConfig = "JOT_CONFIG"
	// EnvInstallDir sets the default directory for `jot install`.
	EnvInstallDir = "JOT_INSTALL_DIR"
)

// DefaultJournalName is the journal used when none is named.
const DefaultJournalName = "default"

// ModulePath is the import path of this module.
const ModulePath = "github.com/moneytosms/jot"

// Config controls where journals live, which journal commands default to and
// which editor `jot edit` launches.
//
// Precedence, highest first: command-line flags, JOT_* environment variables,
// the config file, built-in defaults.
type Config struct {
	// Root is the journal root directory.
	Root string `json:"root,omitempty"`
	// DefaultJournal is the journal used when a command names none.
	DefaultJournal string `json:"default_journal,omitempty"`
	// Editor is the command `jot edit` launches.
	Editor string `json:"editor,omitempty"`
}

// DefaultConfig resolves the effective configuration.
func DefaultConfig() (Config, error) {
	cfg := Config{DefaultJournal: DefaultJournalName}

	file, err := LoadConfigFile()
	if err != nil {
		return Config{}, err
	}
	if file.Root != "" {
		cfg.Root = file.Root
	}
	if file.DefaultJournal != "" {
		cfg.DefaultJournal = file.DefaultJournal
	}
	if file.Editor != "" {
		cfg.Editor = file.Editor
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return Config{}, err
	}
	if strings.TrimSpace(cfg.Root) == "" {
		cfg.Root = filepath.Join(home, ".jot")
	}
	cfg.Root = expandHome(cfg.Root, home)

	if v := os.Getenv(EnvRoot); strings.TrimSpace(v) != "" {
		cfg.Root = expandHome(v, home)
	}
	if v := os.Getenv(EnvJournal); strings.TrimSpace(v) != "" {
		cfg.DefaultJournal = v
	}
	if v := os.Getenv(EnvEditor); strings.TrimSpace(v) != "" {
		cfg.Editor = v
	}

	cfg.Root = filepath.Clean(cfg.Root)
	cfg.DefaultJournal = SanitizeName(cfg.DefaultJournal)
	return cfg, nil
}

// ConfigPath returns the path of the config file in use.
func ConfigPath() string {
	if p := strings.TrimSpace(os.Getenv(EnvConfig)); p != "" {
		return p
	}
	dir, err := os.UserConfigDir()
	if err != nil || dir == "" {
		return ""
	}
	return filepath.Join(dir, "jot", "config.json")
}

// LoadConfigFile reads the config file. A missing file yields a zero Config and
// no error; malformed JSON is reported so `jot doctor` can flag it.
func LoadConfigFile() (Config, error) {
	path := ConfigPath()
	if path == "" {
		return Config{}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, nil
		}
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, &ConfigError{Path: path, Err: err}
	}
	return cfg, nil
}

// SaveConfig writes the config file atomically and returns its path.
func SaveConfig(cfg Config) (string, error) {
	path := ConfigPath()
	if path == "" {
		return "", &ConfigError{Err: errNoConfigDir}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", err
	}
	data = append(data, '\n')
	if err := writeFileAtomic(path, data, 0o600); err != nil {
		return "", err
	}
	return path, nil
}

// ConfigError describes a problem with the config file.
type ConfigError struct {
	// Path is the config file that failed, empty if unknown.
	Path string
	// Err is the underlying error.
	Err error
}

// Error implements error.
func (e *ConfigError) Error() string {
	if e.Path == "" {
		return "config: " + e.Err.Error()
	}
	return "config file " + e.Path + ": " + e.Err.Error()
}

// Unwrap returns the underlying error for errors.Is and errors.As.
func (e *ConfigError) Unwrap() error { return e.Err }

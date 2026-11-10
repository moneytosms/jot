package jot

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// SetupOptions drives the first-run wizard.
type SetupOptions struct {
	// Root, Journal and Editor pre-answer the prompts. Empty means ask.
	Root    string
	Journal string
	Editor  string
	// InstallDir overrides the directory the binary is copied into.
	InstallDir string
	// Yes skips every prompt and accepts the defaults.
	Yes bool
	// Install, when non-nil, decides whether to copy the binary.
	Install *bool
}

// SetupResult reports what the wizard did.
type SetupResult struct {
	// Config is the configuration that was written.
	Config Config
	// ConfigPath is the path of the config file.
	ConfigPath string
	// Installed reports whether the binary was copied.
	Installed bool
	// BinaryPath is the installed binary path, empty if not installed.
	BinaryPath string
}

// Setup configures jot interactively: it picks a root directory, a default
// journal and an editor, writes the config file, creates the root and copies
// the binary if asked. Without a terminal (or with Yes) it uses defaults.
func Setup(in io.Reader, out io.Writer, opts SetupOptions) (SetupResult, error) {
	cfg, err := DefaultConfig()
	if err != nil {
		return SetupResult{}, err
	}

	interactive := !opts.Yes && IsTerminal(in)
	reader := bufio.NewReader(in)
	ask := func(label, def string) string {
		if !interactive {
			return def
		}
		if def != "" {
			fmt.Fprintf(out, "%s [%s]: ", label, def)
		} else {
			fmt.Fprintf(out, "%s: ", label)
		}
		line, err := reader.ReadString('\n')
		if err != nil && line == "" {
			fmt.Fprintln(out)
			return def
		}
		line = strings.TrimSpace(line)
		if line == "" {
			return def
		}
		return line
	}
	askBool := func(label string, def bool) bool {
		if !interactive {
			return def
		}
		hint := "[Y/n]"
		if !def {
			hint = "[y/N]"
		}
		answer := strings.ToLower(ask(label+" "+hint, ""))
		switch answer {
		case "":
			return def
		case "y", "yes":
			return true
		default:
			return false
		}
	}

	root := opts.Root
	if strings.TrimSpace(root) == "" {
		root = ask("Journal root", cfg.Root)
	}
	journal := opts.Journal
	if strings.TrimSpace(journal) == "" {
		journal = ask("Default journal", cfg.DefaultJournal)
	}
	journal = SanitizeName(journal)

	editor := opts.Editor
	if strings.TrimSpace(editor) == "" {
		def := cfg.Editor
		if def == "" {
			def = DetectEditor()
		}
		if def == "" {
			def = "vi"
		}
		editor = ask("Editor", def)
	}

	installDir := opts.InstallDir
	if strings.TrimSpace(installDir) == "" {
		installDir = DefaultInstallDir()
	}
	install := true
	if opts.Install != nil {
		install = *opts.Install
	} else {
		if !interactive {
			install = true
		} else {
			install = askBool(fmt.Sprintf("Install the jot binary into %s?", installDir), true)
		}
	}

	cfg = Config{
		Root:           root,
		DefaultJournal: journal,
		Editor:         editor,
	}
	if _, err := os.UserHomeDir(); err == nil {
		cfg.Root = filepath.Clean(expandHome(cfg.Root, mustHome()))
	}

	configPath, err := SaveConfig(cfg)
	if err != nil {
		return SetupResult{}, err
	}

	j := New(cfg)
	if err := j.Init(); err != nil {
		return SetupResult{}, err
	}

	result := SetupResult{Config: cfg, ConfigPath: configPath}
	if install {
		path, installed, err := j.Install(installDir)
		if err != nil {
			return result, err
		}
		result.BinaryPath = path
		result.Installed = installed
	}
	return result, nil
}

// IsTerminal reports whether r is an interactive terminal.
func IsTerminal(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	st, err := f.Stat()
	if err != nil {
		return false
	}
	return st.Mode()&os.ModeCharDevice != 0
}

func mustHome() string {
	home, _ := os.UserHomeDir()
	return home
}

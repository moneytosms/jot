// Command jot is a small, fast journaling CLI.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/moneytosms/jot"
	"github.com/moneytosms/jot/internal/tui"
)

// version is overridden at build time:
//
//	go build -ldflags "-X main.version=v1.2.3" ./cmd/jot
var version = "0.0.0-dev"

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, in io.Reader, out, errw io.Writer) int {
	a := &app{in: in, out: out, errw: errw}
	if len(args) == 0 {
		// In a real terminal, bare jot is the interactive interface; anywhere
		// else (pipes, redirects, tests) it stays the help page.
		if interactive(in, out) {
			if err := a.runTUI("", ""); err != nil {
				fmt.Fprintf(errw, "jot: %v\n", err)
				return 1
			}
			return 0
		}
		printHelp(out)
		return 0
	}

	switch args[0] {
	case "-h", "--help", "help":
		if len(args) > 1 {
			if err := printCommandHelp(args[1], out); err != nil {
				fmt.Fprintf(errw, "jot: %v\n", err)
				return 1
			}
			return 0
		}
		printHelp(out)
		return 0
	case "-v", "--version", "version":
		fmt.Fprintf(out, "jot %s\n", version)
		return 0
	}

	var err error
	switch args[0] {
	case "tui":
		err = a.tuiCmd(args[1:])
	case "init":
		err = a.initCmd(args[1:])
	case "add":
		err = a.addCmd(args[1:])
	case "show":
		err = a.showCmd(args[1:])
	case "edit":
		err = a.editCmd(args[1:])
	case "ls":
		err = a.lsCmd(args[1:])
	case "search":
		err = a.searchCmd(args[1:])
	case "todo":
		err = a.todoCmd(args[1:])
	case "watch":
		err = a.watchCmd(args[1:])
	case "remind":
		err = a.remindCmd(args[1:])
	case "backup":
		err = a.backupCmd(args[1:])
	case "export":
		err = a.exportCmd(args[1:])
	case "journal":
		err = a.journalCmd(args[1:])
	case "install":
		err = a.installCmd(args[1:])
	case "update":
		err = a.updateCmd(args[1:])
	case "uninstall":
		err = a.uninstallCmd(args[1:])
	case "doctor":
		err = a.doctorCmd(args[1:])
	case "setup":
		err = a.setupCmd(args[1:])
	default:
		err = fmt.Errorf("unknown command %q; run `jot help`", args[0])
	}

	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		if errors.Is(err, errUsage) {
			return 2
		}
		fmt.Fprintf(errw, "jot: %v\n", err)
		return 1
	}
	return 0
}

// errUsage marks a usage error whose help page has already been printed. run
// turns it into exit status 2, the convention the flag package follows.
var errUsage = errors.New("usage error")

// plural returns one for n == 1, otherwise many. It keeps summary lines
// grammatical without a template engine.
func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// app carries the streams a command writes to, which keeps the CLI testable.
type app struct {
	in   io.Reader
	out  io.Writer
	errw io.Writer
}

// loadConfig resolves the effective configuration.
func (a *app) loadConfig() (jot.Config, error) {
	return jot.DefaultConfig()
}

// loadJot resolves configuration and returns a ready handle.
func (a *app) loadJot() (*jot.Jot, error) {
	cfg, err := a.loadConfig()
	if err != nil {
		return nil, err
	}
	return jot.New(cfg), nil
}

// tuiCmd parses the tui flags and hands over to the interactive interface.
func (a *app) tuiCmd(args []string) error {
	fs := a.flags("tui")
	journal := journalFlag(fs)
	theme := fs.String("theme", "", "colour theme name")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return err
		}
		return errUsage // the flag set already printed the help page
	}
	if fs.NArg() > 0 {
		_ = printCommandHelp("tui", a.out)
		return errUsage
	}
	return a.runTUI(*journal, *theme)
}

// runTUI refuses non-terminal streams, then blocks inside the interface.
func (a *app) runTUI(journal, theme string) error {
	if !interactive(a.in, a.out) {
		return errors.New("jot tui needs an interactive terminal; run jot help for the command line")
	}
	cfg, err := a.loadConfig()
	if err != nil {
		return err
	}
	return tui.Run(tui.Options{
		Cfg:     cfg,
		Journal: journal,
		Theme:   theme,
		In:      a.in,
		Out:     a.out,
	})
}

// flags returns a flag set that prints jot's own help page for -h/--help
// instead of Go's generated usage, and keeps parse errors quiet because run
// reports them itself.
func (a *app) flags(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() { _ = printCommandHelp(name, a.out) }
	return fs
}

func journalFlag(fs *flag.FlagSet) *string {
	v := fs.String("journal", "", "journal to use")
	fs.StringVar(v, "j", "", "shorthand for -journal")
	return v
}

func dateFlag(fs *flag.FlagSet) *string {
	v := fs.String("date", "", "day: today, yesterday or YYYY-MM-DD")
	fs.StringVar(v, "d", "", "shorthand for -date")
	return v
}

// pickJournal resolves an explicit journal name, falling back to the default.
func pickJournal(j *jot.Jot, explicit string) string {
	if strings.TrimSpace(explicit) != "" {
		return jot.SanitizeName(explicit)
	}
	return j.DefaultJournal()
}

// noJournalErr builds the hint shown when a journal has no data yet.
func noJournalErr(name string) error {
	return fmt.Errorf("no journal named %q yet; add an entry with `jot add -j %s \"...\"`", name, name)
}

// stdinPiped reports whether stdin may hold piped text. Anything that is not a
// terminal counts, so scripts and tests are read rather than refused.
func stdinPiped(in io.Reader) bool {
	f, ok := in.(*os.File)
	if !ok {
		return true
	}
	return !jot.IsTerminal(f)
}

// interactive reports whether both streams are real terminals, so the TUI
// only starts where it can read keys and draw. It never looks at TERM.
func interactive(in io.Reader, out io.Writer) bool {
	return terminalFile(in) && terminalFile(out)
}

// terminalFile reports whether v is an *os.File on a character device.
// jot.IsTerminal holds the actual check so the CLI has one definition.
func terminalFile(v any) bool {
	f, ok := v.(*os.File)
	if !ok {
		return false
	}
	return jot.IsTerminal(f)
}

func readBody(r io.Reader) (string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// onPath reports whether dir is part of PATH.
func onPath(dir string) bool {
	target := strings.TrimSpace(dir)
	if target == "" {
		return false
	}
	if abs, err := filepath.Abs(target); err == nil {
		target = abs
	}
	for _, entry := range strings.Split(os.Getenv("PATH"), string(os.PathListSeparator)) {
		if strings.TrimSpace(entry) == "" {
			continue
		}
		if abs, err := filepath.Abs(entry); err == nil && abs == target {
			return true
		}
	}
	return false
}

// newTable returns a tabwriter with aligned columns.
func newTable(w io.Writer) *tabwriter.Writer {
	return tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for n/div >= unit && exp < 3 {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

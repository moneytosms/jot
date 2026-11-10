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
)

// version is overridden at build time:
//
//	go build -ldflags "-X main.version=v1.2.3" ./cmd/jot
var version = "0.0.0-dev"

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, in io.Reader, out, errw io.Writer) int {
	if len(args) == 0 {
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

	a := &app{in: in, out: out, errw: errw}
	var err error
	switch args[0] {
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
	case "watch":
		err = a.watchCmd(args[1:])
	case "backup":
		err = a.backupCmd(args[1:])
	case "export":
		err = a.exportCmd(args[1:])
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
		fmt.Fprintf(errw, "jot: %v\n", err)
		return 1
	}
	return 0
}

// app carries the streams a command writes to, which keeps the CLI testable.
type app struct {
	in   io.Reader
	out  io.Writer
	errw io.Writer
}

// loadJot resolves configuration and returns a ready handle.
func (a *app) loadJot() (*jot.Jot, error) {
	cfg, err := jot.DefaultConfig()
	if err != nil {
		return nil, err
	}
	return jot.New(cfg), nil
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

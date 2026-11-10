package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/moneytosms/jot"
)

func (a *app) watchCmd(args []string) error {
	fs := a.flags("watch")
	journal := journalFlag(fs)
	poll := fs.Duration("poll", 750*time.Millisecond, "how often to check for new text")
	if err := fs.Parse(args); err != nil {
		return err
	}

	j, err := a.loadJot()
	if err != nil {
		return err
	}
	name := pickJournal(j, *journal)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	fmt.Fprintf(a.out, "watching %s (Ctrl+C to stop)\n", filepath.Join(j.Root(), name)+string(os.PathSeparator))
	if err := j.Watch(ctx, name, a.out, *poll); err != nil {
		if errors.Is(err, jot.ErrNoJournal) {
			return noJournalErr(name)
		}
		return err
	}
	fmt.Fprintln(a.out, "stopped")
	return nil
}

func (a *app) installCmd(args []string) error {
	fs := a.flags("install")
	dir := fs.String("dir", "", "destination directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		*dir = fs.Arg(0)
	}

	j, err := a.loadJot()
	if err != nil {
		return err
	}
	path, installed, err := j.Install(*dir)
	if err != nil {
		return err
	}
	if installed {
		fmt.Fprintf(a.out, "installed jot to %s\n", path)
	} else {
		fmt.Fprintf(a.out, "jot is already installed at %s\n", path)
	}

	binDir := filepath.Dir(path)
	if onPath(binDir) {
		fmt.Fprintf(a.out, "%s is on your PATH; run `jot doctor` to confirm\n", binDir)
		return nil
	}
	fmt.Fprintf(a.out, "\n%s is not on your PATH. Add it, for example:\n", binDir)
	fmt.Fprintf(a.out, "  echo 'export PATH=\"%s:$PATH\"' >> ~/.bashrc && source ~/.bashrc\n", binDir)
	fmt.Fprintf(a.out, "(use ~/.zshrc on zsh, ~/.config/fish/config.fish on fish)\n")
	return nil
}

func (a *app) updateCmd(args []string) error {
	fs := a.flags("update")
	src := fs.String("src", "", "source checkout to build from")
	dir := fs.String("dir", "", "install directory")
	if err := fs.Parse(args); err != nil {
		return err
	}

	j, err := a.loadJot()
	if err != nil {
		return err
	}
	path, err := j.Update(*src, *dir)
	if err != nil {
		return err
	}
	fmt.Fprintf(a.out, "rebuilt and installed %s\n", path)

	// Prove the fresh binary actually runs.
	if out, err := exec.Command(path, "version").Output(); err == nil {
		fmt.Fprintf(a.out, "%s\n", strings.TrimSpace(string(out)))
	}
	return nil
}

func (a *app) uninstallCmd(args []string) error {
	fs := a.flags("uninstall")
	yes := fs.Bool("yes", false, "do not ask for confirmation")
	fs.BoolVar(yes, "y", false, "shorthand for -yes")
	if err := fs.Parse(args); err != nil {
		return err
	}

	j, err := a.loadJot()
	if err != nil {
		return err
	}
	if !*yes {
		if !jot.IsTerminal(a.in) {
			return errors.New("refusing to delete journal data without confirmation; pass --yes")
		}
		fmt.Fprintf(a.out, "Delete %s and every entry in it? [y/N] ", j.Root())
		line, _ := bufio.NewReader(a.in).ReadString('\n')
		switch strings.ToLower(strings.TrimSpace(line)) {
		case "y", "yes":
		default:
			fmt.Fprintln(a.out, "aborted")
			return nil
		}
	}
	if err := j.Uninstall(); err != nil {
		return err
	}
	fmt.Fprintf(a.out, "deleted %s\n", j.Root())
	fmt.Fprintln(a.out, "the jot binary was left in place; remove it with: rm \"$(command -v jot)\"")
	return nil
}

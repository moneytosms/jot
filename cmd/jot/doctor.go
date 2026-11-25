package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/moneytosms/jot"
)

func (a *app) doctorCmd(args []string) error {
	fs := a.flags("doctor")
	if err := fs.Parse(args); err != nil {
		return err
	}
	j, err := a.loadJot()
	if err != nil {
		return err
	}

	fmt.Fprintf(a.out, "jot %s\n\n", version)
	checks := append(j.Doctor(), cliDoctorChecks()...)
	failures := 0
	tw := newTable(a.out)
	for _, c := range checks {
		label := map[string]string{
			"ok":   "[ok]",
			"warn": "[warn]",
			"fail": "[fail]",
		}[c.Status]
		if label == "" {
			label = "[" + c.Status + "]"
		}
		if c.Status == "fail" {
			failures++
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\n", label, c.Name, c.Detail)
	}
	tw.Flush()
	if failures > 0 {
		return fmt.Errorf("%d check(s) failed", failures)
	}
	return nil
}

// cliDoctorChecks adds CLI-side checks after the library checks, in the same
// table style. They only ever warn: reminders and helper binaries are
// optional, so a warning must not fail the command.
func cliDoctorChecks() []jot.Check {
	var checks []jot.Check
	if jot.RemindersSupported() {
		how := "systemd-run + notify-send"
		if execLookPath("omarchy") {
			how = "omarchy"
		}
		checks = append(checks, jot.Check{Name: "reminders", Status: "ok", Detail: "available via " + how})
	} else {
		checks = append(checks, jot.Check{Name: "reminders", Status: "warn", Detail: "unavailable: need omarchy, or systemd-run + notify-send"})
	}
	for _, bin := range []string{"omarchy", "notify-send", "systemd-run"} {
		if execLookPath(bin) {
			checks = append(checks, jot.Check{Name: bin, Status: "ok", Detail: "found on PATH"})
		} else {
			checks = append(checks, jot.Check{Name: bin, Status: "warn", Detail: "not on PATH (optional)"})
		}
	}
	if st, err := os.Stat("vendor"); err == nil && st.IsDir() {
		checks = append(checks, jot.Check{Name: "vendor", Status: "ok", Detail: "vendor/ directory present"})
	} else {
		checks = append(checks, jot.Check{Name: "vendor", Status: "warn", Detail: "no vendor/ directory"})
	}
	return checks
}

// execLookPath reports whether name is an executable on PATH.
func execLookPath(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func (a *app) setupCmd(args []string) error {
	fs := a.flags("setup")
	root := fs.String("root", "", "journal root")
	journal := fs.String("journal", "", "default journal name")
	editor := fs.String("editor", "", "editor command")
	installDir := fs.String("install-dir", "", "directory to copy the binary into")
	noInstall := fs.Bool("no-install", false, "do not copy the binary")
	install := fs.Bool("install", false, "copy the binary without asking")
	yes := fs.Bool("yes", false, "accept defaults without prompting")
	fs.BoolVar(yes, "y", false, "shorthand for -yes")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *noInstall && *install {
		return errors.New("use only one of --install and --no-install")
	}
	var installFlag *bool
	if *noInstall {
		v := false
		installFlag = &v
	} else if *install {
		v := true
		installFlag = &v
	}

	fmt.Fprintln(a.out, "jot setup: a few questions, then you are journalling")
	result, err := jot.Setup(a.in, a.out, jot.SetupOptions{
		Root:       *root,
		Journal:    *journal,
		Editor:     *editor,
		InstallDir: *installDir,
		Yes:        *yes,
		Install:    installFlag,
	})
	if err != nil {
		return err
	}

	fmt.Fprintln(a.out)
	fmt.Fprintf(a.out, "config:  %s\n", result.ConfigPath)
	fmt.Fprintf(a.out, "journals: %s\n", result.Config.Root)
	fmt.Fprintf(a.out, "default:  %s\n", result.Config.DefaultJournal)
	if result.BinaryPath != "" {
		status := "already there"
		if result.Installed {
			status = "installed"
		}
		fmt.Fprintf(a.out, "binary:   %s (%s)\n", result.BinaryPath, status)
	}

	fmt.Fprintln(a.out)
	fmt.Fprintln(a.out, "Try it:")
	fmt.Fprintln(a.out, "  jot add \"first entry\"")
	fmt.Fprintln(a.out, "  jot show")
	fmt.Fprintln(a.out, "  jot doctor")
	if result.BinaryPath != "" && !onPath(filepath.Dir(result.BinaryPath)) {
		fmt.Fprintln(a.out)
		fmt.Fprintln(a.out, "Heads up: the bin directory is not on your PATH yet.")
	}
	return nil
}

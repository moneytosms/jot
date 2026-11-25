package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/moneytosms/jot"
)

// runningBinaryPath and removeRunningBinary are seams that keep the
// destructive binary removal testable without deleting the test binary.
var (
	runningBinaryPath   = jot.RunningBinaryPath
	removeRunningBinary = func(j *jot.Jot) (string, error) { return j.RemoveBinary() }
)

// uninstallUsage is shown when there is no terminal to ask and no component
// was named: silently guessing what to delete would be unsafe.
const uninstallUsage = "refusing to remove anything without an explicit component; pass --data, --binary, --config or --all" +
	" (add --yes for scripts)\n  jot uninstall --data --yes\n  jot uninstall --binary\n  jot uninstall --all --yes"

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

// updateStatus is the object printed by `jot update --json`.
type updateStatus struct {
	Current         string `json:"current"`
	Latest          string `json:"latest"`
	UpdateAvailable bool   `json:"update_available"`
	Released        string `json:"released"`
	URL             string `json:"url"`
}

// updateCmd checks the published releases and installs the platform binary.
// It keeps the old source rebuild path behind --source.
func (a *app) updateCmd(args []string) error {
	fs := a.flags("update")
	check := fs.Bool("check", false, "report the available release without installing")
	source := fs.Bool("source", false, "rebuild from a local source checkout")
	src := fs.String("src", "", "source checkout to build from")
	dir := fs.String("dir", "", "install directory")
	pin := fs.String("version", "", "install a specific release tag")
	yes := fs.Bool("yes", false, "install without asking")
	fs.BoolVar(yes, "y", false, "shorthand for -yes")
	notes := fs.Bool("notes", false, "print the full release notes")
	asJSON := fs.Bool("json", false, "print a machine-readable status object")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected argument %q; see `jot help update`", fs.Arg(0))
	}

	if *source {
		if *check || *asJSON {
			return errors.New("--source cannot be combined with --check or --json")
		}
		if strings.TrimSpace(*pin) != "" {
			return errors.New("--version cannot be combined with --source")
		}
		return a.updateFromSource(*src, *dir)
	}

	current := version
	release, err := jot.FetchRelease(current, *pin)
	if err != nil {
		return updateFetchError(err)
	}
	available := jot.UpdateAvailable(current, release)

	if *asJSON {
		status := updateStatus{
			Current:         displayVersion(current),
			Latest:          displayVersion(release.Tag),
			UpdateAvailable: available,
			Released:        releaseDate(release),
			URL:             platformAssetURL(release),
		}
		data, err := json.Marshal(status)
		if err != nil {
			return err
		}
		fmt.Fprintf(a.out, "%s\n", data)
		return nil
	}

	if !available {
		if date := releaseDate(release); date != "" {
			fmt.Fprintf(a.out, "<-- up to date: jot %s (released %s)\n", displayVersion(release.Tag), date)
		} else {
			fmt.Fprintf(a.out, "<-- up to date: jot %s\n", displayVersion(release.Tag))
		}
		return nil
	}

	if date := releaseDate(release); date != "" {
		fmt.Fprintf(a.out, "update available: %s -> %s (released %s)\n", displayVersion(current), displayVersion(release.Tag), date)
	} else {
		fmt.Fprintf(a.out, "update available: %s -> %s\n", displayVersion(current), displayVersion(release.Tag))
	}
	printReleaseNotes(a.out, release.Notes, *notes)

	if *check {
		return nil
	}
	if !*yes {
		// Never hang on a pipe: a check that cannot ask is still a success.
		if !jot.IsTerminal(a.in) {
			fmt.Fprintln(a.out, "re-run with --yes to install")
			return nil
		}
		fmt.Fprint(a.out, "Update now? [Y/n] ")
		answer, err := readLine(a.in)
		if err != nil {
			return err
		}
		switch strings.ToLower(strings.TrimSpace(answer)) {
		case "", "y", "yes":
		default:
			fmt.Fprintln(a.out, "aborted")
			return nil
		}
	}

	j, err := a.loadJot()
	if err != nil {
		return err
	}
	path, err := j.UpdateFromRelease(current, *pin, *dir, a.out)
	if err != nil {
		return updateInstallError(err)
	}
	installed := displayVersion(release.Tag)
	fmt.Fprintf(a.out, "installed jot %s at %s\n", installed, path)
	printChangeRecap(a.out, installed, release.Notes)
	fmt.Fprintln(a.out, "restart jot (or the TUI) to use it")

	// Prove the fresh binary runs, but never fail the update because of it.
	if out, err := exec.Command(path, "version").Output(); err == nil {
		fmt.Fprintf(a.out, "%s\n", strings.TrimSpace(string(out)))
	}
	return nil
}

// updateFromSource keeps the original rebuild-from-checkout behaviour.
func (a *app) updateFromSource(src, dir string) error {
	j, err := a.loadJot()
	if err != nil {
		return err
	}
	path, err := j.Update(src, dir)
	if err != nil {
		return err
	}
	fmt.Fprintf(a.out, "rebuilt and installed %s\n", path)
	if out, err := exec.Command(path, "version").Output(); err == nil {
		fmt.Fprintf(a.out, "%s\n", strings.TrimSpace(string(out)))
	}
	return nil
}

// displayVersion keeps the "v" prefix consistent in status lines.
func displayVersion(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "(unknown)"
	}
	if strings.HasPrefix(v, "v") || strings.HasPrefix(v, "V") {
		return v
	}
	return "v" + v
}

// releaseDate formats a release date, or an empty string when unknown.
func releaseDate(release jot.ReleaseInfo) string {
	if release.Published.IsZero() {
		return ""
	}
	return release.Published.Format("2006-01-02")
}

// platformAssetURL is the download URL of this platform, for --json.
func platformAssetURL(release jot.ReleaseInfo) string {
	for _, asset := range release.Assets {
		if asset.Name == jot.PlatformAssetName() {
			return asset.URL
		}
	}
	return ""
}

// updateFetchError turns release API failures into actionable messages.
func updateFetchError(err error) error {
	var status *jot.HTTPStatusError
	if errors.As(err, &status) {
		if status.StatusCode == http.StatusNotFound {
			return fmt.Errorf("no releases published yet (the release API returned 404 for %s); see https://github.com/moneytosms/jot/releases", status.URL)
		}
		return fmt.Errorf("release API returned %s for %s; try again later", status.Status, status.URL)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("the release API timed out; check your network connection: %w", err)
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return fmt.Errorf("cannot reach the release API at %s: %w\ncheck your network connection, or set %s to a mirror", urlErr.URL, urlErr.Err, jot.EnvReleasesAPI)
	}
	return err
}

// updateInstallError adds a hint when the install directory is unwritable.
func updateInstallError(err error) error {
	if errors.Is(err, os.ErrPermission) {
		return fmt.Errorf("%w\nhint: choose a writable directory with --dir or run with sudo", err)
	}
	return err
}

// Release notes are wrapped to fit a terminal and kept short by default.
const (
	notesWidth    = 76
	notesMaxLines = 15
	notesMaxChars = 1200
	recapLines    = 4
)

// printReleaseNotes writes a readable "What's new" block. full keeps every
// line; otherwise the notes are cut to the first 15 lines or 1200 characters.
func printReleaseNotes(w io.Writer, notes string, full bool) {
	lines, truncated := releaseNoteLines(notes, full)
	if len(lines) == 0 {
		return
	}
	fmt.Fprintln(w, "What's new:")
	for _, line := range lines {
		fmt.Fprintf(w, "  %s\n", line)
	}
	if truncated {
		fmt.Fprintln(w, "  ... (truncated; pass --notes for the full notes)")
	}
}

// printChangeRecap writes a few lines of the same notes after an install.
func printChangeRecap(w io.Writer, version, notes string) {
	lines, truncated := releaseNoteLines(notes, false)
	if len(lines) > recapLines {
		lines = lines[:recapLines]
		truncated = true
	}
	if len(lines) == 0 {
		fmt.Fprintf(w, "<- what changed in %s: no release notes\n", version)
		return
	}
	fmt.Fprintf(w, "<- what changed in %s:\n", version)
	for _, line := range lines {
		fmt.Fprintf(w, "  %s\n", line)
	}
	if truncated {
		fmt.Fprintln(w, "  ...")
	}
}

// releaseNoteLines trims, wraps and (unless full) truncates release notes.
func releaseNoteLines(notes string, full bool) ([]string, bool) {
	notes = strings.TrimSpace(strings.ReplaceAll(notes, "\r\n", "\n"))
	if notes == "" {
		return nil, false
	}
	truncated := false
	if !full {
		raw := strings.Split(notes, "\n")
		if len(raw) > notesMaxLines {
			raw = raw[:notesMaxLines]
			truncated = true
		}
		notes = strings.Join(raw, "\n")
		if runes := []rune(notes); len(runes) > notesMaxChars {
			notes = string(runes[:notesMaxChars])
			truncated = true
		}
	}
	var lines []string
	for _, line := range strings.Split(notes, "\n") {
		line = strings.TrimRight(line, " \t")
		if strings.TrimSpace(line) == "" {
			lines = append(lines, "")
			continue
		}
		lines = append(lines, wrapNoteLine(line, notesWidth)...)
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines, truncated
}

// wrapNoteLine word-wraps one markdown line, keeping a list marker on the
// first line and aligning continuation lines under its text.
func wrapNoteLine(line string, width int) []string {
	trimmed := strings.TrimLeft(line, " \t")
	indent := line[:len(line)-len(trimmed)]
	marker := ""
	if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") || strings.HasPrefix(trimmed, "+ ") {
		marker = trimmed[:2]
		trimmed = trimmed[2:]
	}
	words := strings.Fields(trimmed)
	if len(words) == 0 {
		if marker != "" {
			return []string{indent + marker}
		}
		return []string{""}
	}
	firstPrefix := indent + marker
	continuation := indent + strings.Repeat(" ", len(marker))
	current := firstPrefix
	var out []string
	for _, word := range words {
		runes := []rune(word)
		for len(runes) > width {
			// Flush any pending text, then hard-break the oversized word.
			if strings.TrimSpace(current) != "" {
				out = append(out, strings.TrimRight(current, " "))
				current = continuation
			}
			out = append(out, strings.TrimRight(current+string(runes[:width]), " "))
			current = continuation
			runes = runes[width:]
		}
		if len(runes) == 0 {
			continue
		}
		word = string(runes)
		if strings.TrimSpace(current) == "" {
			current += word
			continue
		}
		if len([]rune(current))+1+len(word) <= width {
			current += " " + word
			continue
		}
		out = append(out, strings.TrimRight(current, " "))
		current = continuation + word
	}
	if strings.TrimSpace(current) != "" {
		out = append(out, strings.TrimRight(current, " "))
	}
	return out
}

// uninstallCmd removes the components the user selected, in a safe order and
// with an honest report of what happened.
func (a *app) uninstallCmd(args []string) error {
	fs := a.flags("uninstall")
	data := fs.Bool("data", false, "remove the journal root, including backups")
	binary := fs.Bool("binary", false, "remove the running jot executable")
	config := fs.Bool("config", false, "remove the config file")
	all := fs.Bool("all", false, "remove data, binary and config")
	yes := fs.Bool("yes", false, "do not ask for confirmation")
	fs.BoolVar(yes, "y", false, "shorthand for -yes")
	dryRun := fs.Bool("dry-run", false, "print what would be removed, remove nothing")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected argument %q; see `jot help uninstall`", fs.Arg(0))
	}
	if *all {
		*data, *binary, *config = true, true, true
	}

	j, err := a.loadJot()
	if err != nil {
		return err
	}
	configPath := jot.ConfigPath()

	// One buffered reader keeps multi-step prompts from swallowing each
	// other's buffered input.
	reader := bufio.NewReader(a.in)
	readAnswer := func() (string, error) {
		line, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				return line, nil
			}
			return "", err
		}
		return line, nil
	}

	if !*data && !*binary && !*config {
		switch {
		case *yes:
			// Backwards compatibility: `jot uninstall --yes` was data only.
			*data = true
		case !jot.IsTerminal(a.in):
			return errors.New(uninstallUsage)
		default:
			a.printUninstallInventory(j, configPath)
			fmt.Fprint(a.out, "remove [d]ata, [b]inary, [c]onfig, [a]ll, or [q]uit? ")
			answer, err := readAnswer()
			if err != nil {
				return err
			}
			switch strings.ToLower(strings.TrimSpace(answer)) {
			case "d", "data":
				*data = true
			case "b", "binary":
				*binary = true
			case "c", "config":
				*config = true
			case "a", "all":
				*data, *binary, *config = true, true, true
			case "", "q", "quit":
				fmt.Fprintln(a.out, "nothing removed")
				return nil
			default:
				return fmt.Errorf("unknown choice %q; nothing removed", strings.TrimSpace(answer))
			}
		}
	}

	if *dryRun {
		a.printUninstallPlan(j, configPath, *data, *binary, *config)
		return nil
	}

	doData, doBinary, doConfig := *data, *binary, *config
	if doData && !*yes {
		if !jot.IsTerminal(a.in) {
			return errors.New("refusing to delete journal data without confirmation; re-run with --yes")
		}
		fmt.Fprintf(a.out, "warning: this deletes every entry and backup under %s\n", j.Root())
		fmt.Fprint(a.out, "type delete to confirm: ")
		answer, err := readAnswer()
		if err != nil {
			return err
		}
		if strings.ToLower(strings.TrimSpace(answer)) != "delete" {
			fmt.Fprintln(a.out, "aborted; nothing was removed")
			return nil
		}
	}
	if (doBinary || doConfig) && !*yes {
		if !jot.IsTerminal(a.in) {
			return errors.New("refusing to remove files without confirmation; re-run with --yes")
		}
		if doConfig {
			doConfig = a.confirm(reader, "Remove the config file? [y/N] ")
		}
		if doBinary {
			doBinary = a.confirm(reader, "Remove the jot binary? [y/N] ")
		}
	}

	var removed, left, skipped []string
	remove := func(label, path string, exists bool, removeFn func() error) {
		if path == "" {
			skipped = append(skipped, label)
			fmt.Fprintf(a.out, "<- skipped %s (no path)\n", label)
			return
		}
		if !exists {
			skipped = append(skipped, label)
			fmt.Fprintf(a.out, "<- skipped %s (not found)\n", path)
			return
		}
		if err := removeFn(); err != nil {
			fmt.Fprintf(a.errw, "jot: could not remove %s: %v\n", path, err)
			left = append(left, fmt.Sprintf("%s (%s)", label, path))
			return
		}
		fmt.Fprintf(a.out, "<- removed %s\n", path)
		removed = append(removed, fmt.Sprintf("%s (%s)", label, path))
	}

	// Config first, data next, and the running binary last.
	if doConfig {
		exists := false
		if configPath != "" {
			if _, err := os.Stat(configPath); err == nil {
				exists = true
			}
		}
		remove("config", configPath, exists, func() error { return os.Remove(configPath) })
	} else if *config {
		left = append(left, "config (kept)")
	}

	if doData {
		exists := false
		if _, err := os.Stat(j.Root()); err == nil {
			exists = true
		}
		remove("data", j.Root(), exists, j.Uninstall)
	} else if *data {
		left = append(left, "data (kept)")
	}

	binaryExists := false
	binaryRemoved := false
	binaryPath, binaryErr := runningBinaryPath()
	if binaryErr == nil {
		if _, err := os.Stat(binaryPath); err == nil {
			binaryExists = true
		}
	}
	if doBinary {
		if binaryErr != nil {
			fmt.Fprintf(a.errw, "jot: cannot locate the binary: %v\n", binaryErr)
			left = append(left, "binary (unknown path)")
		} else {
			remove("binary", binaryPath, binaryExists, func() error {
				if _, err := removeRunningBinary(j); err != nil {
					return err
				}
				binaryRemoved = true
				return nil
			})
		}
	} else if *binary {
		left = append(left, "binary (kept)")
	}

	if len(removed) == 0 {
		fmt.Fprintln(a.out, "summary: removed nothing")
	} else {
		verb := "removed"
		if doData {
			for _, entry := range removed {
				if strings.HasPrefix(entry, "data (") {
					verb = "deleted"
				}
			}
		}
		fmt.Fprintf(a.out, "summary: %s %s\n", verb, strings.Join(removed, ", "))
	}
	if len(left) > 0 {
		fmt.Fprintf(a.out, "summary: left %s\n", strings.Join(left, ", "))
	}
	if len(skipped) > 0 {
		fmt.Fprintf(a.out, "summary: skipped %s\n", strings.Join(skipped, ", "))
	}
	if (!binaryRemoved && binaryExists) || (doBinary && binaryErr != nil) {
		fmt.Fprintln(a.out, "to remove the binary: rm \"$(command -v jot)\"")
	}
	return nil
}

// confirm asks a yes/no question. An empty or unreadable answer means no.
func (a *app) confirm(reader *bufio.Reader, question string) bool {
	fmt.Fprint(a.out, question)
	answer, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "y", "yes":
		return true
	}
	return false
}

// printUninstallInventory shows what exists before the interactive menu.
func (a *app) printUninstallInventory(j *jot.Jot, configPath string) {
	root := j.Root()
	if info, err := os.Stat(root); err == nil && info.IsDir() {
		journals, days, size := journalDataStats(j)
		fmt.Fprintf(a.out, "data:   %s (%d %s, %d %s, %s)\n", root,
			journals, plural(journals, "journal", "journals"),
			days, plural(days, "day", "days"), humanBytes(size))
	} else {
		fmt.Fprintf(a.out, "data:   %s (not found)\n", root)
	}
	if path, err := runningBinaryPath(); err == nil {
		fmt.Fprintf(a.out, "binary: %s\n", path)
	} else {
		fmt.Fprintf(a.out, "binary: (cannot resolve: %v)\n", err)
	}
	switch {
	case configPath == "":
		fmt.Fprintln(a.out, "config: (cannot determine path)")
	default:
		if _, err := os.Stat(configPath); err == nil {
			fmt.Fprintf(a.out, "config: %s\n", configPath)
		} else {
			fmt.Fprintf(a.out, "config: %s (not found)\n", configPath)
		}
	}
}

// printUninstallPlan reports what a dry run would remove.
func (a *app) printUninstallPlan(j *jot.Jot, configPath string, data, binary, config bool) {
	fmt.Fprintln(a.out, "dry run: no files will be removed")
	if config {
		planRemoval(a.out, configPath)
	}
	if data {
		planRemoval(a.out, j.Root())
	}
	if binary {
		if path, err := runningBinaryPath(); err == nil {
			planRemoval(a.out, path)
		} else {
			fmt.Fprintf(a.out, "<- would skip binary (cannot resolve: %v)\n", err)
		}
	}
}

// planRemoval prints one dry-run line for a path.
func planRemoval(w io.Writer, path string) {
	if path == "" {
		fmt.Fprintln(w, "<- would skip (no path)")
		return
	}
	if _, err := os.Stat(path); err != nil {
		fmt.Fprintf(w, "<- would skip %s (not found)\n", path)
		return
	}
	fmt.Fprintf(w, "<- would remove %s\n", path)
}

// journalDataStats counts journals, days and bytes under the root.
func journalDataStats(j *jot.Jot) (journals, days int, size int64) {
	if infos, err := j.List(); err == nil {
		journals = len(infos)
		for _, info := range infos {
			days += info.Days
		}
	}
	_ = filepath.Walk(j.Root(), func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return journals, days, size
}

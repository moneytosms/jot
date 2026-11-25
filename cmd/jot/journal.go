package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/moneytosms/jot"
)

// journalCmd manages journals. It delegates to the same listing code as
// `jot ls` so the two stay in step.
func (a *app) journalCmd(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: jot journal <new|rm|rename|ls|encrypt|decrypt|archives|restore> [arguments] (see `jot help journal`)")
	}
	switch args[0] {
	case "-h", "--help":
		return printCommandHelp("journal", a.out)
	}
	j, err := a.loadJot()
	if err != nil {
		return err
	}
	switch args[0] {
	case "new":
		return a.journalNew(j, args[1:])
	case "rm", "remove", "delete":
		return a.journalRm(j, args[1:])
	case "rename", "mv":
		return a.journalRename(j, args[1:])
	case "ls":
		return a.lsCmd(args[1:])
	case "encrypt", "decrypt":
		if len(args) < 2 {
			return fmt.Errorf("usage: jot journal %s NAME", args[0])
		}
		on := args[0] == "encrypt"
		n, err := j.SetEncrypted(args[1], on)
		if err != nil {
			return err
		}
		fmt.Fprintf(a.out, "%sed %d %s in %s\n", args[0], n, plural(n, "day", "days"), jot.SanitizeName(args[1]))
		if on {
			fmt.Fprintf(a.out, "key: %s (back it up: without it this journal cannot be read)\n", jot.KeyPath())
		}
		return nil
	case "archives":
		list, err := j.Archives()
		if err != nil {
			return err
		}
		if len(list) == 0 {
			fmt.Fprintln(a.out, "no archives in "+filepath.Join(j.Root(), "backups"))
		}
		for _, ar := range list {
			fmt.Fprintf(a.out, "%s  %s\n", ar.Modified.Format("2006-01-02 15:04"), ar.Path)
		}
		return nil
	case "restore":
		if len(args) < 2 {
			return errors.New("usage: jot journal restore ARCHIVE (see `jot journal archives`)")
		}
		path := args[1]
		if !strings.Contains(path, string(filepath.Separator)) {
			path = filepath.Join(j.Root(), "backups", path)
		}
		names, err := j.RestoreArchive(path)
		if err != nil {
			return err
		}
		fmt.Fprintf(a.out, "restored %s\n", strings.Join(names, ", "))
		return nil
	default:
		return fmt.Errorf("unknown journal command %q; use new, rm, rename, ls, encrypt, decrypt, archives or restore", args[0])
	}
}

// journalNew creates a journal directory and prints its path.
func (a *app) journalNew(j *jot.Jot, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: jot journal new NAME")
	}
	name := jot.SanitizeName(args[0])
	if j.JournalExists(name) {
		return fmt.Errorf("journal %q already exists", name)
	}
	dir, err := j.EnsureJournal(name)
	if err != nil {
		return err
	}
	fmt.Fprintf(a.out, "created %s\n", dir)
	return nil
}

// journalRm archives a journal into <root>/backups, then deletes it after a
// typed confirmation. The default journal is refused; --yes skips the prompt
// for scripts, and a non-terminal stdin is refused so a script never blocks.
func (a *app) journalRm(j *jot.Jot, args []string) error {
	fs := a.flags("journal rm")
	yes := fs.Bool("yes", false, "delete without asking")
	fs.BoolVar(yes, "y", false, "shorthand for -yes")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("usage: jot journal rm [--yes] NAME")
	}
	name := pickJournal(j, fs.Arg(0))
	if name == j.DefaultJournal() {
		return fmt.Errorf("%w: %s", jot.ErrDefaultJournal, name)
	}
	if !*yes {
		// A real terminal can prompt; a piped stdin cannot answer, so it is
		// refused before anything is archived rather than allowed to block.
		// A reader of unknown origin (tests, embedding) is given the chance
		// to answer.
		if f, ok := a.in.(*os.File); ok && !jot.IsTerminal(f) {
			return fmt.Errorf("refusing to delete %q: stdin is not a terminal; re-run with --yes to skip the prompt", name)
		}
	}

	archive, err := j.ArchiveJournal(name)
	if err != nil {
		return err
	}
	fmt.Fprintf(a.out, "archived %s to %s\n", name, archive)

	if !*yes {
		fmt.Fprintf(a.out, "type %s to delete it (anything else aborts): ", name)
		answer, err := readLine(a.in)
		if err != nil {
			return err
		}
		if strings.TrimSpace(answer) != name {
			fmt.Fprintf(a.out, "aborted; %s is untouched (archive kept at %s)\n", name, archive)
			return nil
		}
	}

	if err := j.DeleteJournal(name); err != nil {
		return err
	}
	fmt.Fprintf(a.out, "deleted %s\n", name)
	return nil
}

// journalRename renames a journal directory. The library refuses the default
// journal, the backups name, a missing source and an existing destination.
func (a *app) journalRename(j *jot.Jot, args []string) error {
	if len(args) != 2 {
		return errors.New("usage: jot journal rename OLD NEW")
	}
	from := pickJournal(j, args[0])
	if from == j.DefaultJournal() {
		return fmt.Errorf("%w: %s", jot.ErrDefaultJournal, from)
	}
	if err := j.RenameJournal(from, args[1]); err != nil {
		return err
	}
	fmt.Fprintf(a.out, "renamed %s to %s\n", from, jot.SanitizeName(args[1]))
	return nil
}

// readLine reads one line from r. End of input is an empty line, not an error.
func readLine(r io.Reader) (string, error) {
	sc := bufio.NewScanner(r)
	if !sc.Scan() {
		return "", sc.Err()
	}
	return sc.Text(), nil
}

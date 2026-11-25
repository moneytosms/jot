package jot

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Git sync settings for the "git" config key.
const (
	GitOff    = "off"
	GitCommit = "commit"
	GitPush   = "push"
)

// GitSettings lists the "git" values in the order the TUI cycles them.
var GitSettings = []string{GitOff, GitCommit, GitPush}

// SyncError means a write succeeded but the git sync after it failed.
type SyncError struct{ Err error }

func (e *SyncError) Error() string { return "git sync: " + e.Err.Error() }
func (e *SyncError) Unwrap() error { return e.Err }

// Sync commits the journal root (and pushes, under "push") when git sync is
// on. Failures come back as *SyncError so callers can warn and carry on.
func (j *Jot) Sync(msg string) error { return j.sync(msg) }

func (j *Jot) sync(msg string) error {
	mode := j.cfg.Git
	if mode != GitCommit && mode != GitPush {
		return nil
	}
	if err := j.gitSync(msg, mode == GitPush); err != nil {
		return &SyncError{err}
	}
	return nil
}

// gitSync initializes the repository on first use, commits every change and
// optionally pushes to the configured remote.
func (j *Jot) gitSync(msg string, push bool) error {
	git := func(timeout time.Duration, args ...string) (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		cmd := exec.CommandContext(ctx, "git", append([]string{"-C", j.root}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0") // never hang on a credential prompt
		out, err := cmd.CombinedOutput()
		text := strings.TrimSpace(string(out))
		if err != nil {
			if text == "" {
				text = err.Error()
			}
			return text, errors.New(args[0] + ": " + text)
		}
		return text, nil
	}
	short := 10 * time.Second
	if _, err := os.Stat(filepath.Join(j.root, ".git")); os.IsNotExist(err) {
		if _, err := git(short, "init", "-q"); err != nil {
			return err
		}
		ignore := filepath.Join(j.root, ".gitignore")
		if _, err := os.Stat(ignore); os.IsNotExist(err) {
			if err := os.WriteFile(ignore, []byte(backupDirName+"/\n"), 0o600); err != nil {
				return err
			}
		}
	}
	if _, err := git(short, "add", "-A"); err != nil {
		return err
	}
	if status, err := git(short, "status", "--porcelain"); err != nil || status == "" {
		return err // nothing to commit
	}
	args := []string{"commit", "-q", "-m", "jot: " + msg}
	if email, _ := git(short, "config", "user.email"); email == "" {
		args = append([]string{"-c", "user.name=jot", "-c", "user.email=jot@localhost"}, args...)
	}
	if _, err := git(short, args...); err != nil {
		return err
	}
	if !push {
		return nil
	}
	if remotes, _ := git(short, "remote"); remotes == "" {
		return nil
	}
	_, err := git(30*time.Second, "push", "-q")
	return err
}

// templateName is the per-journal (or root-wide) daily template file. It has
// no .md suffix so it never shows up as a day.
const templateName = ".template"

// TemplatePath returns the journal's own template file path.
func (j *Jot) TemplatePath(name string) string {
	return filepath.Join(j.root, j.journalName(name), templateName)
}

// template returns the text a new day starts with: the journal's template,
// else the root-wide one, else nothing.
func (j *Jot) template(name string) string {
	for _, p := range []string{j.TemplatePath(name), filepath.Join(j.root, templateName)} {
		if data, err := os.ReadFile(p); err == nil && strings.TrimSpace(string(data)) != "" {
			return strings.TrimRight(string(data), "\n") + "\n"
		}
	}
	return ""
}

// EditFile prepares a day for an external editor and returns the path to open
// and a finish func to call once the editor exits. A new day is seeded with
// the template as a timestamped entry. Encrypted journals are decrypted into
// a private temporary file that finish re-encrypts and removes. finish also
// runs git sync; a *SyncError from it means the edit itself was saved.
func (j *Jot) EditFile(name, day string) (string, func() error, error) {
	label, err := ParseDay(day)
	if err != nil {
		return "", nil, err
	}
	if err := j.checkJournal(name); err != nil {
		return "", nil, err
	}
	path := filepath.Join(j.root, j.journalName(name), label+".md")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", nil, err
	}
	old, err := readDayFile(path)
	isNew := os.IsNotExist(err)
	if err != nil && !isNew {
		return "", nil, err
	}
	var seed []byte
	if isNew {
		if t := j.template(name); t != "" {
			seed = []byte(FormatEntry(time.Now().Format(timeLayout), t))
		}
	}
	msg := "edit " + j.journalName(name) + "/" + label
	if !j.IsEncrypted(name) {
		if isNew {
			if err := os.WriteFile(path, seed, 0o600); err != nil {
				return "", nil, err
			}
		}
		return path, func() error { return j.sync(msg) }, nil
	}
	dir, err := os.MkdirTemp(os.Getenv("XDG_RUNTIME_DIR"), "jot-edit-")
	if err != nil {
		return "", nil, err
	}
	start := old
	if isNew {
		start = seed
	}
	tmp := filepath.Join(dir, label+".md")
	if err := os.WriteFile(tmp, start, 0o600); err != nil {
		os.RemoveAll(dir)
		return "", nil, err
	}
	finish := func() error {
		defer os.RemoveAll(dir)
		data, err := os.ReadFile(tmp)
		if err != nil {
			return fmt.Errorf("reading edited file: %w", err)
		}
		if (!isNew && bytes.Equal(data, old)) || (isNew && strings.TrimSpace(string(data)) == "") {
			return nil
		}
		if err := j.writeDayFile(name, path, data); err != nil {
			return err
		}
		return j.sync(msg)
	}
	return tmp, finish, nil
}

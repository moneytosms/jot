package jot

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// backupDirName is the directory inside the root that holds backups. It is not
// a journal and is excluded from listings and from backup archives.
const backupDirName = "backups"

// DefaultInstallDir returns the directory `jot install` targets by default.
func DefaultInstallDir() string {
	if dir := strings.TrimSpace(os.Getenv(EnvInstallDir)); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return filepath.Join(home, ".local", "bin")
}

// Edit opens a journal day in the user's editor, creating the file if needed.
func (j *Jot) Edit(name, day string, in io.Reader, out, errw io.Writer) error {
	label, err := ParseDay(day)
	if err != nil {
		return err
	}
	if err := j.checkJournal(name); err != nil {
		return err
	}
	path := filepath.Join(j.root, j.journalName(name), label+".md")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
	}

	editor := strings.TrimSpace(j.cfg.Editor)
	if editor == "" {
		editor = DetectEditor()
	}
	if editor == "" {
		return fmt.Errorf("no editor found: set %s or $EDITOR", EnvEditor)
	}
	fields := strings.Fields(editor)
	args := append(fields[1:], path)
	cmd := exec.Command(fields[0], args...)
	cmd.Stdin = in
	cmd.Stdout = out
	cmd.Stderr = errw
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("running editor %s: %w", fields[0], err)
	}
	return nil
}

// DetectEditor picks an editor from the environment, then from PATH.
func DetectEditor() string {
	for _, env := range []string{EnvEditor, "VISUAL", "EDITOR"} {
		if v := strings.TrimSpace(os.Getenv(env)); v != "" {
			return v
		}
	}
	for _, candidate := range []string{"vim", "vi", "nano"} {
		if p, err := exec.LookPath(candidate); err == nil {
			return p
		}
	}
	return ""
}

// Watch follows today's file of a journal and copies new entries to w until the
// context is cancelled. It keeps following across midnight.
func (j *Jot) Watch(ctx context.Context, name string, w io.Writer, poll time.Duration) error {
	if err := j.checkJournal(name); err != nil {
		return err
	}
	name = j.journalName(name)
	dir := filepath.Join(j.root, name)
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return fmt.Errorf("%w: %s", ErrNoJournal, name)
	}
	if poll <= 0 {
		poll = 750 * time.Millisecond
	}

	var (
		day    string
		offset int64
	)
	report := func(format string, args ...any) {
		fmt.Fprintf(w, format, args...)
	}

	for {
		if current := today(); current != day {
			if day != "" {
				report("\n")
			}
			day = current
			offset = 0
			report("--- %s/%s.md ---\n", name, day)
		}
		path := filepath.Join(dir, day+".md")
		data, err := os.ReadFile(path)
		switch {
		case err == nil:
			if int64(len(data)) < offset {
				// The file was truncated or rewritten; start over.
				offset = 0
			}
			if int64(len(data)) > offset {
				if _, err := w.Write(data[offset:]); err != nil {
					return err
				}
				offset = int64(len(data))
			}
		case !os.IsNotExist(err):
			return err
		}

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(poll):
		}
	}
}

// Backup writes a compressed archive of every journal to dir and returns the
// archive path. An empty dir means <root>/backups.
func (j *Jot) Backup(dir string) (string, error) {
	if strings.TrimSpace(dir) == "" {
		dir = filepath.Join(j.root, backupDirName)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	archive := filepath.Join(dir, "jot-backup-"+time.Now().Format("20060102-150405")+".tar.gz")
	f, err := os.Create(archive)
	if err != nil {
		return "", err
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	skip := filepath.Clean(filepath.Join(j.root, backupDirName))

	walkErr := filepath.Walk(j.root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if filepath.Clean(path) == skip {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(info.Name(), ".md") {
			return nil
		}
		rel, err := filepath.Rel(j.root, path)
		if err != nil {
			return err
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = filepath.ToSlash(rel)
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		src, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(tw, src)
		closeErr := src.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})

	closeErr := tw.Close()
	if err := gz.Close(); closeErr == nil {
		closeErr = err
	}
	if err := f.Close(); closeErr == nil {
		closeErr = err
	}
	if walkErr != nil || closeErr != nil {
		os.Remove(archive)
		if walkErr != nil {
			return "", walkErr
		}
		return "", closeErr
	}
	return archive, nil
}

// Export writes one markdown file per journal into dir and returns the paths
// written. An empty dir means ./jot-export.
func (j *Jot) Export(dir string) ([]string, error) {
	if strings.TrimSpace(dir) == "" {
		dir = "jot-export"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	infos, err := j.List()
	if err != nil {
		return nil, err
	}
	written := make([]string, 0, len(infos))
	for _, info := range infos {
		files, err := j.ShowDays(info.Name)
		if err != nil {
			return nil, err
		}
		var b strings.Builder
		fmt.Fprintf(&b, "# %s\n\n", info.Name)
		for _, file := range files {
			content := strings.TrimRight(file.Content, "\n")
			if strings.TrimSpace(content) == "" {
				continue
			}
			fmt.Fprintf(&b, "## %s\n\n%s\n\n", file.Day, content)
		}
		path := filepath.Join(dir, info.Name+".md")
		if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
			return nil, err
		}
		written = append(written, path)
	}
	return written, nil
}

// Install copies the running binary into dir and returns its path. The second
// return value reports whether a copy was actually made.
func (j *Jot) Install(dir string) (string, bool, error) {
	if strings.TrimSpace(dir) == "" {
		dir = DefaultInstallDir()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", false, err
	}
	exe, err := os.Executable()
	if err != nil {
		return "", false, err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	dest := filepath.Join(dir, "jot")
	if same, err := sameFile(exe, dest); err == nil && same {
		return dest, false, nil
	}
	if same, err := sameContents(exe, dest); err == nil && same {
		return dest, false, nil
	}

	src, err := os.Open(exe)
	if err != nil {
		return "", false, err
	}
	defer src.Close()

	tmp := dest + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return "", false, err
	}
	_, copyErr := io.Copy(out, src)
	if copyErr == nil {
		copyErr = out.Sync()
	}
	closeErr := out.Close()
	if copyErr == nil {
		copyErr = closeErr
	}
	if copyErr != nil {
		os.Remove(tmp)
		return "", false, copyErr
	}
	if err := os.Rename(tmp, dest); err != nil {
		os.Remove(tmp)
		return "", false, err
	}
	return dest, true, nil
}

// Uninstall deletes the journal root. It refuses obviously dangerous paths.
func (j *Jot) Uninstall() error {
	root := filepath.Clean(j.root)
	if root == "" || root == "." || root == string(filepath.Separator) {
		return fmt.Errorf("refusing to remove %q", j.root)
	}
	if home, err := os.UserHomeDir(); err == nil && filepath.Clean(home) == root {
		return errors.New("refusing to remove your home directory")
	}
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: %s", os.ErrNotExist, root)
		}
		return err
	}
	return os.RemoveAll(root)
}

// Update rebuilds jot from a local source checkout and installs the binary into
// installDir, returning its path. An empty srcDir means JOT_SRC or the current
// directory tree.
func (j *Jot) Update(srcDir, installDir string) (string, error) {
	if strings.TrimSpace(srcDir) == "" {
		srcDir = os.Getenv("JOT_SRC")
	}
	moduleRoot, err := FindModuleRoot(srcDir)
	if err != nil {
		return "", err
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		return "", errors.New("go toolchain not found: install Go to update from source")
	}
	if strings.TrimSpace(installDir) == "" {
		installDir = DefaultInstallDir()
	}
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return "", err
	}
	dest := filepath.Join(installDir, "jot")
	cmd := exec.Command(goBin, "build", "-trimpath", "-o", dest, "./cmd/jot")
	cmd.Dir = moduleRoot
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("go build failed: %w\n%s", err, strings.TrimSpace(string(output)))
	}
	return dest, nil
}

// FindModuleRoot walks up from start looking for this module's go.mod.
func FindModuleRoot(start string) (string, error) {
	if strings.TrimSpace(start) == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		start = wd
	}
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		if err == nil && strings.Contains(string(data), "module "+ModulePath) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no %s source checkout found from %s (set JOT_SRC)", ModulePath, start)
		}
		dir = parent
	}
}

// Check is one doctor result.
type Check struct {
	// Name is the short label of the check.
	Name   string
	Status string // "ok", "warn" or "fail"
	// Detail explains the result.
	Detail string
}

// Doctor inspects the installation and reports anything worth fixing. It never
// fails: each problem becomes a check result.
func (j *Jot) Doctor() []Check {
	var checks []Check
	add := func(name, status, detail string) {
		checks = append(checks, Check{Name: name, Status: status, Detail: detail})
	}

	// Root directory.
	if st, err := os.Stat(j.root); err == nil && st.IsDir() {
		add("root", "ok", j.root)
		probe := filepath.Join(j.root, fmt.Sprintf(".jot-doctor-%d", os.Getpid()))
		if err := os.WriteFile(probe, []byte("ok"), 0o600); err != nil {
			add("writable", "fail", "cannot write to root: "+err.Error())
		} else {
			os.Remove(probe)
			add("writable", "ok", "root is writable")
		}
	} else if os.IsNotExist(err) {
		add("root", "warn", "does not exist yet; it is created on the first entry")
	} else if err != nil {
		add("root", "fail", err.Error())
	} else {
		add("root", "fail", j.root+" is not a directory")
	}

	// Config file.
	configPath := ConfigPath()
	if configPath == "" {
		add("config", "warn", "no config directory available; using defaults")
	} else if _, err := LoadConfigFile(); err != nil {
		add("config", "fail", err.Error())
	} else if _, err := os.Stat(configPath); os.IsNotExist(err) {
		add("config", "ok", "none yet; using defaults")
	} else {
		add("config", "ok", configPath)
	}

	// Journals.
	infos, err := j.List()
	if err != nil {
		add("journals", "fail", err.Error())
	} else {
		entries := 0
		for _, info := range infos {
			entries += info.Entries
		}
		add("journals", "ok", fmt.Sprintf("%d journals, %d entries", len(infos), entries))
	}

	// Default journal.
	if _, err := os.Stat(filepath.Join(j.root, j.cfg.DefaultJournal)); err != nil {
		add("default journal", "warn", fmt.Sprintf("%q does not exist yet; `jot add` creates it", j.cfg.DefaultJournal))
	} else {
		add("default journal", "ok", j.cfg.DefaultJournal)
	}

	// Editor.
	editor := strings.TrimSpace(j.cfg.Editor)
	if editor == "" {
		editor = DetectEditor()
	}
	if editor == "" {
		add("editor", "warn", "no editor found; set "+EnvEditor+" or $EDITOR")
	} else if fields := strings.Fields(editor); len(fields) > 0 {
		if _, err := exec.LookPath(fields[0]); err != nil {
			add("editor", "fail", editor+" is not executable")
		} else {
			add("editor", "ok", editor)
		}
	}

	// Binary on PATH.
	if path, err := exec.LookPath("jot"); err != nil {
		add("binary", "warn", "`jot` is not on your PATH")
	} else {
		add("binary", "ok", path)
	}

	return checks
}

// sameContents reports whether two files hold identical bytes.
func sameContents(a, b string) (bool, error) {
	ai, err := os.Stat(a)
	if err != nil {
		return false, err
	}
	bi, err := os.Stat(b)
	if err != nil {
		return false, err
	}
	if ai.Size() != bi.Size() {
		return false, nil
	}
	aDigest, err := fileDigest(a)
	if err != nil {
		return false, err
	}
	bDigest, err := fileDigest(b)
	if err != nil {
		return false, err
	}
	return bytes.Equal(aDigest, bDigest), nil
}

func fileDigest(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}

// sameFile reports whether two paths point at the same file.
func sameFile(a, b string) (bool, error) {
	ai, err := os.Stat(a)
	if err != nil {
		return false, err
	}
	bi, err := os.Stat(b)
	if err != nil {
		return false, err
	}
	return os.SameFile(ai, bi), nil
}

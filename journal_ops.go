package jot

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ErrDefaultJournal means the default journal cannot be removed or renamed.
var ErrDefaultJournal = errors.New("the default journal cannot be removed or renamed")

// JournalExists reports whether a journal directory exists in the root. An
// empty name means the default journal.
func (j *Jot) JournalExists(name string) bool {
	_, err := os.Stat(filepath.Join(j.root, j.journalName(name)))
	return err == nil
}

// ArchiveJournal writes a compressed archive of one journal to
// <root>/backups/jot-<name>-YYYYMMDD-HHMMSS.tar.gz and returns the archive
// path. Files are stored under <name>/... so the archive unpacks as a journal
// directory, like the full backup. A missing journal wraps ErrNoJournal.
func (j *Jot) ArchiveJournal(name string) (string, error) {
	if err := j.checkJournal(name); err != nil {
		return "", err
	}
	name = j.journalName(name)
	dir := filepath.Join(j.root, name)
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return "", fmt.Errorf("%w: %s", ErrNoJournal, name)
	}
	backups := filepath.Join(j.root, backupDirName)
	if err := os.MkdirAll(backups, 0o700); err != nil {
		return "", err
	}
	archive := filepath.Join(backups, "jot-"+name+"-"+time.Now().Format("20060102-150405")+".tar.gz")
	f, err := os.Create(archive)
	if err != nil {
		return "", err
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)

	walkErr := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = filepath.ToSlash(filepath.Join(name, rel))
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

// DeleteJournal removes a journal directory and every file in it. It is a
// destructive primitive with no undo: the CALLER (CLI/TUI) is required to
// archive the journal first, with ArchiveJournal, and confirm with the user
// before calling this. It refuses the default journal with ErrDefaultJournal
// and the reserved backups name.
func (j *Jot) DeleteJournal(name string) error {
	name = j.journalName(name)
	if name == j.cfg.DefaultJournal {
		return fmt.Errorf("%w: %s", ErrDefaultJournal, name)
	}
	if name == backupDirName {
		return fmt.Errorf("journal name %q is reserved for backups; pick another name", name)
	}
	dir := filepath.Join(j.root, name)
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return fmt.Errorf("%w: %s", ErrNoJournal, name)
	}
	return os.RemoveAll(dir)
}

// RenameJournal renames a journal directory by sanitizing the new name and
// moving the directory. Renaming a journal to itself is a no-op success. It
// refuses renaming the default journal (ErrDefaultJournal), the reserved
// backups name on either side, a missing source (wrapping ErrNoJournal) and an
// existing destination. A failed rename touches nothing: both journals are
// left exactly as they were.
func (j *Jot) RenameJournal(from, to string) error {
	from = j.journalName(from)
	if strings.TrimSpace(to) == "" {
		return errors.New("new journal name is empty")
	}
	to = SanitizeName(to)
	if from == j.cfg.DefaultJournal {
		return fmt.Errorf("%w: %s", ErrDefaultJournal, from)
	}
	if from == backupDirName || to == backupDirName {
		n := from
		if n != backupDirName {
			n = to
		}
		return fmt.Errorf("journal name %q is reserved for backups; pick another name", n)
	}
	if from == to {
		return nil
	}
	src := filepath.Join(j.root, from)
	if st, err := os.Stat(src); err != nil || !st.IsDir() {
		return fmt.Errorf("%w: %s", ErrNoJournal, from)
	}
	dst := filepath.Join(j.root, to)
	if _, err := os.Stat(dst); err == nil {
		return fmt.Errorf("cannot rename journal %q: journal %q already exists", from, to)
	}
	return os.Rename(src, dst)
}

// ResolveRoot expands "~" and makes a root path absolute and clean.
func ResolveRoot(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", errors.New("root path is empty")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Abs(expandHome(path, home))
}

// MoveRoot moves the whole journal root to dst, which must not exist or be an
// empty directory outside the current root. It renames when it can and
// otherwise copies everything, preserving modes, and removes the old root
// only after the copy succeeded. A missing root just leaves dst to be created
// on first write. The caller saves the new root to the config.
func (j *Jot) MoveRoot(dst string) error {
	dst, err := ResolveRoot(dst)
	if err != nil {
		return err
	}
	if rel, err := filepath.Rel(j.root, dst); err == nil && (rel == "." || !strings.HasPrefix(rel, "..")) {
		return fmt.Errorf("new root %s is inside the current root %s", dst, j.root)
	}
	if entries, err := os.ReadDir(dst); err == nil {
		if len(entries) > 0 {
			return fmt.Errorf("new root %s is not empty", dst)
		}
		if err := os.Remove(dst); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if _, err := os.Stat(j.root); os.IsNotExist(err) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	if os.Rename(j.root, dst) == nil {
		return nil
	}
	// Different filesystem (or volume): copy, then drop the original.
	if err := copyTree(j.root, dst); err != nil {
		os.RemoveAll(dst) // dst was empty or missing, so only the partial copy goes
		return err
	}
	return os.RemoveAll(j.root)
}

// copyTree copies the directory src to dst, keeping file and directory modes.
func copyTree(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		if !info.Mode().IsRegular() {
			return nil // journals hold plain files only
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			out.Close()
			return err
		}
		return out.Close()
	})
}

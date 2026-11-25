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

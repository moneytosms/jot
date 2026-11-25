package jot

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// opsTestJot returns a Jot rooted in a temporary directory.
func opsTestJot(t *testing.T) *Jot {
	t.Helper()
	return New(Config{Root: filepath.Join(t.TempDir(), "jot"), DefaultJournal: "default"})
}

// addEntry writes one entry into a journal so archives have content.
func addEntry(t *testing.T, j *Jot, name, body string) {
	t.Helper()
	if err := j.Add(Entry{Name: name, Body: body}); err != nil {
		t.Fatal(err)
	}
}

func TestJournalExists(t *testing.T) {
	j := opsTestJot(t)
	if j.JournalExists("work") {
		t.Fatal("expected work journal to not exist yet")
	}
	if err := j.Init(); err != nil {
		t.Fatal(err)
	}
	if !j.JournalExists("") {
		t.Fatal("expected default journal to exist after Init")
	}
	if j.JournalExists("work") {
		t.Fatal("expected work journal to not exist yet")
	}
	if _, err := j.EnsureJournal("work"); err != nil {
		t.Fatal(err)
	}
	if !j.JournalExists("work") {
		t.Fatal("expected work journal to exist after EnsureJournal")
	}
	// Names resolve through SanitizeName, same as everywhere else.
	if _, err := j.EnsureJournal("my notes"); err != nil {
		t.Fatal(err)
	}
	if !j.JournalExists("my notes") {
		t.Fatal("expected sanitized name my-notes to be found from \"my notes\"")
	}
}

func TestArchiveJournal_ContentsAndName(t *testing.T) {
	j := opsTestJot(t)
	addEntry(t, j, "work", "hello archive")

	archive, err := j.ArchiveJournal("work")
	if err != nil {
		t.Fatal(err)
	}
	base := filepath.Base(archive)
	if !strings.HasPrefix(base, "jot-work-") || !strings.HasSuffix(base, ".tar.gz") {
		t.Fatalf("unexpected archive name %q", base)
	}
	if filepath.Dir(archive) != filepath.Join(j.Root(), backupDirName) {
		t.Fatalf("expected archive in backups dir, got %q", archive)
	}

	f, err := os.Open(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	var names []string
	var body string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, hdr.Name)
		if strings.HasSuffix(hdr.Name, ".md") {
			data, err := io.ReadAll(tr)
			if err != nil {
				t.Fatal(err)
			}
			body = string(data)
		}
	}
	if len(names) != 1 {
		t.Fatalf("expected one archive entry, got %v", names)
	}
	if !strings.HasPrefix(names[0], "work/") || !strings.HasSuffix(names[0], ".md") {
		t.Fatalf("unexpected archive entry %q", names[0])
	}
	if !strings.Contains(body, "hello archive") {
		t.Fatalf("archive body missing entry text, got %q", body)
	}

	// Archiving leaves the journal itself untouched.
	if !j.JournalExists("work") {
		t.Fatal("journal disappeared after archiving")
	}
}

func TestArchiveJournal_Missing(t *testing.T) {
	j := opsTestJot(t)
	if _, err := j.ArchiveJournal("ghost"); !errors.Is(err, ErrNoJournal) {
		t.Fatalf("expected ErrNoJournal, got %v", err)
	}
}

func TestDeleteJournal_Default(t *testing.T) {
	j := opsTestJot(t)
	if err := j.Init(); err != nil {
		t.Fatal(err)
	}
	err := j.DeleteJournal("")
	if !errors.Is(err, ErrDefaultJournal) {
		t.Fatalf("expected ErrDefaultJournal, got %v", err)
	}
	if !j.JournalExists("") {
		t.Fatal("default journal was removed despite refusal")
	}
}

func TestDeleteJournal_ReservedName(t *testing.T) {
	j := opsTestJot(t)
	err := j.DeleteJournal(backupDirName)
	if err == nil {
		t.Fatal("expected error for reserved backups name")
	}
	if errors.Is(err, ErrNoJournal) {
		t.Fatal("reserved name should be refused before any existence check")
	}
	if !strings.Contains(err.Error(), "reserved for backups") {
		t.Fatalf("unexpected error message %q", err.Error())
	}
}

func TestDeleteJournal_Missing(t *testing.T) {
	j := opsTestJot(t)
	if err := j.DeleteJournal("ghost"); !errors.Is(err, ErrNoJournal) {
		t.Fatalf("expected ErrNoJournal, got %v", err)
	}
}

func TestDeleteJournal_RemovesDirectoryAndFiles(t *testing.T) {
	j := opsTestJot(t)
	addEntry(t, j, "temp", "to be removed")
	// Archive first, as the caller is required to.
	archive, err := j.ArchiveJournal("temp")
	if err != nil {
		t.Fatal(err)
	}
	if err := j.DeleteJournal("temp"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(j.Root(), "temp")); !os.IsNotExist(err) {
		t.Fatalf("journal directory still exists: %v", err)
	}
	if j.JournalExists("temp") {
		t.Fatal("JournalExists still reports the deleted journal")
	}
	// The pre-delete archive survives the delete.
	entries, err := os.ReadDir(filepath.Join(j.Root(), backupDirName))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range entries {
		if e.Name() == filepath.Base(archive) {
			found = true
		}
	}
	if !found {
		t.Fatalf("archive %q missing from backups after delete", filepath.Base(archive))
	}
}

func TestRenameJournal_Success(t *testing.T) {
	j := opsTestJot(t)
	addEntry(t, j, "old-name", "keep me")

	if err := j.RenameJournal("old-name", "new name"); err != nil {
		t.Fatal(err)
	}
	if j.JournalExists("old-name") {
		t.Fatal("old journal directory still exists")
	}
	if !j.JournalExists("new-name") {
		t.Fatal("new journal directory missing after rename")
	}
	days, err := j.Days("new-name")
	if err != nil {
		t.Fatal(err)
	}
	if len(days) != 1 {
		t.Fatalf("expected one day file after rename, got %v", days)
	}
	content, err := ReadEntries(filepath.Join(j.Root(), "new-name", days[0]+".md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, "keep me") {
		t.Fatalf("entry content lost in rename, got %q", content)
	}
}

func TestRenameJournal_Refusals(t *testing.T) {
	j := opsTestJot(t)
	if err := j.Init(); err != nil {
		t.Fatal(err)
	}
	addEntry(t, j, "work", "x")

	if err := j.RenameJournal("", "other"); !errors.Is(err, ErrDefaultJournal) {
		t.Fatalf("expected ErrDefaultJournal for default journal, got %v", err)
	}
	if err := j.RenameJournal(backupDirName, "other"); err == nil || !strings.Contains(err.Error(), "reserved for backups") {
		t.Fatalf("expected reserved-name refusal for source, got %v", err)
	}
	if err := j.RenameJournal("work", backupDirName); err == nil || !strings.Contains(err.Error(), "reserved for backups") {
		t.Fatalf("expected reserved-name refusal for destination, got %v", err)
	}
	if err := j.RenameJournal("ghost", "anywhere"); !errors.Is(err, ErrNoJournal) {
		t.Fatalf("expected ErrNoJournal for missing source, got %v", err)
	}
	if err := j.RenameJournal("work", ""); err == nil {
		t.Fatal("expected error for empty new name")
	}
	if err := j.RenameJournal("work", "default"); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected existing-destination refusal, got %v", err)
	}

	// None of the refusals may have moved or removed anything.
	if !j.JournalExists("work") {
		t.Fatal("work journal lost during refused renames")
	}
	if j.JournalExists("other") || j.JournalExists("anywhere") {
		t.Fatal("refused rename created a journal")
	}
}

func TestRenameJournal_SameName(t *testing.T) {
	j := opsTestJot(t)
	addEntry(t, j, "daily", "unchanged")

	if err := j.RenameJournal("daily", "daily"); err != nil {
		t.Fatalf("expected no-op success, got %v", err)
	}
	if !j.JournalExists("daily") {
		t.Fatal("journal disappeared on same-name rename")
	}
	// Same-name rename of a journal that does not exist is also a no-op.
	if err := j.RenameJournal("missing", "missing"); err != nil {
		t.Fatalf("expected no-op success, got %v", err)
	}
}

func TestMoveRoot(t *testing.T) {
	j := opsTestJot(t)
	addEntry(t, j, "work", "moved note")
	dst := filepath.Join(t.TempDir(), "new")
	if err := j.MoveRoot(filepath.Join(j.Root(), "inside")); err == nil {
		t.Fatal("moving into the current root should fail")
	}
	if err := j.MoveRoot(dst); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(j.Root()); !os.IsNotExist(err) {
		t.Fatalf("old root still there: %v", err)
	}
	moved := New(Config{Root: dst, DefaultJournal: "default"})
	if !moved.JournalExists("work") {
		t.Fatal("journal missing at the new root")
	}

	// copyTree is the cross-device path; check it directly.
	cp := filepath.Join(t.TempDir(), "copy")
	if err := copyTree(dst, cp); err != nil {
		t.Fatal(err)
	}
	if st, err := os.Stat(filepath.Join(cp, "work")); err != nil || st.Mode().Perm() != 0o700 {
		t.Fatalf("copied journal dir: %v %v", st, err)
	}

	full := filepath.Join(t.TempDir(), "full")
	if err := os.MkdirAll(filepath.Join(full, "x"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := moved.MoveRoot(full); err == nil {
		t.Fatal("moving onto a non-empty dir should fail")
	}
}

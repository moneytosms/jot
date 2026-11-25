package jot

import (
	"archive/zip"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// cryptTestJot isolates the key file and config for one test.
func cryptTestJot(t *testing.T) *Jot {
	t.Helper()
	t.Setenv(EnvKey, filepath.Join(t.TempDir(), "key.txt"))
	keyMu.Lock()
	keyCache = nil
	keyMu.Unlock()
	return opsTestJot(t)
}

func TestEncryptedJournalRoundTrip(t *testing.T) {
	j := cryptTestJot(t)
	addEntry(t, j, "secret", "before encryption")
	n, err := j.SetEncrypted("secret", true)
	if err != nil || n != 1 {
		t.Fatalf("SetEncrypted = %d, %v", n, err)
	}
	addEntry(t, j, "secret", "after encryption\n- [ ] todo")
	day := today()
	raw, _ := os.ReadFile(filepath.Join(j.Root(), "secret", day+".md"))
	if !isCiphertext(raw) || strings.Contains(string(raw), "encryption") {
		t.Fatalf("day file is not ciphertext:\n%s", raw)
	}
	got, err := j.Show("secret", day)
	if err != nil || len(got) != 1 || !strings.Contains(got[0], "before encryption") || !strings.Contains(got[0], "after encryption") {
		t.Fatalf("Show = %q, %v", got, err)
	}
	if todos, err := j.TodosJournal("secret"); err != nil || len(todos) != 1 {
		t.Fatalf("todos = %v, %v", todos, err)
	}
	if hits, err := j.SearchEntriesJournal("secret", "after"); err != nil || len(hits) == 0 {
		t.Fatalf("search = %v, %v", hits, err)
	}
	if days, _ := j.Days("secret"); len(days) != 1 {
		t.Fatalf("marker listed as a day: %v", days)
	}

	// Editing goes through a plaintext temp file and lands encrypted.
	path, finish, err := j.EditFile("secret", day)
	if err != nil || strings.HasPrefix(path, j.Root()) {
		t.Fatalf("EditFile = %q, %v", path, err)
	}
	if err := os.WriteFile(path, []byte("## 10:00:00\n\nrewritten\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := finish(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("plaintext temp file left behind")
	}
	if got, _ := j.Show("secret", day); len(got) != 1 || got[0] != "## 10:00:00\n\nrewritten\n" {
		t.Fatalf("after edit: %q", got)
	}

	if n, err := j.SetEncrypted("secret", false); err != nil || n != 1 || j.IsEncrypted("secret") {
		t.Fatalf("decrypt = %d, %v", n, err)
	}
	raw, _ = os.ReadFile(filepath.Join(j.Root(), "secret", day+".md"))
	if string(raw) != "## 10:00:00\n\nrewritten\n" {
		t.Fatalf("decrypted file = %q", raw)
	}
}

func TestEncryptedWithoutKey(t *testing.T) {
	j := cryptTestJot(t)
	addEntry(t, j, "secret", "x")
	if _, err := j.SetEncrypted("secret", true); err != nil {
		t.Fatal(err)
	}
	os.Remove(KeyPath())
	keyMu.Lock()
	keyCache = nil
	keyMu.Unlock()
	if _, err := j.Show("secret", ""); !errors.Is(err, ErrNoKey) {
		t.Fatalf("err = %v, want ErrNoKey", err)
	}
}

func TestTemplateSeedsNewDay(t *testing.T) {
	j := cryptTestJot(t)
	if _, err := j.EnsureJournal("work"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(j.TemplatePath("work"), []byte("3 wins:\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	path, finish, err := j.EditFile("work", "2026-01-02")
	if err != nil {
		t.Fatal(err)
	}
	finish()
	data, _ := os.ReadFile(path)
	if !strings.HasPrefix(string(data), "## ") || !strings.Contains(string(data), "3 wins:") {
		t.Fatalf("seeded day = %q", data)
	}
	if days, _ := j.Days("work"); len(days) != 1 {
		t.Fatalf("template listed as a day: %v", days)
	}
}

func TestGitSyncCommits(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	j := New(Config{Root: filepath.Join(t.TempDir(), "jot"), DefaultJournal: "default", Git: GitCommit})
	addEntry(t, j, "default", "first")
	addEntry(t, j, "default", "second")
	out, err := exec.Command("git", "-C", j.Root(), "log", "--oneline").Output()
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(strings.TrimSpace(string(out)), "\n") + 1; n != 2 {
		t.Fatalf("commits = %d:\n%s", n, out)
	}
}

func TestBackupWithZipConfigAndKey(t *testing.T) {
	j := cryptTestJot(t)
	t.Setenv(EnvConfig, filepath.Join(t.TempDir(), "config.json"))
	if _, err := SaveConfig(Config{Editor: "vim"}); err != nil {
		t.Fatal(err)
	}
	addEntry(t, j, "work", "zip me")
	if _, err := j.SetEncrypted("work", true); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(j.Root(), templateName), []byte("t"), 0o600); err != nil {
		t.Fatal(err)
	}
	names := func(path string) []string {
		zr, err := zip.OpenReader(path)
		if err != nil {
			t.Fatal(err)
		}
		defer zr.Close()
		var out []string
		for _, f := range zr.File {
			out = append(out, f.Name)
		}
		return out
	}
	dir := t.TempDir()
	plain, err := j.BackupWith(BackupOptions{Dir: dir, Zip: true})
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(names(plain), " ")
	for _, want := range []string{".jot/config.json", "work/.encrypted", ".template", "work/" + today() + ".md"} {
		if !strings.Contains(got, want) {
			t.Errorf("zip missing %s: %s", want, got)
		}
	}
	if strings.Contains(got, "age-key") {
		t.Errorf("key bundled without asking: %s", got)
	}
	keyed, err := j.BackupWith(BackupOptions{Dir: t.TempDir(), Zip: true, Key: true})
	if err != nil || !strings.Contains(strings.Join(names(keyed), " "), ".jot/age-key.txt") {
		t.Fatalf("with key: %v %v", names(keyed), err)
	}
	tgz, err := j.BackupWith(BackupOptions{Dir: t.TempDir()})
	if err != nil || !strings.HasSuffix(tgz, ".tar.gz") {
		t.Fatalf("tar backup = %q, %v", tgz, err)
	}

	// The zip restores the journal (still encrypted) and skips .jot/ extras.
	if err := j.DeleteJournal("work"); err != nil {
		t.Fatal(err)
	}
	restored, err := j.RestoreArchive(plain)
	if err != nil || len(restored) != 1 || !j.IsEncrypted("work") {
		t.Fatalf("restore zip = %v, %v", restored, err)
	}
	if got, _ := j.Show("work", ""); len(got) != 1 || !strings.Contains(got[0], "zip me") {
		t.Fatalf("restored content = %q", got)
	}
}

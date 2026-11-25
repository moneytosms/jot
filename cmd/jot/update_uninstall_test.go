package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/moneytosms/jot"
)

// newCLIReleaseServer serves a fake release API and its assets for the CLI
// tests. Tests point JOT_RELEASES_API at it, so no real network is touched.
func newCLIReleaseServer(t *testing.T, release *jot.ReleaseInfo, files map[string][]byte) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	serve := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(release); err != nil {
			t.Errorf("encoding release: %v", err)
		}
	}
	mux.HandleFunc("/latest", serve)
	mux.HandleFunc("/tags/", func(w http.ResponseWriter, r *http.Request) {
		if strings.TrimPrefix(r.URL.Path, "/tags/") != release.Tag {
			http.NotFound(w, r)
			return
		}
		serve(w, r)
	})
	mux.HandleFunc("/download/", func(w http.ResponseWriter, r *http.Request) {
		data, ok := files[strings.TrimPrefix(r.URL.Path, "/download/")]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(data)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	for i := range release.Assets {
		release.Assets[i].URL = server.URL + "/download/" + release.Assets[i].Name
	}
	return server
}

// cliRelease builds a release fixture with a matching SHA256SUMS file.
func cliRelease(tag string, payload []byte) (*jot.ReleaseInfo, map[string][]byte) {
	name := jot.PlatformAssetName()
	digest := sha256.Sum256(payload)
	sums := fmt.Sprintf("%x  %s\n", digest, name)
	release := &jot.ReleaseInfo{
		Tag:       tag,
		Name:      "jot " + tag,
		Notes:     "- fixed the parser\n- added more tests",
		Published: time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC),
		Assets: []jot.ReleaseAsset{
			{Name: name, Size: int64(len(payload))},
			{Name: "SHA256SUMS", Size: int64(len(sums))},
		},
	}
	return release, map[string][]byte{name: payload, "SHA256SUMS": []byte(sums)}
}

// withVersion temporarily changes the build version the CLI reports.
func withVersion(t *testing.T, v string) {
	t.Helper()
	old := version
	version = v
	t.Cleanup(func() { version = old })
}

func TestCLIUpdateCheck(t *testing.T) {
	payload := []byte("fake binary")
	release, files := cliRelease("v0.2.0", payload)
	server := newCLIReleaseServer(t, release, files)
	t.Setenv(jot.EnvReleasesAPI, server.URL)
	testEnv(t)
	withVersion(t, "v0.1.0")

	code, out, errw := runCLI(t, "", "update", "--check")
	if code != 0 {
		t.Fatalf("update --check failed (%d): %s", code, errw)
	}
	for _, want := range []string{
		"update available: v0.1.0 -> v0.2.0",
		"released 2026-11-01",
		"What's new:",
		"fixed the parser",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("update --check missing %q:\n%s", want, out)
		}
	}

	withVersion(t, "v0.2.0")
	code, out, errw = runCLI(t, "", "update", "--check")
	if code != 0 {
		t.Fatalf("up-to-date check failed (%d): %s", code, errw)
	}
	if !strings.Contains(out, "<-- up to date: jot v0.2.0") {
		t.Errorf("expected an up-to-date status:\n%s", out)
	}
	if strings.Contains(out, "update available") {
		t.Errorf("up-to-date check reported an update:\n%s", out)
	}
}

func TestCLIUpdateWithoutTerminalDoesNotInstall(t *testing.T) {
	payload := []byte("fake binary")
	release, files := cliRelease("v0.2.0", payload)
	server := newCLIReleaseServer(t, release, files)
	t.Setenv(jot.EnvReleasesAPI, server.URL)
	dir := testEnv(t)
	withVersion(t, "v0.1.0")

	code, out, errw := runCLI(t, "", "update")
	if code != 0 {
		t.Fatalf("non-interactive update failed (%d): %s", code, errw)
	}
	if !strings.Contains(out, "re-run with --yes to install") {
		t.Errorf("expected a --yes hint:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(dir, "bin", "jot")); !os.IsNotExist(err) {
		t.Fatalf("a binary was installed without --yes: %v", err)
	}
}

func TestCLIUpdateJSON(t *testing.T) {
	payload := []byte("fake binary")
	release, files := cliRelease("v0.2.0", payload)
	server := newCLIReleaseServer(t, release, files)
	t.Setenv(jot.EnvReleasesAPI, server.URL)
	testEnv(t)
	withVersion(t, "v0.1.0")

	code, out, errw := runCLI(t, "", "update", "--check", "--json")
	if code != 0 {
		t.Fatalf("update --json failed (%d): %s", code, errw)
	}
	var status struct {
		Current         string `json:"current"`
		Latest          string `json:"latest"`
		UpdateAvailable bool   `json:"update_available"`
		Released        string `json:"released"`
		URL             string `json:"url"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &status); err != nil {
		t.Fatalf("decoding --json output %q: %v", out, err)
	}
	if status.Current != "v0.1.0" || status.Latest != "v0.2.0" || !status.UpdateAvailable {
		t.Errorf("unexpected status: %+v", status)
	}
	if status.Released != "2026-11-01" || status.URL == "" {
		t.Errorf("unexpected release fields: %+v", status)
	}
}

func TestCLIUpdateYesInstallsBinary(t *testing.T) {
	payload := []byte("#!/bin/sh\necho jot v0.2.0\n")
	release, files := cliRelease("v0.2.0", payload)
	server := newCLIReleaseServer(t, release, files)
	t.Setenv(jot.EnvReleasesAPI, server.URL)
	testEnv(t)
	withVersion(t, "v0.1.0")
	installDir := filepath.Join(t.TempDir(), "bin")

	code, out, errw := runCLI(t, "", "update", "--yes", "--dir", installDir)
	if code != 0 {
		t.Fatalf("update --yes failed (%d): %s", code, errw)
	}
	path := filepath.Join(installDir, "jot")
	for _, want := range []string{"installed jot v0.2.0 at " + path, "what changed", "restart jot"} {
		if !strings.Contains(out, want) {
			t.Errorf("update output missing %q:\n%s", want, out)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading installed binary: %v", err)
	}
	if string(data) != string(payload) {
		t.Fatalf("installed bytes = %q, want %q", data, payload)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("installed mode = %o, want 755", info.Mode().Perm())
	}
}

// withTemporaryBinary points binary removal at a temporary file so tests do
// not delete the running test binary.
func withTemporaryBinary(t *testing.T) (string, func()) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "jot")
	if err := os.WriteFile(path, []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	oldPath, oldRemove := runningBinaryPath, removeRunningBinary
	runningBinaryPath = func() (string, error) { return path, nil }
	removeRunningBinary = func(*jot.Jot) (string, error) {
		if err := os.Remove(path); err != nil {
			return "", err
		}
		return path, nil
	}
	restore := func() {
		runningBinaryPath = oldPath
		removeRunningBinary = oldRemove
	}
	t.Cleanup(restore)
	return path, restore
}

func TestCLIUninstallBinary(t *testing.T) {
	testEnv(t)
	path, _ := withTemporaryBinary(t)

	code, out, errw := runCLI(t, "", "uninstall", "--binary", "--yes")
	if code != 0 {
		t.Fatalf("uninstall --binary failed (%d): %s", code, errw)
	}
	if !strings.Contains(out, "<- removed "+path) {
		t.Errorf("uninstall did not report the removed binary:\n%s", out)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("binary still exists: %v", err)
	}
}

func TestCLIUninstallData(t *testing.T) {
	dir := testEnv(t)
	root := filepath.Join(dir, "jot")
	mustRun(t, "", "add", "keep me")

	code, out, errw := runCLI(t, "", "uninstall", "--data", "--yes")
	if code != 0 {
		t.Fatalf("uninstall --data failed (%d): %s", code, errw)
	}
	if !strings.Contains(out, "deleted") {
		t.Errorf("uninstall --yes must keep the old deleted wording:\n%s", out)
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("root still exists: %v", err)
	}
}

func TestCLIUninstallConfig(t *testing.T) {
	dir := testEnv(t)
	root := filepath.Join(dir, "jot")
	mustRun(t, "", "add", "keep me")
	configPath := filepath.Join(dir, "config.json")
	if err := os.WriteFile(configPath, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	code, out, errw := runCLI(t, "", "uninstall", "--config", "--yes")
	if code != 0 {
		t.Fatalf("uninstall --config failed (%d): %s", code, errw)
	}
	if !strings.Contains(out, "<- removed "+configPath) {
		t.Errorf("config removal not reported:\n%s", out)
	}
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Fatalf("config still exists: %v", err)
	}
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("data was removed by --config: %v", err)
	}
}

func TestCLIUninstallDryRunAll(t *testing.T) {
	dir := testEnv(t)
	root := filepath.Join(dir, "jot")
	mustRun(t, "", "add", "keep me")
	configPath := filepath.Join(dir, "config.json")
	if err := os.WriteFile(configPath, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	path, _ := withTemporaryBinary(t)

	code, out, errw := runCLI(t, "", "uninstall", "--dry-run", "--all")
	if code != 0 {
		t.Fatalf("uninstall --dry-run failed (%d): %s", code, errw)
	}
	for _, want := range []string{"dry run", "<- would remove " + configPath, "<- would remove " + root, "<- would remove " + path} {
		if !strings.Contains(out, want) {
			t.Errorf("dry run missing %q:\n%s", want, out)
		}
	}
	for _, still := range []string{root, configPath, path} {
		if _, err := os.Stat(still); err != nil {
			t.Errorf("dry run removed %s: %v", still, err)
		}
	}
}

func TestCLIUninstallNonInteractiveWithoutFlags(t *testing.T) {
	dir := testEnv(t)
	root := filepath.Join(dir, "jot")
	mustRun(t, "", "add", "keep me")

	code, _, errw := runCLI(t, "", "uninstall")
	if code != 1 {
		t.Fatalf("expected a usage error, got %d", code)
	}
	for _, want := range []string{"--data", "--binary", "--config", "--all", "--yes"} {
		if !strings.Contains(errw, want) {
			t.Errorf("usage error missing %q: %s", want, errw)
		}
	}
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("data was removed without an explicit component: %v", err)
	}
}
func TestReleaseNoteLines(t *testing.T) {
	var many strings.Builder
	for i := 0; i < 20; i++ {
		fmt.Fprintf(&many, "- line %d\n", i)
	}
	lines, truncated := releaseNoteLines(many.String(), false)
	if !truncated {
		t.Fatal("expected short notes to be marked truncated")
	}
	if len(lines) != 15 {
		t.Fatalf("short notes = %d lines, want 15", len(lines))
	}
	all, truncated := releaseNoteLines(many.String(), true)
	if truncated || len(all) != 20 {
		t.Fatalf("full notes = %d lines, truncated=%v; want 20, false", len(all), truncated)
	}

	wrapped, _ := releaseNoteLines(strings.Repeat("word ", 40), false)
	if len(wrapped) < 2 {
		t.Fatalf("long notes did not wrap: %q", wrapped)
	}
	for _, line := range wrapped {
		if len([]rune(line)) > notesWidth {
			t.Errorf("wrapped line is %d runes, want <= %d: %q", len([]rune(line)), notesWidth, line)
		}
	}
}

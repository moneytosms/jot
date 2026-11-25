package jot

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		name string
		a    string
		b    string
		want int
	}{
		{"equal with v prefix", "v1.2.3", "1.2.3", 0},
		{"missing components", "v1.2", "v1.2.0", 0},
		{"single component", "1", "1.0.0", 0},
		{"newer patch", "v1.2.4", "v1.2.3", 1},
		{"older minor", "v1.2.9", "v1.3.0", -1},
		{"numeric not lexical", "v1.10.0", "v1.9.9", 1},
		{"pre-release sorts before release", "v1.3.0-rc1", "v1.3.0", -1},
		{"release sorts after pre-release", "v1.3.0", "v1.3.0-rc1", 1},
		{"alphanumeric pre-release sorts lexically", "v1.3.0-rc10", "v1.3.0-rc2", -1},
		{"numeric before alphanumeric", "v1.0.0-1", "v1.0.0-alpha", -1},
		{"shorter pre-release prefix", "v1.0.0-alpha", "v1.0.0-alpha.1", -1},
		{"dev build older than release", "0.0.0-dev", "v1.0.0", -1},
		{"dev build older than same release", "0.0.0-dev", "v0.0.0", -1},
		{"dev build older than any pre-release", "0.0.0-dev", "v0.0.0-alpha", -1},
		{"build metadata ignored", "v1.2.3+build.7", "v1.2.3", 0},
		{"unparseable is older", "not-a-version", "v0.0.1", -1},
		{"parseable is newer than unparseable", "v0.0.1", "not-a-version", 1},
		{"two unparseable versions", "dev", "nightly", 0},
		{"empty is unparseable", "", "v1.0.0", -1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CompareVersions(tc.a, tc.b); got != tc.want {
				t.Fatalf("CompareVersions(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
			}
			if got := CompareVersions(tc.b, tc.a); got != -tc.want {
				t.Fatalf("CompareVersions(%q, %q) = %d, want %d", tc.b, tc.a, got, -tc.want)
			}
		})
	}
}

func TestUpdateAvailable(t *testing.T) {
	cases := []struct {
		current string
		tag     string
		want    bool
	}{
		{"v1.0.0", "v1.0.1", true},
		{"v1.0.0", "v1.0.0", false},
		{"v1.0.0", "v0.9.9", false},
		{"0.0.0-dev", "v0.1.0", true},
		{"v1.2", "v1.2.0", false},
	}
	for _, tc := range cases {
		if got := UpdateAvailable(tc.current, ReleaseInfo{Tag: tc.tag}); got != tc.want {
			t.Errorf("UpdateAvailable(%q, %q) = %v, want %v", tc.current, tc.tag, got, tc.want)
		}
	}
}

// newReleaseAPIServer serves a fake release API and its assets.
func newReleaseAPIServer(t *testing.T, release *ReleaseInfo, files map[string][]byte) (*httptest.Server, *atomic.Int64) {
	t.Helper()
	var hits atomic.Int64
	mux := http.NewServeMux()
	mux.HandleFunc("/latest", func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		writeReleaseJSON(t, w, release)
	})
	mux.HandleFunc("/tags/", func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if strings.TrimPrefix(r.URL.Path, "/tags/") != release.Tag {
			http.NotFound(w, r)
			return
		}
		writeReleaseJSON(t, w, release)
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
	return server, &hits
}

func writeReleaseJSON(t *testing.T, w http.ResponseWriter, release *ReleaseInfo) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(release); err != nil {
		t.Errorf("encoding release: %v", err)
	}
}

// releaseFixture builds a release with a platform binary and SHA256SUMS.
func releaseFixture(tag, notes string, payload []byte, checksum string) (*ReleaseInfo, map[string][]byte) {
	assetName := PlatformAssetName()
	sums := fmt.Sprintf("%s  %s\n", checksum, assetName)
	release := &ReleaseInfo{
		Tag:       tag,
		Name:      "jot " + tag,
		Notes:     notes,
		Published: time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC),
		Assets: []ReleaseAsset{
			{Name: assetName, Size: int64(len(payload))},
			{Name: "SHA256SUMS", Size: int64(len(sums))},
		},
	}
	return release, map[string][]byte{assetName: payload, "SHA256SUMS": []byte(sums)}
}

func TestUpdateFromReleaseInstallsVerifiedBinary(t *testing.T) {
	payload := []byte("#!/bin/sh\necho jot v9.9.9\n")
	digest := sha256.Sum256(payload)
	release, files := releaseFixture("v9.9.9", "- fixed a thing\n- added another", payload, fmt.Sprintf("%x", digest))
	server, hits := newReleaseAPIServer(t, release, files)
	t.Setenv(EnvReleasesAPI, server.URL)

	installDir := filepath.Join(t.TempDir(), "bin")
	var progress bytes.Buffer
	path, err := testJot(t).UpdateFromRelease("v0.1.0", "", installDir, &progress)
	if err != nil {
		t.Fatalf("UpdateFromRelease failed: %v", err)
	}
	want := filepath.Join(installDir, "jot")
	if path != want {
		t.Fatalf("installed path = %q, want %q", path, want)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading installed binary: %v", err)
	}
	if !bytes.Equal(data, payload) {
		t.Fatalf("installed bytes = %q, want %q", data, payload)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat installed binary: %v", err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("installed mode = %o, want 755", info.Mode().Perm())
	}
	for _, wantLine := range []string{"downloading", "verifying", "installed"} {
		if !strings.Contains(progress.String(), wantLine) {
			t.Errorf("progress missing %q:\n%s", wantLine, progress.String())
		}
	}
	if hits.Load() == 0 {
		t.Fatal("release API was never called")
	}
}

func TestUpdateFromReleaseRejectsBadChecksum(t *testing.T) {
	payload := []byte("verified bytes")
	release, files := releaseFixture("v2.0.0", "notes", payload, strings.Repeat("0", 64))
	server, _ := newReleaseAPIServer(t, release, files)
	t.Setenv(EnvReleasesAPI, server.URL)

	installDir := filepath.Join(t.TempDir(), "bin")
	_, err := testJot(t).UpdateFromRelease("v1.0.0", "", installDir, nil)
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("expected a checksum mismatch, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(installDir, "jot")); !os.IsNotExist(err) {
		t.Fatalf("a binary was left behind after a bad checksum: %v", err)
	}
	entries, err := os.ReadDir(installDir)
	if err != nil {
		t.Fatalf("reading install dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("temporary files were left behind: %v", entries)
	}
}

func TestUpdateFromReleaseRejectsMissingChecksumEntry(t *testing.T) {
	payload := []byte("bytes")
	digest := sha256.Sum256(payload)
	release, files := releaseFixture("v2.0.0", "notes", payload, fmt.Sprintf("%x", digest))
	files["SHA256SUMS"] = []byte(strings.Repeat("a", 64) + "  jot-other-platform\n")
	server, _ := newReleaseAPIServer(t, release, files)
	t.Setenv(EnvReleasesAPI, server.URL)

	_, err := testJot(t).UpdateFromRelease("v1.0.0", "", t.TempDir(), nil)
	if err == nil || !strings.Contains(err.Error(), "no entry") {
		t.Fatalf("expected a missing checksum entry error, got %v", err)
	}
}

func TestUpdateFromReleaseMissingPlatformAsset(t *testing.T) {
	release := &ReleaseInfo{
		Tag:       "v3.0.0",
		Published: time.Now(),
		Assets: []ReleaseAsset{
			{Name: "jot-plan9-mips"},
			{Name: "SHA256SUMS"},
		},
	}
	server, _ := newReleaseAPIServer(t, release, map[string][]byte{})
	t.Setenv(EnvReleasesAPI, server.URL)

	_, err := testJot(t).UpdateFromRelease("v2.0.0", "", t.TempDir(), nil)
	if err == nil {
		t.Fatal("expected an error for a missing platform asset")
	}
	for _, want := range []string{runtime.GOOS, runtime.GOARCH, "no binary"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

func TestFetchReleaseUsesOverrideAndPinnedTag(t *testing.T) {
	payload := []byte("x")
	digest := sha256.Sum256(payload)
	release, files := releaseFixture("v4.5.6", "notes", payload, fmt.Sprintf("%x", digest))
	server, hits := newReleaseAPIServer(t, release, files)
	t.Setenv(EnvReleasesAPI, server.URL)

	latest, err := FetchRelease("v0.0.1", "")
	if err != nil {
		t.Fatalf("FetchRelease latest failed: %v", err)
	}
	if latest.Tag != "v4.5.6" || latest.Notes != "notes" {
		t.Fatalf("unexpected latest release: %+v", latest)
	}
	pinned, err := FetchRelease("v0.0.1", "v4.5.6")
	if err != nil {
		t.Fatalf("FetchRelease pinned failed: %v", err)
	}
	if pinned.Tag != release.Tag {
		t.Fatalf("pinned tag = %q, want %q", pinned.Tag, release.Tag)
	}
	if hits.Load() < 2 {
		t.Fatalf("expected the override server to serve both requests, got %d", hits.Load())
	}
}

func TestFetchReleaseHTTPErrorIncludesStatusAndURL(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	t.Setenv(EnvReleasesAPI, server.URL)

	_, err := FetchRelease("v1.0.0", "")
	if err == nil {
		t.Fatal("expected an HTTP error")
	}
	var status *HTTPStatusError
	if !errors.As(err, &status) {
		t.Fatalf("expected HTTPStatusError, got %T: %v", err, err)
	}
	if status.StatusCode != http.StatusNotFound {
		t.Fatalf("status code = %d, want 404", status.StatusCode)
	}
	if !strings.Contains(err.Error(), "404") || !strings.Contains(err.Error(), server.URL) {
		t.Fatalf("error %q must include the status and the URL", err)
	}
}

func TestRemoveBinary(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "jot")
	if err := os.WriteFile(path, []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	old := executable
	executable = func() (string, error) { return path, nil }
	t.Cleanup(func() { executable = old })

	removed, err := testJot(t).RemoveBinary()
	if err != nil {
		t.Fatalf("RemoveBinary failed: %v", err)
	}
	if removed != path {
		t.Fatalf("removed path = %q, want %q", removed, path)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("binary still exists: %v", err)
	}
}

func TestRemoveBinaryRefusesDirectoryAndEmptyPath(t *testing.T) {
	dir := t.TempDir()
	old := executable
	t.Cleanup(func() { executable = old })

	executable = func() (string, error) { return dir, nil }
	if _, err := testJot(t).RemoveBinary(); err == nil || !strings.Contains(err.Error(), "directory") {
		t.Fatalf("expected a directory refusal, got %v", err)
	}

	executable = func() (string, error) { return "  ", nil }
	if _, err := testJot(t).RemoveBinary(); err == nil || !strings.Contains(err.Error(), "cannot determine") {
		t.Fatalf("expected an empty-path refusal, got %v", err)
	}
}

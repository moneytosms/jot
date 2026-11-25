package jot

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// EnvReleasesAPI overrides the release API base URL. It exists for mirrors
// and for tests, which set it to a local httptest server.
const EnvReleasesAPI = "JOT_RELEASES_API"

// ReleaseAPIBase is the base URL of the GitHub releases API. The latest
// release lives at <base>/latest and a pinned release at <base>/tags/<tag>.
// Set JOT_RELEASES_API to override it at runtime, for example for a mirror.
var ReleaseAPIBase = "https://api.github.com/repos/moneytosms/jot/releases"

// releaseHTTPTimeout bounds every release API and download request.
const releaseHTTPTimeout = 15 * time.Second

// releaseHTTPClient is shared by release lookups and downloads.
var releaseHTTPClient = &http.Client{Timeout: releaseHTTPTimeout}

// executable resolves the running executable. It is a variable so tests can
// point binary removal at a temporary file instead of the test binary.
var executable = os.Executable

// ReleaseAsset is one downloadable file attached to a release.
type ReleaseAsset struct {
	// Name is the asset file name, for example "jot-linux-amd64".
	Name string `json:"name"`
	// URL is the browser download URL of the asset.
	URL string `json:"browser_download_url"`
	// Size is the asset size in bytes, zero when the API omits it.
	Size int64 `json:"size"`
}

// ReleaseInfo describes one published release.
type ReleaseInfo struct {
	// Tag is the release tag, for example "v1.2.3".
	Tag string `json:"tag_name"`
	// Name is the human-readable release title.
	Name string `json:"name"`
	// Notes is the release body in markdown, also called the changelog.
	Notes string `json:"body"`
	// Published is the release publication time, zero when unknown.
	Published time.Time `json:"published_at"`
	// Assets are the files attached to the release.
	Assets []ReleaseAsset `json:"assets"`
}

// HTTPStatusError reports a non-success response from the release API.
type HTTPStatusError struct {
	// URL is the request URL that failed.
	URL string
	// StatusCode is the HTTP status code, for example 404.
	StatusCode int
	// Status is the HTTP status line, for example "404 Not Found".
	Status string
}

// Error implements error.
func (e *HTTPStatusError) Error() string {
	status := strings.TrimSpace(e.Status)
	if status == "" {
		status = strconv.Itoa(e.StatusCode)
	}
	return fmt.Sprintf("release API request to %s failed: HTTP %s", e.URL, status)
}

// releaseAPIBase returns the effective API base, honouring JOT_RELEASES_API.
func releaseAPIBase() string {
	if override := strings.TrimSpace(os.Getenv(EnvReleasesAPI)); override != "" {
		return strings.TrimRight(override, "/")
	}
	return strings.TrimRight(strings.TrimSpace(ReleaseAPIBase), "/")
}

// PlatformAssetName returns the release asset name for the running platform.
func PlatformAssetName() string {
	return "jot-" + runtime.GOOS + "-" + runtime.GOARCH
}

// FetchRelease fetches the latest release when tag is empty, otherwise the
// release carrying that exact tag. currentVersion is used in the User-Agent
// and may be empty.
func FetchRelease(currentVersion, tag string) (ReleaseInfo, error) {
	base := releaseAPIBase()
	endpoint := base + "/latest"
	if tag = strings.TrimSpace(tag); tag != "" {
		endpoint = base + "/tags/" + url.PathEscape(tag)
	}

	request, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return ReleaseInfo{}, fmt.Errorf("building release request: %w", err)
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "jot/"+userAgentVersion(currentVersion))

	response, err := releaseHTTPClient.Do(request)
	if err != nil {
		return ReleaseInfo{}, fmt.Errorf("fetching %s: %w", endpoint, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		// Drain a little so the connection can be reused, then report.
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4<<10))
		return ReleaseInfo{}, &HTTPStatusError{
			URL:        endpoint,
			StatusCode: response.StatusCode,
			Status:     response.Status,
		}
	}

	var release ReleaseInfo
	decoder := json.NewDecoder(io.LimitReader(response.Body, 4<<20))
	if err := decoder.Decode(&release); err != nil {
		return ReleaseInfo{}, fmt.Errorf("decoding release from %s: %w", endpoint, err)
	}
	if strings.TrimSpace(release.Tag) == "" {
		return ReleaseInfo{}, fmt.Errorf("release at %s has no tag", endpoint)
	}
	return release, nil
}

// userAgentVersion keeps the User-Agent readable when no version is known.
func userAgentVersion(current string) string {
	if current = strings.TrimSpace(current); current != "" {
		return current
	}
	return "0.0.0-dev"
}

// CompareVersions orders two semver-ish version strings. A leading "v" and
// missing components are ignored (v1.2 equals v1.2.0), and pre-release
// suffixes sort before their final release (v1.3.0-rc1 < v1.3.0). A version
// that cannot be parsed, such as a development build, sorts before every
// parseable version. It returns -1, 0 or 1.
func CompareVersions(a, b string) int {
	aVersion, aOK := parseVersion(a)
	bVersion, bOK := parseVersion(b)
	switch {
	case !aOK && !bOK:
		return 0
	case !aOK:
		return -1
	case !bOK:
		return 1
	}

	components := len(aVersion.numbers)
	if len(bVersion.numbers) > components {
		components = len(bVersion.numbers)
	}
	for i := 0; i < components; i++ {
		left, right := 0, 0
		if i < len(aVersion.numbers) {
			left = aVersion.numbers[i]
		}
		if i < len(bVersion.numbers) {
			right = bVersion.numbers[i]
		}
		if left < right {
			return -1
		}
		if left > right {
			return 1
		}
	}

	// A final release sorts after its pre-releases.
	switch {
	case aVersion.pre == nil && bVersion.pre == nil:
		return 0
	case aVersion.pre == nil:
		return 1
	case bVersion.pre == nil:
		return -1
	}
	return comparePrerelease(aVersion.pre, bVersion.pre)
}

// UpdateAvailable reports whether rel is newer than current.
func UpdateAvailable(current string, rel ReleaseInfo) bool {
	return CompareVersions(current, rel.Tag) < 0
}

// parsedVersion is a version split into numeric components and a pre-release.
type parsedVersion struct {
	numbers []int
	pre     []string
}

// parseVersion parses a semver-ish string. Build metadata is ignored, leading
// "v" is stripped and missing components are treated as zero.
func parseVersion(s string) (parsedVersion, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return parsedVersion{}, false
	}
	if s[0] == 'v' || s[0] == 'V' {
		s = s[1:]
	}
	if s == "" {
		return parsedVersion{}, false
	}
	if i := strings.IndexByte(s, '+'); i >= 0 {
		s = s[:i]
	}
	var pre []string
	if i := strings.IndexByte(s, '-'); i >= 0 {
		pre = strings.Split(s[i+1:], ".")
		s = s[:i]
		for _, identifier := range pre {
			if identifier == "" {
				return parsedVersion{}, false
			}
			// Development builds such as the default 0.0.0-dev are not
			// releases; they sort before every parseable version.
			if strings.EqualFold(identifier, "dev") {
				return parsedVersion{}, false
			}
		}
	}
	if s == "" {
		return parsedVersion{}, false
	}
	parts := strings.Split(s, ".")
	numbers := make([]int, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			return parsedVersion{}, false
		}
		number, err := strconv.Atoi(part)
		if err != nil || number < 0 {
			return parsedVersion{}, false
		}
		numbers = append(numbers, number)
	}
	return parsedVersion{numbers: numbers, pre: pre}, true
}

// comparePrerelease orders two pre-release identifier lists per semver:
// numeric identifiers compare numerically, numeric sorts before alphanumeric,
// alphanumeric compares lexically, and a shorter matching prefix sorts first.
func comparePrerelease(a, b []string) int {
	for i := 0; i < len(a) && i < len(b); i++ {
		left, right := a[i], b[i]
		leftNumeric := isNumericIdentifier(left)
		rightNumeric := isNumericIdentifier(right)
		switch {
		case leftNumeric && rightNumeric:
			if cmp := compareNumericStrings(left, right); cmp != 0 {
				return cmp
			}
		case leftNumeric:
			return -1
		case rightNumeric:
			return 1
		default:
			if cmp := strings.Compare(left, right); cmp != 0 {
				return cmp
			}
		}
	}
	switch {
	case len(a) < len(b):
		return -1
	case len(a) > len(b):
		return 1
	}
	return 0
}

// isNumericIdentifier reports whether a pre-release identifier is all digits.
func isNumericIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// compareNumericStrings compares two digit strings as numbers without risking
// integer overflow.
func compareNumericStrings(a, b string) int {
	a = strings.TrimLeft(a, "0")
	b = strings.TrimLeft(b, "0")
	if len(a) != len(b) {
		if len(a) < len(b) {
			return -1
		}
		return 1
	}
	return strings.Compare(a, b)
}

// UpdateFromRelease downloads and installs the release binary for this
// platform and returns the installed path. pinVersion selects a tag; an empty
// pinVersion means the latest release. installDir defaults to
// DefaultInstallDir. The download is verified against the release
// SHA256SUMS before it atomically replaces <installDir>/jot.
func (j *Jot) UpdateFromRelease(currentVersion, pinVersion, installDir string, progress io.Writer) (string, error) {
	release, err := FetchRelease(currentVersion, pinVersion)
	if err != nil {
		return "", err
	}
	assetName := PlatformAssetName()
	asset, ok := findAsset(release.Assets, assetName)
	if !ok {
		return "", fmt.Errorf("release %s has no binary for %s/%s (expected asset %s)",
			release.Tag, runtime.GOOS, runtime.GOARCH, assetName)
	}
	sums, ok := findAsset(release.Assets, "SHA256SUMS")
	if !ok {
		return "", fmt.Errorf("release %s has no SHA256SUMS asset; cannot verify the download", release.Tag)
	}

	dir := strings.TrimSpace(installDir)
	if dir == "" {
		dir = DefaultInstallDir()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating install directory %s: %w", dir, err)
	}
	destination := filepath.Join(dir, "jot")
	userAgent := userAgentVersion(currentVersion)

	progressf(progress, "downloading %s...\n", asset.Name)
	temporary, err := os.CreateTemp(dir, ".jot-download-*")
	if err != nil {
		return "", fmt.Errorf("creating a temporary file in %s: %w", dir, err)
	}
	temporaryPath := temporary.Name()
	keepTemporary := false
	defer func() {
		if !keepTemporary {
			_ = os.Remove(temporaryPath)
		}
	}()

	defer temporary.Close()
	digest, err := downloadAsset(asset.URL, userAgent, temporary)
	if err != nil {
		return "", fmt.Errorf("downloading %s: %w", asset.Name, err)
	}

	progressf(progress, "verifying SHA-256...\n")
	sumsData, err := downloadBytes(sums.URL, userAgent)
	if err != nil {
		return "", fmt.Errorf("downloading SHA256SUMS: %w", err)
	}
	want, ok := checksumFor(sumsData, asset.Name)
	if !ok {
		return "", fmt.Errorf("SHA256SUMS has no entry for %s; refusing to install an unverified binary", asset.Name)
	}
	if !strings.EqualFold(want, digest) {
		return "", fmt.Errorf("checksum mismatch for %s: expected %s, got %s", asset.Name, want, digest)
	}

	if err := temporary.Chmod(0o755); err != nil {
		return "", fmt.Errorf("making %s executable: %w", asset.Name, err)
	}
	if err := temporary.Sync(); err != nil {
		return "", fmt.Errorf("syncing %s: %w", asset.Name, err)
	}
	if err := temporary.Close(); err != nil {
		return "", fmt.Errorf("closing %s: %w", asset.Name, err)
	}
	if err := os.Rename(temporaryPath, destination); err != nil {
		return "", fmt.Errorf("installing %s: %w", destination, err)
	}
	keepTemporary = true
	progressf(progress, "installed %s\n", destination)
	return destination, nil
}

// findAsset returns the named asset from a release.
func findAsset(assets []ReleaseAsset, name string) (ReleaseAsset, bool) {
	for _, asset := range assets {
		if asset.Name == name {
			return asset, true
		}
	}
	return ReleaseAsset{}, false
}

// progressf writes one progress line when progress is not nil.
func progressf(progress io.Writer, format string, args ...any) {
	if progress != nil {
		fmt.Fprintf(progress, format, args...)
	}
}

// downloadAsset streams a URL into w and returns the SHA-256 of the bytes.
func downloadAsset(rawURL, userAgent string, w io.Writer) (string, error) {
	response, err := openDownload(rawURL, userAgent)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	hash := sha256.New()
	if _, err := io.Copy(io.MultiWriter(w, hash), response.Body); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// downloadBytes fetches a small URL into memory.
func downloadBytes(rawURL, userAgent string) ([]byte, error) {
	response, err := openDownload(rawURL, userAgent)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	return io.ReadAll(io.LimitReader(response.Body, 1<<20))
}

// openDownload starts a GET request and rejects non-success statuses.
func openDownload(rawURL, userAgent string) (*http.Response, error) {
	request, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building download request: %w", err)
	}
	request.Header.Set("User-Agent", "jot/"+userAgent)
	response, err := releaseHTTPClient.Do(request)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4<<10))
		response.Body.Close()
		return nil, &HTTPStatusError{
			URL:        rawURL,
			StatusCode: response.StatusCode,
			Status:     response.Status,
		}
	}
	return response, nil
}

// checksumFor finds the SHA-256 for name in SHA256SUMS content. Lines look
// like "<sha256>  jot-linux-amd64" or "<sha256> *jot-linux-amd64".
func checksumFor(sums []byte, name string) (string, bool) {
	for _, line := range strings.Split(string(sums), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		file := strings.TrimPrefix(fields[1], "*")
		if filepath.Base(file) == name {
			return fields[0], true
		}
	}
	return "", false
}

// RunningBinaryPath resolves the running executable with symlinks resolved. It
// returns an error when the path cannot be determined.
func RunningBinaryPath() (string, error) {
	path, err := executable()
	if err != nil {
		return "", fmt.Errorf("locating the running binary: %w", err)
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return "", errors.New("cannot determine the running binary path")
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	return path, nil
}

// RemoveBinary removes the running jot executable and returns the removed
// path. It refuses an empty path and a directory.
func (j *Jot) RemoveBinary() (string, error) {
	path, err := RunningBinaryPath()
	if err != nil {
		return "", err
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("removing the binary at %s: %w", path, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("refusing to remove %s: it is a directory, not the jot binary", path)
	}
	if err := os.Remove(path); err != nil {
		return "", fmt.Errorf("removing the binary at %s: %w", path, err)
	}
	return path, nil
}

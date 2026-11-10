package jot

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	dayLayout  = "2006-01-02"
	timeLayout = "15:04:05"
)

var errNoConfigDir = errors.New("cannot determine a config directory; set " + EnvConfig)

func today() string {
	return time.Now().Format(dayLayout)
}

// ParseDay normalizes a day reference into YYYY-MM-DD. It accepts "today",
// "yesterday", "tomorrow", an empty string (meaning today) and absolute dates.
func ParseDay(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return today(), nil
	}
	now := time.Now()
	switch strings.ToLower(s) {
	case "today":
		return now.Format(dayLayout), nil
	case "yesterday":
		return now.AddDate(0, 0, -1).Format(dayLayout), nil
	case "tomorrow":
		return now.AddDate(0, 0, 1).Format(dayLayout), nil
	}
	if t, err := time.ParseInLocation(dayLayout, s, time.Local); err == nil {
		return t.Format(dayLayout), nil
	}
	return "", fmt.Errorf("invalid date %q: use today, yesterday, tomorrow or YYYY-MM-DD", s)
}

// FormatEntry renders a timestamped entry in markdown.
func FormatEntry(ts string, body string) string {
	return fmt.Sprintf("## %s\n\n%s\n", ts, body)
}

// Render renders markdown-ish journal content into a simple visual form.
// It preserves markdown structure while emphasizing headers and list items.
func Render(md string) string {
	lines := strings.Split(md, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "#"):
			out = append(out, "→ "+line)
		case strings.HasPrefix(line, "*") || strings.HasPrefix(line, "-"):
			out = append(out, "  • "+strings.TrimLeft(line, "*- "))
		case strings.TrimSpace(line) != "":
			out = append(out, "  "+line)
		default:
			out = append(out, "")
		}
	}
	return strings.Join(out, "\n")
}

// ReadEntries reads a journal file as raw text. A missing file is not an error.
func ReadEntries(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}

// SanitizeName ensures a journal name is filesystem-safe.
func SanitizeName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return DefaultJournalName
	}
	var out strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '-', r == '_':
			out.WriteRune(r)
		default:
			out.WriteRune('-')
		}
	}
	s := out.String()
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	s = strings.Trim(s, "-")
	if s == "" {
		return DefaultJournalName
	}
	return s
}

// expandHome replaces a leading ~ with the user's home directory.
func expandHome(path, home string) string {
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		return filepath.Join(home, path[2:])
	}
	return path
}

// writeFileAtomic writes data to a temporary file next to path and renames it
// into place, so a crash never leaves a half-written file behind.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

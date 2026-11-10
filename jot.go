// Package jot is a small journaling tool: plain markdown files on disk,
// append-only timestamped entries, and a CLI that stays out of the way.
//
// Journals live under a single root directory (by default ~/.jot). Each
// journal is a directory of YYYY-MM-DD.md files, and each entry inside a file
// is a "## HH:MM:SS" heading followed by the entry body. Nothing is hidden in
// a database, so grep, git, tar and any markdown editor work on the data as is.
package jot

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// Errors callers may want to react to.
var (
	// ErrNoJournal means the journal directory does not exist yet.
	ErrNoJournal = errors.New("journal not found")
	// ErrEmptyBody means an entry had no text.
	ErrEmptyBody = errors.New("entry body is empty")
)

// Entry is one journal entry.
type Entry struct {
	// Name is the journal the entry belongs to. Empty means the default.
	Name string
	// Day is the entry date: today, yesterday or YYYY-MM-DD. Empty means today.
	Day string
	// At is the entry time. Zero means now.
	At time.Time
	// Body is the markdown text of the entry.
	Body string
}

// Jot is a handle on a journal root directory.
type Jot struct {
	cfg  Config
	root string
}

// New creates a Jot handle. Empty config fields fall back to DefaultConfig,
// the default journal name and an auto-detected editor.
func New(cfg Config) *Jot {
	if strings.TrimSpace(cfg.Root) == "" || strings.TrimSpace(cfg.DefaultJournal) == "" {
		if def, err := DefaultConfig(); err == nil {
			if strings.TrimSpace(cfg.Root) == "" {
				cfg.Root = def.Root
			}
			if strings.TrimSpace(cfg.DefaultJournal) == "" {
				cfg.DefaultJournal = def.DefaultJournal
			}
			if strings.TrimSpace(cfg.Editor) == "" {
				cfg.Editor = def.Editor
			}
		}
	}
	if strings.TrimSpace(cfg.Root) == "" {
		home, _ := os.UserHomeDir()
		cfg.Root = filepath.Join(home, ".jot")
	}
	cfg.Root = filepath.Clean(cfg.Root)
	cfg.DefaultJournal = SanitizeName(cfg.DefaultJournal)
	return &Jot{cfg: cfg, root: cfg.Root}
}

// Root returns the journal root directory.
func (j *Jot) Root() string { return j.root }

// DefaultJournal returns the journal used when a command names none.
func (j *Jot) DefaultJournal() string { return j.cfg.DefaultJournal }

// journalName normalizes an empty or unsafe journal name.
func (j *Jot) journalName(name string) string {
	if strings.TrimSpace(name) == "" {
		return j.cfg.DefaultJournal
	}
	return SanitizeName(name)
}

// checkJournal rejects a name that resolves to the backup directory, since that
// journal would be skipped by ls, search, show and export.
func (j *Jot) checkJournal(name string) error {
	if n := j.journalName(name); n == backupDirName {
		return fmt.Errorf("journal name %q is reserved for backups; pick another name", n)
	}
	return nil
}

// Init creates the root directory and the default journal if they are missing.
func (j *Jot) Init() error {
	if err := j.checkJournal(""); err != nil {
		return err
	}
	if err := os.MkdirAll(j.root, 0o700); err != nil {
		return err
	}
	return os.MkdirAll(filepath.Join(j.root, j.cfg.DefaultJournal), 0o700)
}

// EnsureJournal creates a journal directory and returns its path.
func (j *Jot) EnsureJournal(name string) (string, error) {
	if err := j.checkJournal(name); err != nil {
		return "", err
	}
	dir := filepath.Join(j.root, j.journalName(name))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// Add appends a timestamped entry to a journal for the given day.
func (j *Jot) Add(e Entry) error {
	body := strings.TrimRight(e.Body, "\r\n")
	if strings.TrimSpace(body) == "" {
		return ErrEmptyBody
	}
	day := e.Day
	if strings.TrimSpace(day) == "" {
		day = today()
	} else {
		normalized, err := ParseDay(day)
		if err != nil {
			return err
		}
		day = normalized
	}
	at := e.At
	if at.IsZero() {
		at = time.Now()
	}

	if err := j.checkJournal(e.Name); err != nil {
		return err
	}
	path := filepath.Join(j.root, j.journalName(e.Name), day+".md")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	entry := FormatEntry(at.Format(timeLayout), body)
	// Separate entries with one blank line, but leave a brand-new file clean.
	if st, err := f.Stat(); err == nil && st.Size() > 0 {
		entry = "\n" + entry
	}
	if _, err := f.WriteString(entry); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// DayFile is the markdown content of one day of a journal.
type DayFile struct {
	// Day is the day label, YYYY-MM-DD.
	Day string
	// Content is the raw markdown of the day.
	Content string
}

// Days returns the day labels ("YYYY-MM-DD") recorded in a journal, sorted.
func (j *Jot) Days(name string) ([]string, error) {
	if err := j.checkJournal(name); err != nil {
		return nil, err
	}
	dir := filepath.Join(j.root, j.journalName(name))
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrNoJournal, j.journalName(name))
		}
		return nil, err
	}
	days := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		days = append(days, strings.TrimSuffix(e.Name(), ".md"))
	}
	slices.Sort(days)
	return days, nil
}

// ShowDays returns every day of a journal with its content, oldest first.
func (j *Jot) ShowDays(name string) ([]DayFile, error) {
	days, err := j.Days(name)
	if err != nil {
		return nil, err
	}
	out := make([]DayFile, 0, len(days))
	for _, day := range days {
		data, err := os.ReadFile(filepath.Join(j.root, j.journalName(name), day+".md"))
		if err != nil {
			return nil, err
		}
		out = append(out, DayFile{Day: day, Content: string(data)})
	}
	return out, nil
}

// Show returns the content of a journal for a day. An empty day means today and
// "all" returns every day as separate strings. A day with no entries yields an
// empty slice, not an error.
func (j *Jot) Show(name, day string) ([]string, error) {
	if err := j.checkJournal(name); err != nil {
		return nil, err
	}
	if _, err := os.Stat(filepath.Join(j.root, j.journalName(name))); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrNoJournal, j.journalName(name))
		}
		return nil, err
	}
	if strings.EqualFold(strings.TrimSpace(day), "all") {
		files, err := j.ShowDays(name)
		if err != nil {
			return nil, err
		}
		out := make([]string, 0, len(files))
		for _, f := range files {
			out = append(out, f.Content)
		}
		return out, nil
	}
	label := day
	if strings.TrimSpace(label) == "" {
		label = today()
	}
	d, err := ParseDay(label)
	if err != nil {
		return nil, err
	}
	content, err := ReadEntries(filepath.Join(j.root, j.journalName(name), d+".md"))
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(content) == "" {
		return nil, nil
	}
	return []string{content}, nil
}

// DayInfo summarizes one day of a journal.
type DayInfo struct {
	// Day is the day label, YYYY-MM-DD.
	Day string
	// Entries is the number of entries recorded that day.
	Entries int
	// Bytes is the size of the day file on disk.
	Bytes int64
	// Modified is the day file's last modification time.
	Modified time.Time
}

// DaySummaries describes each day of a journal.
func (j *Jot) DaySummaries(name string) ([]DayInfo, error) {
	days, err := j.Days(name)
	if err != nil {
		return nil, err
	}
	out := make([]DayInfo, 0, len(days))
	for _, day := range days {
		path := filepath.Join(j.root, j.journalName(name), day+".md")
		info := DayInfo{Day: day}
		if st, err := os.Stat(path); err == nil {
			info.Bytes = st.Size()
			info.Modified = st.ModTime()
		}
		if n, err := countEntries(path); err == nil {
			info.Entries = n
		}
		out = append(out, info)
	}
	return out, nil
}

// JournalInfo summarizes one journal.
type JournalInfo struct {
	// Name is the journal name.
	Name string
	// Days is the number of days with a file.
	Days int
	// Entries is the total number of entries across all days.
	Entries int
	// Last is the modification time of the newest day file.
	Last time.Time
}

// List returns every journal in the root, sorted by name. A missing root means
// no journals yet, not an error.
func (j *Jot) List() ([]JournalInfo, error) {
	entries, err := os.ReadDir(j.root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	out := make([]JournalInfo, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") || e.Name() == backupDirName {
			continue
		}
		info := JournalInfo{Name: e.Name()}
		days, err := j.DaySummaries(e.Name())
		if err != nil {
			return nil, err
		}
		info.Days = len(days)
		for _, d := range days {
			info.Entries += d.Entries
			if d.Modified.After(info.Last) {
				info.Last = d.Modified
			}
		}
		out = append(out, info)
	}
	slices.SortFunc(out, func(a, b JournalInfo) int {
		return strings.Compare(a.Name, b.Name)
	})
	return out, nil
}

// Match is one search hit.
type Match struct {
	// Journal is the journal the hit was found in.
	Journal string
	// Day is the day label of the hit, YYYY-MM-DD.
	Day string
	// Line is the 1-based line number of the hit.
	Line int
	// Text is the matching line.
	Text string
}

// Search looks for a case-insensitive substring in every journal, newest day
// first within each journal.
func (j *Jot) Search(query string) ([]Match, error) {
	infos, err := j.List()
	if err != nil {
		return nil, err
	}
	var out []Match
	for _, info := range infos {
		matches, err := j.SearchJournal(info.Name, query)
		if err != nil {
			return nil, err
		}
		out = append(out, matches...)
	}
	return out, nil
}

// SearchJournal looks for a case-insensitive substring in one journal.
func (j *Jot) SearchJournal(name, query string) ([]Match, error) {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return nil, errors.New("empty search query")
	}
	days, err := j.Days(name)
	if err != nil {
		return nil, err
	}
	var out []Match
	for i := len(days) - 1; i >= 0; i-- {
		day := days[i]
		path := filepath.Join(j.root, j.journalName(name), day+".md")
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		lineNo := 0
		for scanner.Scan() {
			lineNo++
			line := scanner.Text()
			if strings.TrimSpace(line) == "" {
				continue
			}
			if strings.Contains(strings.ToLower(line), query) {
				out = append(out, Match{
					Journal: j.journalName(name),
					Day:     day,
					Line:    lineNo,
					Text:    strings.TrimSpace(line),
				})
			}
		}
		scanErr := scanner.Err()
		f.Close()
		if scanErr != nil {
			return nil, scanErr
		}
	}
	return out, nil
}

// countEntries counts "## " entry headings in a file.
func countEntries(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	n := 0
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "## ") {
			n++
		}
	}
	return n, scanner.Err()
}

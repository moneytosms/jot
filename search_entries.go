package jot

import (
	"bufio"
	"errors"
	"path/filepath"
	"slices"
	"strings"
)

// EntryMatch is one fuzzy entry hit.
type EntryMatch struct {
	// Journal is the journal the entry belongs to.
	Journal string
	// Day is the day label of the entry, YYYY-MM-DD.
	Day string
	// Time is the entry's "## HH:MM:SS" time, or "" for untimed content.
	Time string
	// Body is the full entry body, without the heading.
	Body string
	// Score is the fuzzy score of the entry, higher is better.
	Score int
}

// SearchEntries fuzzy-searches every entry in every journal, best score first.
// Ties break newest day first, then later times within a day. An empty or
// whitespace query is an error, like Search.
func (j *Jot) SearchEntries(query string) ([]EntryMatch, error) {
	if strings.TrimSpace(query) == "" {
		return nil, errors.New("empty search query")
	}
	infos, err := j.List()
	if err != nil {
		return nil, err
	}
	q := ParseQuery(query)
	var out []EntryMatch
	for _, info := range infos {
		if q.Journal != "" && info.Name != q.Journal {
			continue
		}
		matches, err := j.searchEntriesJournal(j.journalName(info.Name), q)
		if err != nil {
			return nil, err
		}
		out = append(out, matches...)
	}
	sortEntryMatches(out)
	return out, nil
}

// SearchEntriesJournal fuzzy-searches every entry in one journal, best score
// first. Ties break newest day first, then later times within a day. A missing
// journal wraps ErrNoJournal.
func (j *Jot) SearchEntriesJournal(name, query string) ([]EntryMatch, error) {
	if strings.TrimSpace(query) == "" {
		return nil, errors.New("empty search query")
	}
	return j.searchEntriesJournal(j.journalName(name), ParseQuery(query))
}

// searchEntriesJournal runs the fuzzy entry search for an already normalized
// journal name, without the empty-query check. Entries must pass q's filters;
// a query of filters only matches every passing entry with the same score.
func (j *Jot) searchEntriesJournal(name string, q Query) ([]EntryMatch, error) {
	days, err := j.Days(name)
	if err != nil {
		return nil, err
	}
	var out []EntryMatch
	for _, day := range days {
		if (q.After != "" && day < q.After) || (q.Before != "" && day > q.Before) {
			continue
		}
		path := filepath.Join(j.root, name, day+".md")
		f, err := openDayFile(path)
		if err != nil {
			return nil, err
		}
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		var time, body strings.Builder
		flush := func() {
			// Blank separator lines around the body are file formatting, not
			// entry content.
			if b := strings.Trim(body.String(), "\n"); b != "" && q.matches(name, day, b) {
				score := 1
				if q.Text != "" {
					score = FuzzyScore(q.Text, b+"\n"+time.String())
				}
				if score > 0 {
					out = append(out, EntryMatch{
						Journal: name,
						Day:     day,
						Time:    time.String(),
						Body:    b,
						Score:   score,
					})
				}
			}
			time.Reset()
			body.Reset()
		}
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "## ") {
				flush()
				time.WriteString(strings.TrimSpace(strings.TrimPrefix(line, "## ")))
				continue
			}
			body.WriteString(line)
			body.WriteByte('\n')
		}
		flush()
		scanErr := scanner.Err()
		f.Close()
		if scanErr != nil {
			return nil, scanErr
		}
	}
	sortEntryMatches(out)
	return out, nil
}

// sortEntryMatches orders matches by score, then newest day, then time.
func sortEntryMatches(matches []EntryMatch) {
	slices.SortFunc(matches, func(a, b EntryMatch) int {
		if a.Score != b.Score {
			return b.Score - a.Score
		}
		if a.Day != b.Day {
			return strings.Compare(b.Day, a.Day)
		}
		return strings.Compare(b.Time, a.Time)
	})
}

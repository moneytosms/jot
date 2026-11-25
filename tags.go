package jot

import (
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// tagLen returns the byte length of the tag word at the start of s (the text
// after '#'): a letter, then letters, digits, '_', '-' or '/', without a
// trailing '-' or '/'. 0 means s does not start with a tag.
func tagLen(s string) int {
	n := 0
	for i, r := range s {
		if !unicode.IsLetter(r) && (i == 0 || !(unicode.IsDigit(r) || r == '_' || r == '-' || r == '/')) {
			break
		}
		n = i + utf8.RuneLen(r)
	}
	return len(strings.TrimRight(s[:n], "-/"))
}

// isTagStart reports whether line[i] is a '#' that opens a tag: at the start
// of the line or after a space, followed by a tag word.
func isTagStart(line string, i int) bool {
	return line[i] == '#' && (i == 0 || line[i-1] == ' ' || line[i-1] == '\t' || line[i-1] == '(') &&
		tagLen(line[i+1:]) > 0
}

// Tags returns the distinct #tags in text, lowercased, in first-seen order.
// Markdown headings ("# Title") are not tags because a space follows the '#'.
func Tags(text string) []string {
	var out []string
	for _, line := range strings.Split(text, "\n") {
		for i := 0; i < len(line); i++ {
			if !isTagStart(line, i) {
				continue
			}
			n := tagLen(line[i+1:])
			if t := strings.ToLower(line[i+1 : i+1+n]); !slices.Contains(out, t) {
				out = append(out, t)
			}
			i += n
		}
	}
	return out
}

// Query is a parsed search: free text plus filters. Filter tokens are
// "tag:x" or "#x", "journal:x", "after:DAY", "before:DAY", "has:todo" and
// "is:open" (has an unchecked todo). DAY is YYYY-MM-DD, today, yesterday or
// Nd for N days ago.
type Query struct {
	Text    string
	Tags    []string
	Journal string
	After   string // inclusive YYYY-MM-DD
	Before  string // inclusive YYYY-MM-DD
	HasTodo bool
	Open    bool
}

// Filtered reports whether any filter is set.
func (q Query) Filtered() bool {
	return len(q.Tags) > 0 || q.Journal != "" || q.After != "" || q.Before != "" || q.HasTodo || q.Open
}

// ParseQuery splits a search string into text and filters. Unknown or
// malformed tokens stay part of the text.
func ParseQuery(s string) Query {
	var q Query
	var text []string
	for _, tok := range strings.Fields(s) {
		key, val, ok := strings.Cut(tok, ":")
		low := strings.ToLower(val)
		switch {
		case strings.HasPrefix(tok, "#") && tagLen(tok[1:]) == len(tok)-1:
			q.Tags = append(q.Tags, strings.ToLower(tok[1:]))
		case ok && key == "tag" && val != "":
			q.Tags = append(q.Tags, strings.ToLower(strings.TrimPrefix(val, "#")))
		case ok && (key == "journal" || key == "j") && val != "":
			q.Journal = SanitizeName(val)
		case ok && (key == "after" || key == "before") && queryDay(low) != "":
			if key == "after" {
				q.After = queryDay(low)
			} else {
				q.Before = queryDay(low)
			}
		case ok && key == "has" && low == "todo":
			q.HasTodo = true
		case ok && key == "is" && low == "open":
			q.Open = true
		default:
			text = append(text, tok)
		}
	}
	q.Text = strings.Join(text, " ")
	return q
}

// queryDay resolves a filter day: a date, today, yesterday or "Nd" (N days
// ago). It returns "" when s is none of those.
func queryDay(s string) string {
	if n, err := strconv.Atoi(strings.TrimSuffix(s, "d")); err == nil && strings.HasSuffix(s, "d") && n >= 0 {
		return time.Now().AddDate(0, 0, -n).Format("2006-01-02")
	}
	d, err := ParseDay(s)
	if err != nil {
		return ""
	}
	return d
}

// matches reports whether one entry passes the filters (not the text).
func (q Query) matches(journal, day, body string) bool {
	if q.Journal != "" && journal != q.Journal {
		return false
	}
	if (q.After != "" && day < q.After) || (q.Before != "" && day > q.Before) {
		return false
	}
	if len(q.Tags) > 0 {
		have := Tags(body)
		for _, t := range q.Tags {
			if !slices.Contains(have, t) {
				return false
			}
		}
	}
	if q.HasTodo || q.Open {
		found := false
		for _, line := range strings.Split(body, "\n") {
			if item, ok := parseTodo(line); ok && (!q.Open || !item.Done) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// TagCount is one tag and how many entries use it.
type TagCount struct {
	Tag   string
	Count int
}

// TagCounts counts the entries using each #tag in one journal, or every
// journal when name is "", most used first.
func (j *Jot) TagCounts(name string) ([]TagCount, error) {
	journals := []string{j.journalName(name)}
	if name == "" {
		infos, err := j.List()
		if err != nil {
			return nil, err
		}
		journals = journals[:0]
		for _, info := range infos {
			journals = append(journals, info.Name)
		}
	}
	counts := map[string]int{}
	for _, jn := range journals {
		days, err := j.ShowDays(jn)
		if err != nil {
			return nil, err
		}
		for _, d := range days {
			for _, e := range ParseEntries(d.Content) {
				for _, t := range Tags(e.Body) {
					counts[t]++
				}
			}
		}
	}
	out := make([]TagCount, 0, len(counts))
	for _, t := range slices.Sorted(maps.Keys(counts)) {
		out = append(out, TagCount{Tag: t, Count: counts[t]})
	}
	slices.SortStableFunc(out, func(a, b TagCount) int { return b.Count - a.Count })
	return out, nil
}

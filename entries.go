package jot

import "strings"

// ParsedEntry is one section of journal markdown as split by ParseEntries.
type ParsedEntry struct {
	// Time is the heading text: "HH:MM:SS" when the heading parses as a
	// timestamp, otherwise the full text after "## ". It is empty for the
	// preamble before the first heading.
	Time string
	// Body is the entry text with leading and trailing blank lines removed
	// and internal formatting preserved.
	Body string
	// StartLine is the 1-based line number of the "## " heading, or 0 for
	// the preamble.
	StartLine int
}

// ParseEntries splits journal markdown into entries: every line starting with
// "## " begins a new entry whose body runs to the next such heading. Content
// before the first heading becomes a preamble entry with an empty Time and
// StartLine 0. Hand-edited files parse fine: headings may hold any text,
// blank lines may be missing, trailing whitespace and carriage returns are
// ignored, and deeper headings (###) stay inside the body. Empty or
// whitespace-only content yields nil.
func ParseEntries(content string) []ParsedEntry {
	if strings.TrimSpace(content) == "" {
		return nil
	}
	raw := strings.Split(content, "\n")
	lines := make([]string, len(raw))
	for i, l := range raw {
		lines[i] = strings.TrimSuffix(l, "\r")
	}
	out := make([]ParsedEntry, 0, 8)
	var (
		curTime  string
		curStart int
		curBody  []string
	)
	flush := func() {
		body := trimBlankLines(curBody)
		if body == "" && curTime == "" && curStart == 0 {
			return // a preamble with no content is not an entry
		}
		out = append(out, ParsedEntry{Time: curTime, Body: body, StartLine: curStart})
	}
	for i, line := range lines {
		switch {
		case strings.HasPrefix(line, "## "):
			flush()
			curTime = strings.TrimSpace(line[3:])
			curStart = i + 1
			curBody = curBody[:0]
		case strings.TrimRight(line, " \t") == "##":
			flush()
			curTime = ""
			curStart = i + 1
			curBody = curBody[:0]
		default:
			curBody = append(curBody, line)
		}
	}
	flush()
	return out
}

// trimBlankLines joins body with newlines after removing blank lines from
// both ends. Internal lines are preserved as they are.
func trimBlankLines(body []string) string {
	start, end := 0, len(body)
	for start < end && strings.TrimSpace(body[start]) == "" {
		start++
	}
	for end > start && strings.TrimSpace(body[end-1]) == "" {
		end--
	}
	return strings.Join(body[start:end], "\n")
}

package jot

import (
	"html"
	"regexp"
	"strings"
)

// MarkdownHTML renders journal markdown to HTML: headings, paragraphs with
// hard breaks, nested ordered, unordered and task lists, blockquotes, fenced
// code, rules, pipe tables and inline bold, italic, code, strikethrough,
// links, autolinks and #tags. All text is escaped; links only keep http,
// https, mailto and relative targets.
func MarkdownHTML(md string) string {
	var b strings.Builder
	blocksHTML(&b, splitLines(md))
	return b.String()
}

var tableSep = regexp.MustCompile(`^\|?\s*:?-+:?\s*(\|\s*:?-+:?\s*)*\|?$`)

// isTableStart reports whether lines[i] opens a pipe table.
func isTableStart(lines []string, i int) bool {
	return strings.HasPrefix(strings.TrimSpace(lines[i]), "|") && i+1 < len(lines) &&
		tableSep.MatchString(strings.TrimSpace(lines[i+1]))
}

// startsBlock reports whether lines[i] starts something other than
// paragraph text, which ends a paragraph.
func startsBlock(lines []string, i int) bool {
	l := lines[i]
	t := strings.TrimSpace(l)
	return t == "" || isHeading(l) || isRule(t) || isFence(l) || isQuote(l) || isListItem(l) || isTableStart(lines, i)
}

func blocksHTML(b *strings.Builder, lines []string) {
	for i := 0; i < len(lines); {
		line := lines[i]
		t := strings.TrimSpace(line)
		switch {
		case t == "":
			i++
		case isFence(line):
			marker, _ := fenceOf(line)
			i++
			var code []string
			for i < len(lines) && !fenceClose(lines[i], marker) {
				code = append(code, lines[i])
				i++
			}
			i++ // the closing fence, if any
			b.WriteString("<pre><code>" + html.EscapeString(strings.Join(code, "\n")) + "</code></pre>\n")
		case isHeading(line):
			level, text := parseHeading(line)
			tag := "h5"
			if level <= 2 {
				tag = "h4"
			}
			b.WriteString("<" + tag + ">" + inlineHTML(text) + "</" + tag + ">\n")
			i++
		case isRule(t):
			b.WriteString("<hr>\n")
			i++
		case isQuote(line):
			var inner []string
			for i < len(lines) && isQuote(lines[i]) {
				q := strings.TrimLeft(lines[i], " ")
				inner = append(inner, strings.TrimPrefix(q[1:], " "))
				i++
			}
			b.WriteString("<blockquote>")
			blocksHTML(b, inner)
			b.WriteString("</blockquote>\n")
		case isTableStart(lines, i):
			b.WriteString("<table><thead><tr>")
			for _, c := range tableCells(line) {
				b.WriteString("<th>" + inlineHTML(c) + "</th>")
			}
			b.WriteString("</tr></thead><tbody>")
			for i += 2; i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), "|"); i++ {
				b.WriteString("<tr>")
				for _, c := range tableCells(lines[i]) {
					b.WriteString("<td>" + inlineHTML(c) + "</td>")
				}
				b.WriteString("</tr>")
			}
			b.WriteString("</tbody></table>\n")
		case isListItem(line):
			items, next := collectList(lines, i)
			listHTML(b, items)
			i = next
		default:
			var para []string
			for i < len(lines) && (len(para) == 0 || !startsBlock(lines, i)) {
				l := lines[i]
				text := inlineHTML(strings.TrimSpace(l))
				if strings.HasSuffix(l, "  ") {
					text += "<br>"
				}
				para = append(para, text)
				i++
			}
			b.WriteString("<p>" + strings.Join(para, "\n") + "</p>\n")
		}
	}
}

// tableCells splits a pipe table row into trimmed cells.
func tableCells(line string) []string {
	t := strings.Trim(strings.TrimSpace(line), "|")
	cells := strings.Split(t, "|")
	for i := range cells {
		cells[i] = strings.TrimSpace(cells[i])
	}
	return cells
}

// listHTML nests list items by indentation.
func listHTML(b *strings.Builder, items []parsedItem) {
	type level struct {
		indent int
		tag    string
	}
	var stack []level
	for _, it := range items {
		for len(stack) > 0 && it.indent < stack[len(stack)-1].indent {
			b.WriteString("</li></" + stack[len(stack)-1].tag + ">")
			stack = stack[:len(stack)-1]
		}
		tag := "ul"
		if it.ordered {
			tag = "ol"
		}
		if n := len(stack); n > 0 && it.indent == stack[n-1].indent && tag != stack[n-1].tag {
			// A bullet list followed by a numbered one (or back) at one level.
			b.WriteString("</li></" + stack[n-1].tag + ">")
			stack = stack[:n-1]
		}
		if len(stack) == 0 || it.indent > stack[len(stack)-1].indent {
			b.WriteString("<" + tag + ">")
			stack = append(stack, level{it.indent, tag})
		} else {
			b.WriteString("</li>")
		}
		text, class := it.text, ""
		if !it.ordered {
			switch low := strings.ToLower(text); {
			case strings.HasPrefix(low, "[ ] ") || low == "[ ]":
				text, class = strings.TrimSpace(text[3:]), ` class="task"`
			case strings.HasPrefix(low, "[x] ") || low == "[x]":
				text, class = strings.TrimSpace(text[3:]), ` class="task done"`
			}
		}
		b.WriteString("<li" + class + ">" + inlineHTML(text))
	}
	for len(stack) > 0 {
		b.WriteString("</li></" + stack[len(stack)-1].tag + ">")
		stack = stack[:len(stack)-1]
	}
	b.WriteString("\n")
}

// safeURL reports whether a link target may be emitted.
func safeURL(u string) bool {
	low := strings.ToLower(u)
	if strings.HasPrefix(low, "http://") || strings.HasPrefix(low, "https://") || strings.HasPrefix(low, "mailto:") {
		return true
	}
	return !strings.Contains(strings.SplitN(low, "/", 2)[0], ":") // relative
}

var bareURL = regexp.MustCompile(`^https?://[^\s<>"]+`)

// inlineHTML renders inline markdown spans, escaping all text.
func inlineHTML(s string) string {
	var b strings.Builder
	esc := html.EscapeString
	wrap := func(open, close string, at, n int, i *int) bool {
		end := strings.Index(s[at+n:], close)
		if end <= 0 {
			return false
		}
		b.WriteString(open + inlineHTML(s[at+n:at+n+end]) + closeTags(open))
		*i = at + n + end + len(close)
		return true
	}
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == '\\' && i+1 < len(s) && strings.ContainsRune("\\`*_~[]#<>", rune(s[i+1])):
			b.WriteString(esc(s[i+1 : i+2]))
			i += 2
			continue
		case c == '`':
			if end := strings.IndexByte(s[i+1:], '`'); end >= 0 {
				b.WriteString("<code>" + esc(s[i+1:i+1+end]) + "</code>")
				i += end + 2
				continue
			}
		case strings.HasPrefix(s[i:], "~~"):
			if wrap("<del>", "~~", i, 2, &i) {
				continue
			}
		case strings.HasPrefix(s[i:], "***"):
			if wrap("<strong><em>", "***", i, 3, &i) {
				continue
			}
		case strings.HasPrefix(s[i:], "**"):
			if wrap("<strong>", "**", i, 2, &i) {
				continue
			}
		case c == '*' && i+1 < len(s) && s[i+1] != ' ':
			if wrap("<em>", "*", i, 1, &i) {
				continue
			}
		case c == '[':
			if close := strings.Index(s[i:], "]("); close > 0 {
				if end := strings.IndexByte(s[i+close+2:], ')'); end >= 0 {
					text, url := s[i+1:i+close], strings.TrimSpace(s[i+close+2:i+close+2+end])
					if safeURL(url) {
						b.WriteString(`<a href="` + esc(url) + `">` + inlineHTML(text) + "</a>")
						i += close + 3 + end
						continue
					}
				}
			}
		case c == '<':
			if end := strings.IndexByte(s[i:], '>'); end > 0 && bareURL.MatchString(s[i+1:i+end]) {
				url := s[i+1 : i+end]
				b.WriteString(`<a href="` + esc(url) + `">` + esc(url) + "</a>")
				i += end + 1
				continue
			}
		case c == 'h' && (i == 0 || s[i-1] == ' ' || s[i-1] == '('):
			if url := bareURL.FindString(s[i:]); url != "" {
				url = strings.TrimRight(url, ".,;:!?)")
				b.WriteString(`<a href="` + esc(url) + `">` + esc(url) + "</a>")
				i += len(url)
				continue
			}
		case isTagStart(s, i):
			n := tagLen(s[i+1:]) + 1
			b.WriteString(`<span class="tag">` + esc(s[i:i+n]) + "</span>")
			i += n
			continue
		}
		b.WriteString(esc(s[i : i+1]))
		i++
	}
	return b.String()
}

// closeTags returns the closing tags for a run of opening tags.
func closeTags(open string) string {
	parts := strings.Split(strings.Trim(open, "<>"), "><")
	var b strings.Builder
	for i := len(parts) - 1; i >= 0; i-- {
		b.WriteString("</" + parts[i] + ">")
	}
	return b.String()
}

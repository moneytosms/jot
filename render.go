package jot

import (
	"strings"
	"unicode/utf8"
)

// ansiReset returns the terminal to normal text after a styled span.
const ansiReset = "\x1b[0m"

// quoteMarker prefixes every rendered blockquote line.
const quoteMarker = "│ "

// MDStyles holds the ANSI escape prefixes used by RenderStyled. Every field
// is written before the styled text and followed by a reset; an empty field
// disables that style and emits no escape sequences at all.
type MDStyles struct {
	// Heading1 styles "# " headings.
	Heading1 string
	// Heading2 styles "## " headings, the journal entry format.
	Heading2 string
	// Heading3 styles "### " headings and deeper.
	Heading3 string
	// Bold styles **bold** spans.
	Bold string
	// Italic styles *italic* spans.
	Italic string
	// Code styles `code` spans.
	Code string
	// Strike styles ~~strikethrough~~ spans.
	Strike string
	// Quote styles blockquote markers and text.
	Quote string
	// Dim styles horizontal rules and code blocks.
	Dim string
	// ListBullet styles list markers.
	ListBullet string
	// Accent styles ***bold italic*** spans.
	Accent string
	// Tag styles #tags.
	Tag string
}

// PlainStyles returns styles with every field empty, so RenderStyled emits no
// escape sequences.
func PlainStyles() MDStyles { return MDStyles{} }

// DefaultDarkStyles returns styles tuned for terminals with a dark
// background.
func DefaultDarkStyles() MDStyles {
	return MDStyles{
		Heading1:   "\x1b[1m",
		Heading2:   "\x1b[1;36m",
		Heading3:   "\x1b[36m",
		Bold:       "\x1b[1m",
		Italic:     "\x1b[3m",
		Code:       "\x1b[33m",
		Strike:     "\x1b[9m",
		Quote:      "\x1b[2m",
		Dim:        "\x1b[2m",
		ListBullet: "\x1b[36m",
		Accent:     "\x1b[1;36m",
		Tag:        "\x1b[35m",
	}
}

// DefaultLightStyles returns styles tuned for terminals with a light
// background.
func DefaultLightStyles() MDStyles {
	return MDStyles{
		Heading1:   "\x1b[1m",
		Heading2:   "\x1b[1;34m",
		Heading3:   "\x1b[34m",
		Bold:       "\x1b[1m",
		Italic:     "\x1b[3m",
		Code:       "\x1b[32m",
		Strike:     "\x1b[9m",
		Quote:      "\x1b[2m",
		Dim:        "\x1b[2m",
		ListBullet: "\x1b[34m",
		Accent:     "\x1b[1;34m",
		Tag:        "\x1b[35m",
	}
}

// RenderStyled renders journal-flavored markdown into terminal text. ATX
// headings (# to ######), inline **bold**, *italic*, `code`,
// ~~strikethrough~~ and ***bold italic*** spans, nested unordered and ordered
// lists, blockquotes, fenced and indented code blocks and horizontal rules
// (---, ***, ___) are recognized; table lines starting with "|" and anything
// else pass through untouched. Each non-empty style field of s is written
// before its text and followed by a reset; empty fields emit no escape
// sequences. A positive width word-wraps paragraph and list text and draws
// horizontal rules full width; headings and code lines are never wrapped.
// Inline styles never cross a line boundary, so markers left unbalanced or
// split by wrapping are rendered literally.
func RenderStyled(md string, width int, s MDStyles) string {
	lines := splitLines(md)
	out := make([]string, 0, len(lines))
	for i := 0; i < len(lines); {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		prevBlank := i == 0 || strings.TrimSpace(lines[i-1]) == ""
		switch {
		case trimmed == "":
			for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
				i++
			}
			if i < len(lines) && len(out) > 0 && out[len(out)-1] != "" {
				out = append(out, "")
			}
		case isHeading(line):
			level, text := parseHeading(line)
			style := s.Heading3
			switch level {
			case 1:
				style = s.Heading1
			case 2:
				style = s.Heading2
			}
			out = append(out, styled(style, inlineStr(text, s)))
			i++
		case isRule(trimmed):
			if width > 0 {
				out = append(out, styled(s.Dim, strings.Repeat("─", width)))
			} else {
				out = append(out, styled(s.Dim, trimmed))
			}
			i++
		case isQuote(line):
			var inner []string
			for i < len(lines) {
				t := strings.TrimLeft(lines[i], " ")
				if !strings.HasPrefix(t, ">") {
					break
				}
				inner = append(inner, strings.TrimPrefix(t[1:], " "))
				i++
			}
			for len(inner) > 0 && strings.TrimSpace(inner[len(inner)-1]) == "" {
				inner = inner[:len(inner)-1]
			}
			if len(inner) == 0 {
				continue
			}
			avail := 0
			if width > 0 {
				avail = width - utf8.RuneCountInString(quoteMarker)
			}
			rendered := RenderStyled(strings.Join(inner, "\n"), avail, s)
			for _, l := range strings.Split(rendered, "\n") {
				out = append(out, styled(s.Quote, quoteMarker+l))
			}
		default:
			switch {
			case isListItem(line):
				items, next := collectList(lines, i)
				out = append(out, renderList(items, width, s)...)
				i = next
			case prevBlank && isIndentedCode(line):
				var code []string
				for i < len(lines) {
					l := lines[i]
					if strings.TrimSpace(l) == "" {
						// A blank run stays with the code block only when
						// more code follows; otherwise it separates blocks.
						j := i
						for j < len(lines) && strings.TrimSpace(lines[j]) == "" {
							j++
						}
						if j < len(lines) && isIndentedCode(lines[j]) {
							for k := i; k < j; k++ {
								code = append(code, "")
							}
							i = j
							continue
						}
						break
					}
					if !isIndentedCode(l) {
						break
					}
					code = append(code, dedentCode(l))
					i++
				}
				for len(code) > 0 && strings.TrimSpace(code[len(code)-1]) == "" {
					code = code[:len(code)-1]
				}
				for _, c := range code {
					out = append(out, styled(s.Dim, c))
				}
			case isFence(line):
				marker, _ := fenceOf(line)
				i++
				var code []string
				for i < len(lines) && !fenceClose(lines[i], marker) {
					code = append(code, lines[i])
					i++
				}
				if i < len(lines) {
					i++ // closing fence
				}
				for len(code) > 0 && strings.TrimSpace(code[len(code)-1]) == "" {
					code = code[:len(code)-1]
				}
				for _, c := range code {
					out = append(out, styled(s.Dim, c))
				}
			case strings.HasPrefix(trimmed, "|"):
				out = append(out, line) // tables and similar stay untouched
				i++
			default:
				for _, w := range wrapText(trimmed, width) {
					out = append(out, inlineStr(w, s))
				}
				i++
			}
		}
	}
	return strings.Join(out, "\n")
}

// splitLines splits markdown into lines without trailing carriage returns.
func splitLines(md string) []string {
	lines := strings.Split(md, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimSuffix(l, "\r")
	}
	return lines
}

// fenceOf reports whether a line opens a fenced code block and returns its
// marker, "```" or "~~~".
func fenceOf(line string) (string, bool) {
	t := strings.TrimSpace(line)
	if strings.HasPrefix(t, "```") {
		return "```", true
	}
	if strings.HasPrefix(t, "~~~") {
		return "~~~", true
	}
	return "", false
}

// isFence reports whether a line opens a fenced code block.
func isFence(line string) bool {
	_, ok := fenceOf(line)
	return ok
}

// fenceClose reports whether a line closes a fenced code block opened with
// marker: only the marker character, repeated at least three times.
func fenceClose(line, marker string) bool {
	t := strings.TrimSpace(line)
	if len(t) < len(marker) {
		return false
	}
	for _, r := range t {
		if r != rune(marker[0]) {
			return false
		}
	}
	return true
}

// isHeading reports whether a line is an ATX heading: one to six '#' after up
// to three spaces, followed by a space or the end of the line.
func isHeading(line string) bool {
	indent := leadingSpaces(line)
	if indent >= 4 {
		return false
	}
	t := line[indent:]
	n := 0
	for n < len(t) && t[n] == '#' {
		n++
	}
	if n == 0 || n > 6 {
		return false
	}
	return n == len(t) || t[n] == ' ' || t[n] == '\t'
}

// parseHeading splits an ATX heading line into its level (number of #) and
// text. The caller has verified the line with isHeading.
func parseHeading(line string) (level int, text string) {
	indent := leadingSpaces(line)
	t := line[indent:]
	for level < len(t) && t[level] == '#' {
		level++
	}
	return level, strings.TrimSpace(t[level:])
}

// isRule reports whether a trimmed line is a horizontal rule.
func isRule(trimmed string) bool {
	return trimmed == "---" || trimmed == "***" || trimmed == "___"
}

// isQuote reports whether a line is a blockquote line, as long as it is not
// indented into a code block.
func isQuote(line string) bool {
	if leadingSpaces(line) >= 4 {
		return false
	}
	return strings.HasPrefix(strings.TrimLeft(line, " "), ">")
}

// leadingSpaces counts the spaces at the start of a line. A tab counts as a
// full four-space indent, which callers treat as code.
func leadingSpaces(line string) int {
	n := 0
	for _, r := range line {
		switch r {
		case ' ':
			n++
		case '\t':
			return n + 4
		default:
			return n
		}
	}
	return n
}

// isIndentedCode reports whether a line is indented like a code block: four
// spaces or a tab.
func isIndentedCode(line string) bool {
	return strings.HasPrefix(line, "    ") || strings.HasPrefix(line, "\t")
}

// dedentCode removes the four-space or one-tab indent of a code line.
func dedentCode(line string) string {
	if strings.HasPrefix(line, "\t") {
		return line[1:]
	}
	return strings.TrimPrefix(line, "    ")
}

// parsedItem is one list item line collected by collectList.
type parsedItem struct {
	indent  int    // leading spaces of the line
	ordered bool   // true for "1." and "1)" items
	marker  string // original ordered marker; empty for bullets
	text    string // item text after the marker
}

// parseListItem recognizes one list item line: "- ", "* ", "1. " or "1) ".
func parseListItem(line string) (parsedItem, bool) {
	indent := leadingSpaces(line)
	if indent >= 4 {
		return parsedItem{}, false
	}
	t := line[indent:]
	var it parsedItem
	it.indent = indent
	switch {
	case strings.HasPrefix(t, "- "), strings.HasPrefix(t, "* "):
		it.text = strings.TrimSpace(t[2:])
	default:
		d := 0
		for d < len(t) && t[d] >= '0' && t[d] <= '9' {
			d++
		}
		if d == 0 || d >= len(t) || (t[d] != '.' && t[d] != ')') ||
			d+1 >= len(t) || (t[d+1] != ' ' && t[d+1] != '\t') {
			return parsedItem{}, false
		}
		it.ordered = true
		it.marker = t[:d+1]
		it.text = strings.TrimSpace(t[d+1:])
	}
	return it, true
}

// isListItem reports whether a line starts a list item.
func isListItem(line string) bool {
	_, ok := parseListItem(line)
	return ok
}

// isListContinuation reports whether a line is indented text continuing a
// list item rather than starting a new block.
func isListContinuation(line string) bool {
	if leadingSpaces(line) < 2 {
		return false
	}
	t := strings.TrimSpace(line)
	if t == "" || strings.HasPrefix(t, ">") || strings.HasPrefix(t, "#") || isRule(t) {
		return false
	}
	if _, ok := fenceOf(t); ok {
		return false
	}
	_, ok := parseListItem(line)
	return !ok
}

// collectList gathers a run of list items and their indented continuation
// lines, starting at lines[i]. It returns the items and the index of the
// first line that is not part of the list.
func collectList(lines []string, i int) ([]parsedItem, int) {
	var items []parsedItem
	for i < len(lines) {
		line := lines[i]
		if item, ok := parseListItem(line); ok {
			items = append(items, item)
			i++
			continue
		}
		if len(items) > 0 && isListContinuation(line) {
			items[len(items)-1].text += " " + strings.TrimSpace(line)
			i++
			continue
		}
		if strings.TrimSpace(line) == "" {
			// A blank run continues the list only if list content follows.
			j := i
			for j < len(lines) && strings.TrimSpace(lines[j]) == "" {
				j++
			}
			if j < len(lines) {
				_, okItem := parseListItem(lines[j])
				if okItem || isListContinuation(lines[j]) {
					i = j
					continue
				}
			}
		}
		break
	}
	return items, i
}

// bullets are the markers for unordered list levels zero, one and deeper.
var bullets = [...]string{"•", "◦", "▪"}

// renderList renders parsed list items with nested indentation and wrapped
// text. Markers are styled with s.ListBullet; the text carries inline styles.
func renderList(items []parsedItem, width int, s MDStyles) []string {
	var out []string
	if len(items) == 0 {
		return out
	}
	base := items[0].indent
	for _, it := range items {
		level := (it.indent - base) / 2
		if level < 0 {
			level = 0
		}
		marker := it.marker
		text := it.text
		done := false
		if !it.ordered {
			if level >= len(bullets) {
				level = len(bullets) - 1
			}
			marker = bullets[level]
			// Task items: the checkbox replaces the bullet; done ones are dimmed.
			switch {
			case strings.HasPrefix(text, "[ ] ") || text == "[ ]":
				marker, text = "☐", strings.TrimSpace(text[3:])
			case strings.HasPrefix(strings.ToLower(text), "[x] ") || strings.EqualFold(text, "[x]"):
				marker, text, done = "☑", strings.TrimSpace(text[3:]), true
			}
		}
		pad := it.indent + utf8.RuneCountInString(marker) + 1
		prefix := strings.Repeat(" ", it.indent) + styled(s.ListBullet, marker) + " "
		avail := 0
		if width > 0 {
			avail = width - pad
		}
		for n, w := range wrapText(text, avail) {
			line := inlineStr(w, s)
			if done {
				line = styled(s.Dim, w)
			}
			if n == 0 {
				out = append(out, prefix+line)
			} else {
				out = append(out, strings.Repeat(" ", pad)+line)
			}
		}
	}
	return out
}

// wrapText word-wraps a single line of text to width cells, breaking words
// that are longer than the line. width <= 0 returns the text unwrapped.
func wrapText(text string, width int) []string {
	if width <= 0 || strings.TrimSpace(text) == "" {
		return []string{text}
	}
	var out []string
	line, lineLen := "", 0
	for _, word := range strings.Fields(text) {
		runes := []rune(word)
		for len(runes) > width {
			out = append(out, string(runes[:width]))
			runes = runes[width:]
		}
		word = string(runes)
		w := len([]rune(word))
		switch {
		case lineLen == 0:
			line, lineLen = word, w
		case lineLen+1+w <= width:
			line += " " + word
			lineLen += 1 + w
		default:
			out = append(out, line)
			line, lineLen = word, w
		}
	}
	out = append(out, line)
	return out
}

// styled wraps text in an ANSI prefix and a reset. An empty prefix returns
// the text unchanged, so disabled styles emit no escape sequences.
func styled(prefix, text string) string {
	if prefix == "" {
		return text
	}
	return prefix + text + ansiReset
}

// inlineStr renders one line of text with the inline styles in s applied.
func inlineStr(line string, s MDStyles) string {
	var b strings.Builder
	writeInline(&b, line, s)
	return b.String()
}

// writeInline appends line to b with inline spans styled: `code`, ~~strike~~,
// ***bold italic***, **bold** and *italic*, in that order of precedence.
// Markers without a closer on the same line are copied literally.
func writeInline(b *strings.Builder, line string, s MDStyles) {
	for i := 0; i < len(line); {
		switch {
		case line[i] == '`':
			if end := strings.IndexByte(line[i+1:], '`'); end >= 0 {
				b.WriteString(styled(s.Code, line[i+1:i+1+end]))
				i += end + 2
			} else {
				b.WriteByte('`')
				i++
			}
		case strings.HasPrefix(line[i:], "~~"):
			if end := strings.Index(line[i+2:], "~~"); end >= 0 {
				b.WriteString(styled(s.Strike, inlineStr(line[i+2:i+2+end], s)))
				i += end + 4
			} else {
				b.WriteString("~~")
				i += 2
			}
		case strings.HasPrefix(line[i:], "***"):
			if end := strings.Index(line[i+3:], "***"); end >= 0 {
				b.WriteString(styled(s.Accent, inlineStr(line[i+3:i+3+end], s)))
				i += end + 6
			} else {
				b.WriteByte('*')
				i++
			}
		case strings.HasPrefix(line[i:], "**"):
			if end := strings.Index(line[i+2:], "**"); end >= 0 {
				b.WriteString(styled(s.Bold, inlineStr(line[i+2:i+2+end], s)))
				i += end + 4
			} else {
				b.WriteString("**")
				i += 2
			}
		case line[i] == '*':
			if i+1 < len(line) && line[i+1] != ' ' {
				if end := strings.IndexByte(line[i+1:], '*'); end > 0 {
					b.WriteString(styled(s.Italic, inlineStr(line[i+1:i+1+end], s)))
					i += end + 2
					continue
				}
			}
			b.WriteByte('*')
			i++
		case isTagStart(line, i):
			n := tagLen(line[i+1:]) + 1
			b.WriteString(styled(s.Tag, line[i:i+n]))
			i += n
		default:
			j := nextMarker(line, i)
			b.WriteString(line[i:j])
			i = j
		}
	}
}

// nextMarker returns the index of the next inline marker in line at or after
// i: a backtick, an asterisk or a double tilde. It returns len(line) when
// none remain.
func nextMarker(line string, i int) int {
	for j := i; j < len(line); j++ {
		switch line[j] {
		case '`', '*':
			return j
		case '#':
			if j > i && isTagStart(line, j) {
				return j
			}
		case '~':
			if j+1 < len(line) && line[j+1] == '~' {
				return j
			}
		}
	}
	return len(line)
}

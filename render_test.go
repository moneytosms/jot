package jot

import (
	"strings"
	"testing"
)

func TestRenderStyled_PlainStructure(t *testing.T) {
	md := "## 10:00:00\n\nHello **world**.\n\n- one\n- two\n  - deep\n\n> quoted line\n\n---\n"
	want := "10:00:00\n\nHello world.\n\n• one\n• two\n  ◦ deep\n\n│ quoted line\n\n---"
	if got := RenderStyled(md, 0, PlainStyles()); got != want {
		t.Fatalf("RenderStyled() =\n%q\nwant\n%q", got, want)
	}
}

func TestRenderStyled_PlainInline(t *testing.T) {
	md := "mix **b** *i* `c` ~~s~~ ***bi***."
	want := "mix b i c s bi."
	if got := RenderStyled(md, 0, PlainStyles()); got != want {
		t.Fatalf("RenderStyled() = %q, want %q", got, want)
	}
}

func TestRenderStyled_PlainHasNoEscapes(t *testing.T) {
	md := "# Title\n\n**bold**, *italic*, `code`, ~~gone~~\n\n- item\n\n> quote\n\n```\ncode\n```\n\n---\n"
	got := RenderStyled(md, 0, PlainStyles())
	if strings.Contains(got, "\x1b") {
		t.Fatalf("PlainStyles emitted escape sequences:\n%q", got)
	}
}

func TestRenderStyled_HeadingStyles(t *testing.T) {
	md := "# Title\n## Sub\n### Deep"
	want := "\x1b[1mTitle\x1b[0m\n\x1b[1;36mSub\x1b[0m\n\x1b[36mDeep\x1b[0m"
	if got := RenderStyled(md, 0, DefaultDarkStyles()); got != want {
		t.Fatalf("RenderStyled() = %q, want %q", got, want)
	}
}

func TestRenderStyled_InlineStyles(t *testing.T) {
	md := "**b** *i* `c` ~~s~~ ***bi***"
	want := "\x1b[1mb\x1b[0m \x1b[3mi\x1b[0m \x1b[33mc\x1b[0m \x1b[9ms\x1b[0m \x1b[1;36mbi\x1b[0m"
	if got := RenderStyled(md, 0, DefaultDarkStyles()); got != want {
		t.Fatalf("RenderStyled() = %q, want %q", got, want)
	}
}

func TestRenderStyled_LightStyles(t *testing.T) {
	md := "## 10:00:00\n\n- item"
	want := "\x1b[1;34m10:00:00\x1b[0m\n\n\x1b[34m•\x1b[0m item"
	if got := RenderStyled(md, 0, DefaultLightStyles()); got != want {
		t.Fatalf("RenderStyled() = %q, want %q", got, want)
	}
}

func TestRenderStyled_PartialStyles(t *testing.T) {
	s := MDStyles{Bold: "\x1b[1m"}
	md := "plain **bold** `code`"
	want := "plain \x1b[1mbold\x1b[0m code"
	if got := RenderStyled(md, 0, s); got != want {
		t.Fatalf("RenderStyled() = %q, want %q", got, want)
	}
}

func TestRenderStyled_UnbalancedMarkersLiteral(t *testing.T) {
	tests := []struct {
		name string
		md   string
		want string
	}{
		{name: "unclosed bold", md: "**open", want: "**open"},
		{name: "unclosed italic", md: "a * b * c", want: "a * b * c"},
		{name: "bold literal then italic", md: "open ** and *close*", want: "open ** and close"},
		{name: "unclosed code", md: "a `b c", want: "a `b c"},
		{name: "unclosed strike", md: "~~gone", want: "~~gone"},
		{name: "stray closers", md: "end ** and *", want: "end ** and *"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RenderStyled(tt.md, 0, PlainStyles()); got != tt.want {
				t.Fatalf("RenderStyled() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRenderStyled_NoCrossLineStyles(t *testing.T) {
	md := "start **bold\nacross lines**"
	want := "start **bold\nacross lines**"
	if got := RenderStyled(md, 0, PlainStyles()); got != want {
		t.Fatalf("RenderStyled() = %q, want %q", got, want)
	}
}

func TestRenderStyled_Wrapping(t *testing.T) {
	tests := []struct {
		name  string
		md    string
		width int
		want  string
	}{
		{
			name:  "word wrap",
			md:    "one two three four",
			width: 7,
			want:  "one two\nthree\nfour",
		},
		{
			name:  "exact fit stays on line",
			md:    "one two three",
			width: 7,
			want:  "one two\nthree",
		},
		{
			name:  "long word hard broken",
			md:    "abcdefghijkl",
			width: 5,
			want:  "abcde\nfghij\nkl",
		},
		{
			name:  "zero width no wrap",
			md:    "one two three four",
			width: 0,
			want:  "one two three four",
		},
		{
			name:  "negative width no wrap",
			md:    "one two three four",
			width: -3,
			want:  "one two three four",
		},
		{
			name:  "paragraphs separated",
			md:    "aaa bbb\n\nccc ddd",
			width: 7,
			want:  "aaa bbb\n\nccc ddd",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RenderStyled(tt.md, tt.width, PlainStyles()); got != tt.want {
				t.Fatalf("RenderStyled() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRenderStyled_HeadingAndCodeNotWrapped(t *testing.T) {
	long := strings.Repeat("word ", 12)
	md := "## " + long + "\n\n```\n" + long + "\n```\n"
	got := RenderStyled(md, 10, PlainStyles())
	lines := strings.Split(got, "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d:\n%q", len(lines), got)
	}
	if lines[0] != strings.TrimRight(long, " ") {
		t.Fatalf("heading was wrapped: %q", lines[0])
	}
	if lines[2] != long {
		t.Fatalf("code line was wrapped or trimmed: %q", lines[2])
	}
}

func TestRenderStyled_ListWrapping(t *testing.T) {
	md := "- one two\n  - deep three four"
	want := "• one two\n  ◦ deep\n    three\n    four"
	if got := RenderStyled(md, 10, PlainStyles()); got != want {
		t.Fatalf("RenderStyled() = %q, want %q", got, want)
	}
}

func TestRenderStyled_OrderedLists(t *testing.T) {
	md := "1. first\n2. second\n10. tenth"
	want := "1. first\n2. second\n10. tenth"
	if got := RenderStyled(md, 0, PlainStyles()); got != want {
		t.Fatalf("RenderStyled() = %q, want %q", got, want)
	}
}

func TestRenderStyled_OrderedListContinuation(t *testing.T) {
	md := "1. first line\n   more detail"
	want := "1. first line more detail"
	if got := RenderStyled(md, 0, PlainStyles()); got != want {
		t.Fatalf("RenderStyled() = %q, want %q", got, want)
	}
}

func TestRenderStyled_ListBulletStyled(t *testing.T) {
	md := "- item\n1. ordered"
	want := "\x1b[36m•\x1b[0m item\n\x1b[36m1.\x1b[0m ordered"
	if got := RenderStyled(md, 0, DefaultDarkStyles()); got != want {
		t.Fatalf("RenderStyled() = %q, want %q", got, want)
	}
}

func TestRenderStyled_Blockquotes(t *testing.T) {
	md := "> first\n> second\n\n> > nested"
	want := "│ first\n│ second\n\n│ │ nested"
	if got := RenderStyled(md, 0, PlainStyles()); got != want {
		t.Fatalf("RenderStyled() = %q, want %q", got, want)
	}
}

func TestRenderStyled_QuoteStyled(t *testing.T) {
	md := "> quoted"
	want := "\x1b[2m│ quoted\x1b[0m"
	if got := RenderStyled(md, 0, DefaultDarkStyles()); got != want {
		t.Fatalf("RenderStyled() = %q, want %q", got, want)
	}
}

func TestRenderStyled_HorizontalRules(t *testing.T) {
	tests := []struct {
		name  string
		md    string
		width int
		want  string
	}{
		{name: "marker without width", md: "---", width: 0, want: "---"},
		{name: "asterisks without width", md: "***", width: 0, want: "***"},
		{name: "underscores without width", md: "___", width: 0, want: "___"},
		{name: "full width", md: "---", width: 5, want: "─────"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RenderStyled(tt.md, tt.width, PlainStyles()); got != tt.want {
				t.Fatalf("RenderStyled() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRenderStyled_RuleStyled(t *testing.T) {
	if got := RenderStyled("***", 4, DefaultDarkStyles()); got != "\x1b[2m────\x1b[0m" {
		t.Fatalf("RenderStyled() = %q", got)
	}
}

func TestRenderStyled_FencedCode(t *testing.T) {
	md := "```go\nfmt.Println(\"hi\")\nfmt.Println(2)\n```"
	want := "fmt.Println(\"hi\")\nfmt.Println(2)"
	if got := RenderStyled(md, 0, PlainStyles()); got != want {
		t.Fatalf("RenderStyled() = %q, want %q", got, want)
	}
}

func TestRenderStyled_FencedCodeTilde(t *testing.T) {
	md := "~~~\ncode line\n~~~"
	if got := RenderStyled(md, 0, PlainStyles()); got != "code line" {
		t.Fatalf("RenderStyled() = %q", got)
	}
}

func TestRenderStyled_FencedCodeUnclosed(t *testing.T) {
	md := "before\n\n```\nnever closed\n\nafter"
	want := "before\n\nnever closed\n\nafter"
	if got := RenderStyled(md, 0, PlainStyles()); got != want {
		t.Fatalf("RenderStyled() = %q, want %q", got, want)
	}
}

func TestRenderStyled_IndentedCode(t *testing.T) {
	md := "text\n\n    code one\n    code two\n\nmore text"
	want := "text\n\ncode one\ncode two\n\nmore text"
	if got := RenderStyled(md, 0, PlainStyles()); got != want {
		t.Fatalf("RenderStyled() = %q, want %q", got, want)
	}
}

func TestRenderStyled_IndentedCodeAfterParagraph(t *testing.T) {
	// Lines render one by one, so a four-space line following a paragraph
	// keeps its own line instead of becoming code.
	md := "paragraph\n    continuation"
	want := "paragraph\ncontinuation"
	if got := RenderStyled(md, 0, PlainStyles()); got != want {
		t.Fatalf("RenderStyled() = %q, want %q", got, want)
	}
}

func TestRenderStyled_CodeStyled(t *testing.T) {
	md := "```\nsecret\n```"
	want := "\x1b[2msecret\x1b[0m"
	if got := RenderStyled(md, 0, DefaultDarkStyles()); got != want {
		t.Fatalf("RenderStyled() = %q, want %q", got, want)
	}
}

func TestRenderStyled_BlankSeparationPreserved(t *testing.T) {
	tests := []struct {
		name string
		md   string
		want string
	}{
		{name: "single blank", md: "a\n\nb", want: "a\n\nb"},
		{name: "blank run collapses", md: "a\n\n\n\nb", want: "a\n\nb"},
		{name: "no blank between", md: "a\nb", want: "a\nb"},
		{name: "leading blanks dropped", md: "\n\na", want: "a"},
		{name: "trailing blanks dropped", md: "a\n\n\n", want: "a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RenderStyled(tt.md, 0, PlainStyles()); got != tt.want {
				t.Fatalf("RenderStyled() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRenderStyled_UnknownMarkdownUntouched(t *testing.T) {
	md := "| a | b |\n|---|---|\n| 1 | 2 |"
	if got := RenderStyled(md, 0, PlainStyles()); got != md {
		t.Fatalf("RenderStyled() = %q, want %q", got, md)
	}
}

func TestRenderStyled_EntryHeading(t *testing.T) {
	// A journal day file rendered with entry headings and prose.
	md := "## 08:15:00\n\nMorning pages with **focus**.\n\n## 09:00:00\n\n- plan the day\n- write tests"
	want := "08:15:00\n\nMorning pages with focus.\n\n09:00:00\n\n• plan the day\n• write tests"
	if got := RenderStyled(md, 0, PlainStyles()); got != want {
		t.Fatalf("RenderStyled() = %q, want %q", got, want)
	}
}

func TestRenderStyled_EmptyInput(t *testing.T) {
	tests := []string{"", "\n", "   \n\t\n"}
	for _, md := range tests {
		if got := RenderStyled(md, 40, DefaultDarkStyles()); got != "" {
			t.Fatalf("RenderStyled(%q) = %q, want empty", md, got)
		}
	}
}

func TestRenderStyled_CarriageReturns(t *testing.T) {
	md := "## 10:00:00\r\n\r\nbody line\r\n"
	want := "10:00:00\n\nbody line"
	if got := RenderStyled(md, 0, PlainStyles()); got != want {
		t.Fatalf("RenderStyled() = %q, want %q", got, want)
	}
}

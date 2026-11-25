package jot

import (
	"reflect"
	"testing"
)

func TestParseEntries(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []ParsedEntry
	}{
		{
			name:    "empty",
			content: "",
			want:    nil,
		},
		{
			name:    "whitespace only",
			content: "  \n\t\n",
			want:    nil,
		},
		{
			name:    "preamble only",
			content: "a note before any entry",
			want:    []ParsedEntry{{Body: "a note before any entry"}},
		},
		{
			name:    "single entry",
			content: "## 10:00:00\n\nWrote the parser.\n",
			want:    []ParsedEntry{{Time: "10:00:00", Body: "Wrote the parser.", StartLine: 1}},
		},
		{
			name:    "two entries separated by blanks",
			content: "## 09:00:00\n\nFirst.\n\n## 10:30:00\n\nSecond.",
			want: []ParsedEntry{
				{Time: "09:00:00", Body: "First.", StartLine: 1},
				{Time: "10:30:00", Body: "Second.", StartLine: 5},
			},
		},
		{
			name:    "no blank lines between entries",
			content: "## 09:00:00\nfirst\n## 10:30:00\nsecond",
			want: []ParsedEntry{
				{Time: "09:00:00", Body: "first", StartLine: 1},
				{Time: "10:30:00", Body: "second", StartLine: 3},
			},
		},
		{
			name:    "hand-edited heading text",
			content: "## Morning pages\n\nFree writing.",
			want:    []ParsedEntry{{Time: "Morning pages", Body: "Free writing.", StartLine: 1}},
		},
		{
			name:    "trailing whitespace everywhere",
			content: "## 10:00:00   \nbody  \t\n\n\n## 11:00:00\t\nmore  ",
			want: []ParsedEntry{
				{Time: "10:00:00", Body: "body  \t", StartLine: 1},
				{Time: "11:00:00", Body: "more  ", StartLine: 5},
			},
		},
		{
			name:    "carriage returns",
			content: "## 10:00:00\r\n\r\nline one\r\nline two\r\n",
			want:    []ParsedEntry{{Time: "10:00:00", Body: "line one\nline two", StartLine: 1}},
		},
		{
			name:    "deeper headings stay in body",
			content: "## 10:00:00\nintro\n### sub heading\ndetails",
			want:    []ParsedEntry{{Time: "10:00:00", Body: "intro\n### sub heading\ndetails", StartLine: 1}},
		},
		{
			name:    "preamble then entry",
			content: "scratch\nnotes\n\n## 10:00:00\nentry text",
			want: []ParsedEntry{
				{Body: "scratch\nnotes", StartLine: 0},
				{Time: "10:00:00", Body: "entry text", StartLine: 4},
			},
		},
		{
			name:    "leading blanks are no preamble",
			content: "\n\n\n## 10:00:00\nentry",
			want:    []ParsedEntry{{Time: "10:00:00", Body: "entry", StartLine: 4}},
		},
		{
			name:    "internal blanks preserved",
			content: "## 10:00:00\npara one\n\n\npara two\n\n## 11:00:00\nnext",
			want: []ParsedEntry{
				{Time: "10:00:00", Body: "para one\n\n\npara two", StartLine: 1},
				{Time: "11:00:00", Body: "next", StartLine: 7},
			},
		},
		{
			name:    "empty entry body kept",
			content: "## 10:00:00\n\n\n## 11:00:00\nkept",
			want: []ParsedEntry{
				{Time: "10:00:00", StartLine: 1},
				{Time: "11:00:00", Body: "kept", StartLine: 4},
			},
		},
		{
			name:    "heading without time parses as text",
			content: "## 9:00:00\nnot zero padded",
			want:    []ParsedEntry{{Time: "9:00:00", Body: "not zero padded", StartLine: 1}},
		},
		{
			name:    "bare heading marker",
			content: "##\nbody under bare heading",
			want:    []ParsedEntry{{Body: "body under bare heading", StartLine: 1}},
		},
		{
			name:    "code block with heading-looking line",
			content: "## 10:00:00\n```md\n## example\n```\nend",
			want: []ParsedEntry{
				{Time: "10:00:00", Body: "```md", StartLine: 1},
				{Time: "example", Body: "```\nend", StartLine: 3},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseEntries(tt.content)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ParseEntries() =\n%#v\nwant\n%#v", got, tt.want)
			}
		})
	}
}

// TestParseEntries_FormatEntryRoundTrip checks that what jot writes parses
// back into exactly one entry.
func TestParseEntries_FormatEntryRoundTrip(t *testing.T) {
	content := FormatEntry("12:34:56", "line one\n\nline two")
	got := ParseEntries(content)
	want := []ParsedEntry{{Time: "12:34:56", Body: "line one\n\nline two", StartLine: 1}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseEntries(FormatEntry(...)) =\n%#v\nwant\n%#v", got, want)
	}
}

func TestParseEntries_JournalFileShape(t *testing.T) {
	// Three entries written by Add, then hand-edited: the middle entry
	// heading was retitled and trailing whitespace added.
	content := "## 08:00:00\n\nMorning walk.\n\n## standup notes\n\nTalked to the team.\n  \n## 17:45:00\n\nShipped the release.\n"
	got := ParseEntries(content)
	want := []ParsedEntry{
		{Time: "08:00:00", Body: "Morning walk.", StartLine: 1},
		{Time: "standup notes", Body: "Talked to the team.", StartLine: 5},
		{Time: "17:45:00", Body: "Shipped the release.", StartLine: 9},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseEntries() =\n%#v\nwant\n%#v", got, want)
	}
}

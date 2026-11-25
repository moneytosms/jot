package jot

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestMarkdownHTML(t *testing.T) {
	md := "Hello **bold** *it* `x<y` ~~old~~ #tag <script>\n\n" +
		"- one\n  - nested\n- [ ] todo\n- [x] done\n\n" +
		"1. first\n2. second\n\n> quoted *line*\n\n```\n<raw>\n```\n\n" +
		"| a | b |\n|---|---|\n| 1 | 2 |\n\n[site](https://example.com) [bad](javascript:alert(1)) https://jot.dev."
	got := MarkdownHTML(md)
	for _, want := range []string{
		"<p>Hello <strong>bold</strong> <em>it</em> <code>x&lt;y</code> <del>old</del> <span class=\"tag\">#tag</span> &lt;script&gt;</p>",
		"<ul><li>one<ul><li>nested</li></ul></li><li class=\"task\">todo</li><li class=\"task done\">done</li></ul>",
		"<ol><li>first</li><li>second</li></ol>",
		"<blockquote><p>quoted <em>line</em></p>\n</blockquote>",
		"<pre><code>&lt;raw&gt;</code></pre>",
		"<table><thead><tr><th>a</th><th>b</th></tr></thead><tbody><tr><td>1</td><td>2</td></tr></tbody></table>",
		`<a href="https://example.com">site</a>`,
		`<a href="https://jot.dev">https://jot.dev</a>.`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, `href="javascript`) {
		t.Errorf("unsafe link emitted:\n%s", got)
	}
}

func TestExportDocument(t *testing.T) {
	j := opsTestJot(t)
	addEntry(t, j, "work", "shipped **it** #release")
	dir := t.TempDir()
	path, err := j.ExportDocument("work", ExportOptions{Format: "html", Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	for _, want := range []string{`<section class="cover">`, "<h1>work</h1>", `<nav class="toc">`, "<strong>it</strong>"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("export missing %q", want)
		}
	}
	if _, err := j.ExportDocument("work", ExportOptions{Format: "html", Dir: dir, From: "2999-01-01"}); err == nil {
		t.Error("empty range should fail")
	}
	if os.Getenv("JOT_TEST_PDF") == "" || findBrowser() == "" {
		return
	}
	if _, err := exec.LookPath(findBrowser()); err != nil {
		return
	}
	pdf, err := j.ExportDocument("work", ExportOptions{Format: "pdf", Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if head, _ := os.ReadFile(pdf); !strings.HasPrefix(string(head), "%PDF") {
		t.Fatal("not a PDF")
	}
}

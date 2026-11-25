package jot

import (
	"context"
	"errors"
	"fmt"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// ExportOptions selects what ExportDocument writes.
type ExportOptions struct {
	// Format is "html" or "pdf".
	Format string
	// From and To limit the days, inclusive YYYY-MM-DD; empty means open.
	From, To string
	// Dir is the output directory; empty means ./jot-export.
	Dir string
	// Entries keeps only the entries whose time starts with one of these
	// ("09:41" or "09:41:07"). It needs a single day (From == To).
	Entries []string
}

// ErrNothingToExport means the journal has no entries in the chosen days.
var ErrNothingToExport = errors.New("no entries to export")

// ExportDocument writes one journal as a styled, self-contained document: a
// cover page with the date range and counts, a month-by-month contents list,
// then every day with its entries rendered from markdown. A single day
// (From == To) skips the cover and contents: just the date and its entries,
// optionally only some of them (Entries). PDF output prints
// that HTML with a headless Chromium-family browser. It returns the path
// written. Encrypted journals are exported decrypted.
func (j *Jot) ExportDocument(name string, opt ExportOptions) (string, error) {
	name = j.journalName(name)
	files, err := j.ShowDays(name)
	if err != nil {
		return "", err
	}
	var days []DayFile
	for _, f := range files {
		if (opt.From != "" && f.Day < opt.From) || (opt.To != "" && f.Day > opt.To) || strings.TrimSpace(f.Content) == "" {
			continue
		}
		days = append(days, f)
	}
	single := opt.From != "" && opt.From == opt.To
	if len(opt.Entries) > 0 && !single {
		return "", errors.New("picking entries needs a single day")
	}
	if len(days) == 0 {
		return "", fmt.Errorf("%w: journal %s has nothing in that range", ErrNothingToExport, name)
	}
	base, page := name, ""
	switch {
	case single:
		entries := pickEntries(ParseEntries(days[0].Content), opt.Entries)
		if len(entries) == 0 {
			return "", fmt.Errorf("%w: no entry at %s on %s in %s", ErrNothingToExport, strings.Join(opt.Entries, ", "), opt.From, name)
		}
		base += "-" + opt.From
		if len(opt.Entries) > 0 {
			base += "-" + strings.NewReplacer(":", "", ",", "-").Replace(strings.Join(opt.Entries, ","))
		}
		page = dayHTML(name, opt.From, entries)
	default:
		if opt.From != "" || opt.To != "" {
			base += "-" + or(opt.From, "start") + "_" + or(opt.To, "end")
		}
		page = journalHTML(name, days, time.Now())
	}
	dir := opt.Dir
	if strings.TrimSpace(dir) == "" {
		dir = "jot-export"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	htmlPath := filepath.Join(dir, base+".html")
	switch opt.Format {
	case "", "html":
		return htmlPath, os.WriteFile(htmlPath, []byte(page), 0o644)
	case "pdf":
	default:
		return "", fmt.Errorf("unknown export format %q; use html or pdf", opt.Format)
	}
	browser := findBrowser()
	if browser == "" {
		return "", errors.New("PDF export needs Chrome, Chromium, Edge or Brave; export --format html and print it from any browser instead")
	}
	tmp, err := os.MkdirTemp("", "jot-pdf-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	src := filepath.Join(tmp, base+".html")
	if err := os.WriteFile(src, []byte(page), 0o600); err != nil {
		return "", err
	}
	pdfPath, err := filepath.Abs(filepath.Join(dir, base+".pdf"))
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, browser, "--headless", "--disable-gpu", "--no-sandbox",
		"--no-pdf-header-footer", "--print-to-pdf-no-header", "--user-data-dir="+filepath.Join(tmp, "profile"),
		"--print-to-pdf="+pdfPath, "file://"+filepath.ToSlash(src))
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("printing PDF with %s: %v: %s", filepath.Base(browser), err, strings.TrimSpace(string(out)))
	}
	if st, err := os.Stat(pdfPath); err != nil || st.Size() == 0 {
		return "", fmt.Errorf("%s did not write %s", filepath.Base(browser), pdfPath)
	}
	return pdfPath, nil
}

// findBrowser returns a Chromium-family browser able to print to PDF.
func findBrowser() string {
	for _, n := range []string{"chromium", "chromium-browser", "google-chrome-stable", "google-chrome", "chrome", "microsoft-edge", "msedge", "brave", "brave-browser"} {
		if p, err := exec.LookPath(n); err == nil {
			return p
		}
	}
	var paths []string
	switch runtime.GOOS {
	case "darwin":
		paths = []string{"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome", "/Applications/Chromium.app/Contents/MacOS/Chromium",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge", "/Applications/Brave Browser.app/Contents/MacOS/Brave Browser"}
	case "windows":
		for _, base := range []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)"), os.Getenv("LocalAppData")} {
			if base != "" {
				paths = append(paths, filepath.Join(base, `Google\Chrome\Application\chrome.exe`), filepath.Join(base, `Microsoft\Edge\Application\msedge.exe`))
			}
		}
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// journalHTML builds the whole export document.
func journalHTML(name string, days []DayFile, now time.Time) string {
	type month struct {
		key, label string
		days       []DayFile
	}
	var months []*month
	entries, words := 0, 0
	tags := map[string]bool{}
	for _, d := range days {
		t, _ := time.Parse("2006-01-02", d.Day)
		key := d.Day[:7]
		if len(months) == 0 || months[len(months)-1].key != key {
			months = append(months, &month{key: key, label: t.Format("January 2006")})
		}
		months[len(months)-1].days = append(months[len(months)-1].days, d)
		for _, e := range ParseEntries(d.Content) {
			entries++
			words += len(strings.Fields(e.Body))
			for _, tg := range Tags(e.Body) {
				tags[tg] = true
			}
		}
	}
	first, _ := time.Parse("2006-01-02", days[0].Day)
	last, _ := time.Parse("2006-01-02", days[len(days)-1].Day)
	rangeText := first.Format("2 January 2006")
	if !last.Equal(first) {
		rangeText += " – " + last.Format("2 January 2006")
	}

	var b strings.Builder
	esc := html.EscapeString
	fmt.Fprintf(&b, "<!doctype html>\n<html lang=\"en\"><head><meta charset=\"utf-8\">\n<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n<title>%s · jot</title>\n<style>%s</style></head><body>\n", esc(name), exportCSS)
	fmt.Fprintf(&b, `<section class="cover"><p class="eyebrow">jot · journal</p><h1>%s</h1><p class="range">%s</p>
<dl class="stats"><div><dt>days</dt><dd>%d</dd></div><div><dt>entries</dt><dd>%d</dd></div><div><dt>words</dt><dd>%d</dd></div><div><dt>tags</dt><dd>%d</dd></div></dl>
<p class="generated">Exported %s</p></section>
`, esc(name), esc(rangeText), len(days), entries, words, len(tags), now.Format("2 January 2006"))
	b.WriteString(`<nav class="toc"><h2>Contents</h2><ol>`)
	for _, m := range months {
		fmt.Fprintf(&b, `<li><a href="#m-%s"><span>%s</span><span class="dots"></span><span class="n">%d %s</span></a></li>`,
			m.key, esc(m.label), len(m.days), plural(len(m.days), "day", "days"))
	}
	b.WriteString("</ol></nav>\n<main>\n")
	for _, m := range months {
		fmt.Fprintf(&b, "<section class=\"month\" id=\"m-%s\"><h2>%s</h2>\n", m.key, esc(m.label))
		for _, d := range m.days {
			t, _ := time.Parse("2006-01-02", d.Day)
			fmt.Fprintf(&b, `<article class="day" id="d-%s"><header><h3>%s</h3><time datetime="%s">%s</time></header>`+"\n",
				d.Day, t.Format("Monday"), d.Day, t.Format("2 January 2006"))
			for _, e := range ParseEntries(d.Content) {
				b.WriteString(entryHTML(e))
			}
			b.WriteString("</article>\n")
		}
		b.WriteString("</section>\n")
	}
	b.WriteString("</main></body></html>\n")
	return b.String()
}

// entryHTML renders one entry with its HH:MM time.
func entryHTML(e ParsedEntry) string {
	at := e.Time
	if tm, err := time.Parse("15:04:05", e.Time); err == nil {
		at = tm.Format("15:04")
	}
	return fmt.Sprintf("<div class=\"entry\"><span class=\"at\">%s</span><div class=\"body\">%s</div></div>\n", html.EscapeString(at), MarkdownHTML(e.Body))
}

// pickEntries keeps the entries whose time starts with one of want; an
// empty want keeps them all.
func pickEntries(all []ParsedEntry, want []string) []ParsedEntry {
	if len(want) == 0 {
		return all
	}
	var out []ParsedEntry
	for _, e := range all {
		for _, w := range want {
			if w != "" && strings.HasPrefix(e.Time, w) {
				out = append(out, e)
				break
			}
		}
	}
	return out
}

// dayHTML is the export for one day: no cover, just the date and entries.
func dayHTML(name, day string, entries []ParsedEntry) string {
	t, _ := time.Parse("2006-01-02", day)
	esc := html.EscapeString
	var b strings.Builder
	fmt.Fprintf(&b, "<!doctype html>\n<html lang=\"en\"><head><meta charset=\"utf-8\">\n<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n<title>%s · %s · jot</title>\n<style>%s</style></head><body>\n", esc(name), day, exportCSS)
	fmt.Fprintf(&b, `<article class="day single"><header><div><p class="eyebrow">jot · %s</p><h1>%s</h1></div><time datetime="%s">%s</time></header>`+"\n",
		esc(name), t.Format("Monday, 2 January 2006"), day, day)
	for _, e := range entries {
		b.WriteString(entryHTML(e))
	}
	b.WriteString("</article></body></html>\n")
	return b.String()
}

// or returns s, or def when s is empty.
func or(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// plural picks the singular or plural word for n.
func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// exportCSS styles the document on screen and in print. The palette is the
// jot site's paper, ink, forest and pen colors.
const exportCSS = `
:root{--paper:#faf7ee;--ink:#1b1a15;--ink2:#3d3a30;--ink3:#6b6656;--rule:#cec5b1;--forest:#2f5f38;--tint:#dfe8d6;--pen:#c8352a;
--serif:"Newsreader","Iowan Old Style","Charter","Georgia",serif;--mono:"IBM Plex Mono",ui-monospace,"SFMono-Regular",Menlo,monospace}
*{box-sizing:border-box}
html{background:var(--paper);color:var(--ink);font:11.5pt/1.6 var(--serif);-webkit-print-color-adjust:exact;print-color-adjust:exact}
body{margin:0 auto;max-width:44rem;padding:3rem 1.5rem}
a{color:var(--forest)}
.cover{min-height:92vh;display:flex;flex-direction:column;justify-content:center;border-left:3px solid var(--pen);padding-left:2rem;break-after:page}
.eyebrow,.generated,dt,.at,.toc .n,.day time{font-family:var(--mono);font-size:.72rem;letter-spacing:.08em;text-transform:uppercase;color:var(--ink3)}
.cover h1{font-size:3.6rem;line-height:1.05;margin:.4rem 0 .6rem;font-weight:600;letter-spacing:-.01em}
.range{font-size:1.15rem;color:var(--ink2);margin:0 0 2.5rem}
.stats{display:flex;gap:2.5rem;margin:0 0 3rem}
.stats dd{margin:0;font-size:1.7rem;font-variant-numeric:tabular-nums}
.toc{break-after:page}
.toc h2,.month>h2{font-size:1.6rem;font-weight:600;margin:0 0 1.2rem;padding-bottom:.4rem;border-bottom:1px solid var(--rule)}
.toc ol{list-style:none;margin:0;padding:0}
.toc a{display:flex;gap:.5rem;align-items:baseline;color:var(--ink);text-decoration:none;padding:.25rem 0}
.toc .dots{flex:1;border-bottom:1px dotted var(--rule)}
.month{break-before:page}
.month:first-of-type{break-before:auto}
.day{margin:0 0 2rem}
.day header{display:flex;justify-content:space-between;align-items:baseline;border-bottom:1px solid var(--rule);margin-bottom:.8rem;break-after:avoid}
.day h3{font-size:1.15rem;margin:0;font-weight:600}
.single h1{font-size:2rem;line-height:1.1;margin:.2rem 0 0;font-weight:600}.single header{align-items:flex-end;padding-bottom:.6rem;margin-bottom:1.2rem}
.entry{display:grid;grid-template-columns:3.6rem 1fr;gap:.6rem;margin:0 0 .9rem;break-inside:avoid}
.at{padding-top:.28rem;color:var(--forest)}
.body>:first-child{margin-top:0}.body>:last-child{margin-bottom:0}
.body p{margin:0 0 .6rem}
.body h4,.body h5{font-size:1rem;margin:.9rem 0 .4rem}
.body ul,.body ol{margin:0 0 .6rem;padding-left:1.3rem}
.body li.task{list-style:none;margin-left:-1.3rem}
.body li.task::before{content:"";display:inline-block;width:.72em;height:.72em;margin:0 .58rem -.05em 0;border:1.3px solid var(--forest);border-radius:2px}
.body li.done::before{background:var(--forest);box-shadow:inset 0 0 0 1.5px var(--paper)}
.body li.done{color:var(--ink3);text-decoration:line-through}
.body blockquote{margin:0 0 .6rem;padding:.1rem 0 .1rem 1rem;border-left:2px solid var(--rule);color:var(--ink2);font-style:italic}
.body code{font:.85em var(--mono);background:var(--tint);padding:.05em .3em;border-radius:3px}
.body pre{font:.8rem/1.5 var(--mono);background:#f0ebdd;padding:.7rem .9rem;border-radius:4px;overflow-x:auto;white-space:pre-wrap;break-inside:avoid}
.body pre code{background:none;padding:0}
.body hr{border:0;border-top:1px solid var(--rule);margin:1rem 0}
.body table{border-collapse:collapse;margin:0 0 .6rem;font-size:.92em}
.body th,.body td{border-bottom:1px solid var(--rule);padding:.25rem .7rem .25rem 0;text-align:left}
.tag{font:.78em var(--mono);color:var(--pen)}
@page{size:A4;margin:20mm 18mm 22mm;@bottom-center{content:counter(page);font:9pt "IBM Plex Mono",monospace;color:#6b6656}}
@page:first{@bottom-center{content:none}}
@media print{:root{--paper:#fff}html{background:#fff}body{max-width:none;padding:0}.cover{min-height:250mm}a{text-decoration:none}}
`

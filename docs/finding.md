# Finding and reviewing

Read a day or a journal, list what exists, search by text, tag or date, and hand the files to other tools.

## Read a day

```bash
jot show                   # today
jot show yesterday
jot show 2026-10-01
jot show -j work -d all    # every day, same as -a
```

A single day starts with a `-- <journal> . <day> (<weekday>) . <N> entries`
bar, using `entry` when there is one. Add `--render` to render the markdown
for the terminal: headings, emphasis, inline code, nested lists, quotes and
rules are styled, and text wraps at 80 columns. ANSI styling is used only on a
terminal; `NO_COLOR` turns it off.

## Read a whole journal

```bash
jot show -j work -a
```

`-a` prints every day, oldest first, under a `# <journal> · <day>` heading.

## See what exists

```bash
jot ls                     # journals, with entry and day counts
jot ls work                # days in one journal, with sizes
```

## Find a day without dates

Start with the month grid when you remember roughly when something happened
but not the date:

```bash
jot show -j work --calendar 2026-11
```

```
November 2026
Mo Tu We Th Fr Sa Su
                                1
  2    3    4.   5*   6    7    8
  9   10   11   12   13   14   15
 16   17   18   19   20.  21   22
 23   24  [25]  26   27   28   29
 30

legend: . 1-2 entries, * 3+ entries, [..] today
7 entries in November 2026
```

`.` marks a day with 1-2 entries and `*` a day with 3 or more. Today is
bracketed when you ask for the current month. Pick a marked day and show it:

```bash
jot show -j work -d 2026-11-20
```

In the TUI the calendar stays visible: `h`/`l` move a day, `j`/`k` a week and
`H`/`L` a month.

## Search

```bash
jot search parser
jot search -j work standup
jot search "next steps"
```

Plain search is a case-insensitive substring match, one line at a time. Results
are grouped by journal and day, and each hit keeps a
`journal/day.md:line:` prefix that most terminals and editors can open:

```
work . 2026-11-05 . 2 hits
  work/2026-11-05.md:3: shipped the parser rewrite
  work/2026-11-05.md:7: parser tests are green

2 matches in 1 journal
```

Journals are alphabetical and days newest first. If nothing matches, the
output is `no matches for "..."`.

## Find an old note

When you remember the gist but not the exact words, use `--fuzzy`:

```bash
jot search --fuzzy --limit 10 "parser empty input"
```

Fuzzy search matches query characters in order across the whole entry, ranks
the entries, and prints `journal . day . time` with a short preview. `--limit`
defaults to 20; `0` removes the cap. It searches every journal unless `-j` is
given.

In the TUI press `/` for the same entry-level search; Enter jumps to the day.
Plain `jot search` only finds contiguous substrings, so try fuzzy first when a
phrase is half-remembered.

## Use other tools

Journal files are plain markdown in the root directory, so ordinary tools
work on them:

```bash
jot show -j work | glow -
rg -n --glob '*.md' "TODO" ~/.jot/work
```

If the root is not `~/.jot`, substitute the path shown on the `root` line of
`jot doctor`.

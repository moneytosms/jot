# Command reference

Every command, generally in the order `jot help` lists them. Each section
gives the synopsis, what the command does, its flags, and at least one example
(with output where it helps).

```
jot <command> [flags] [arguments]
```

## Global forms

| Invocation | Result |
| --- | --- |
| `jot` in a terminal | Opens the TUI |
| `jot` with no terminal (a pipe or redirect) | Prints the overview help |
| `jot help`, `jot -h`, `jot --help` | Overview help |
| `jot help <command>` | Help page for one command |
| `jot <command> -h` or `--help` | Help page for that command; not available for `version` or `help` |
| `jot version`, `jot -v`, `jot --version` | Prints `jot <version>` |

Exit status is `0` on success, `1` for ordinary errors (unknown command, invalid
date, missing journal, failed check), and `2` for a `tui` usage error after its
help page has printed. `jot doctor` exits `1` when a check fails. `install.sh`
exits `2` for an unknown option and `1` when Go is missing; a failed build exits
with the status of `go build`.

Flags accept one or two dashes: `-journal` and `--journal` are the same flag.
Options must come before positional arguments. The flag parser stops at the first
argument that is not a flag, so `jot search parser --fuzzy` searches for
`parser --fuzzy`; write `jot search --fuzzy parser` instead.

Shared flags, listed by each command that takes them:

| Flag | Meaning |
| --- | --- |
| `-j`, `--journal <name>` | Journal to act on. Default: the configured default journal (`default` unless changed). |
| `-d`, `--date <day>` | `today` (default), `yesterday`, `tomorrow`, or `YYYY-MM-DD`. `show` also accepts `all`. |

Journal names are sanitised: surrounding whitespace is trimmed, ASCII letters,
digits, `-` and `_` are kept, and every other character becomes `-`. Repeated
dashes collapse, leading and trailing dashes are removed, and an empty name
becomes `default`. `backups` is a reserved name and is rejected as a journal name
with an error.

---

## tui

```
jot tui [flags]
```

Opens the full-screen interface: a month calendar, the selected day's entries, a
journal sidebar and a one-line quick-add box. Running bare `jot` in a terminal
opens the same interface; with no terminal, bare `jot` prints the overview help.

Press `?` inside for the keybinding cheatsheet. The interface is read-mostly:
quick add with `a` appends one entry, editing always happens in your editor
with `e`, and jot never rewrites entries itself. `R` renames the open journal
and `D` deletes it after archiving it to `<root>/backups` and asking you to
type its name; the default journal is protected. The full keybinding reference
is in [tui.md](tui.md).

| Flag | Default | Meaning |
| --- | --- | --- |
| `-j`, `--journal <name>` | `tui.journal` from the config file when that journal still exists, else the configured default | Journal to open |
| `--theme <name>` | `JOT_THEME`, then `tui.theme` in the config file, then `auto` | Colour theme |

Theme names: `auto`, `default`, `light`, `dark`, `gruvbox`, `nord`,
`dracula`, `catppuccin`, `tokyo-night` and `omarchy`. `dark` is the fixed
dark palette; `default` follows the terminal background. `auto` follows the Omarchy theme
when one is available, otherwise the terminal background; `NO_COLOR` makes
`auto` use a plain palette. `--theme` wins over `JOT_THEME` and the saved
`tui.theme`; `JOT_THEME` wins over `tui.theme`.

The TUI refuses a non-terminal stdin or stdout with
`jot tui needs an interactive terminal; run jot help for the command line`.
An unexpected argument prints the `tui` help page and exits `2`.

```bash
jot tui -j work --theme nord
```

---

## add

```
jot add [flags] [text...]
```

Appends one entry under a `## HH:MM:SS` heading to a day file. The body comes
from the arguments (joined with spaces), or from stdin when the only argument
is `-` or when no arguments are given and stdin is piped. With no arguments on
a terminal, `add` fails and shows examples.

The timestamp is the current local time, including when `-d` selects another
day. A blank body fails with `entry body is empty` and writes nothing. At a
terminal, `jot add ""` fails with a `nothing to add` message instead. Entries
are appended, never rewritten. Missing directories are created.

| Flag | Default | Meaning |
| --- | --- | --- |
| `-j`, `--journal <name>` | configured default | Journal to append to |
| `-d`, `--date <day>` | `today` | Day file to append to |

```bash
jot add -j work "standup: parser is done"
```

```
added to work/2026-11-05.md
```

---

## show

```
jot show [flags] [day]
```

Prints one day of a journal, or every day with `-a`. The day can be given with
`-d` or as the first argument, so `jot show yesterday` works.

A single day starts with a header bar naming the journal, the date, the
weekday and the entry count, padded with dashes to 80 columns:

```
-- work . 2026-11-05 (Thu) . 2 entries -----------------------------------------
```

The bar is dimmed when stdout is a terminal and colour is enabled. Below it the
day prints as stored: raw markdown and blank lines.

With `-a` (or `-d all`), every day prints oldest first under a
`# <journal> · <day>` heading; the single-day bar is not used in this mode.

`--render` passes the day body through the terminal markdown renderer:

- ATX headings `#` to `######` lose their markers; levels 1-3 have their own
  styles, and levels 4-6 share the deepest one.
- Inline `**bold**`, `*italic*`, `***bold italic***`, `` `code` `` and
  `~~strikethrough~~` lose their markers and are styled.
- Unordered lists use `•`, `◦` and `▪` by depth; ordered lists keep their
  numbers.
- Blockquote lines start with `│ `.
- Horizontal rules (`---`, `***`, `___`) become a full-width `─` line.
- Fenced and indented code blocks print dim without inline styling; indented
  blocks lose their leading indent, and lines starting with `|` pass through.
- Paragraphs and list text wrap at 80 columns; headings and code lines never
  wrap.

With `--render` and `-a`, each `# <journal> · <day>` heading is printed as-is;
only the body is rendered.

ANSI styles are emitted only when stdout is a terminal and `NO_COLOR` is unset;
otherwise the same structure prints without escapes. `JOT_THEME` selects the
light styles when it contains `light`; any other value uses the dark styles.
The day bar is dim only while styles are active.

`--calendar [YYYY-MM]` prints a month grid instead of entries. A bare
`--calendar` uses the current month; `-j` scopes the grid to one journal.
`--calendar` ignores `-d`, `-a` and `--render`. Each day cell has a density marker: `.` for
1-2 entries, `*` for 3 or more, nothing for 0. Today is bracketed, the legend
explains the markers, and the last line totals the month:

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

Today is bracketed only when the requested month is the current month. An
invalid month fails with `invalid month "<value>": use YYYY-MM`; a journal
that does not exist uses the usual missing-journal error.

| Flag | Default | Meaning |
| --- | --- | --- |
| `-j`, `--journal <name>` | configured default | Journal to read |
| `-d`, `--date <day>` | `today` | Day to print, or `all` |
| `-a`, `--all` | off | Print every day, with headings |
| `--render` | off | Render markdown for the terminal (80 columns) |
| `--calendar [YYYY-MM]` | current month | Print a month grid; omit the value for this month |

Messages:

| Situation | Output |
| --- | --- |
| Day has no entries | `no entries in <journal> for <day>` (exit 0) |
| `-a` and the journal has no day files | `no entries in <journal> yet` (exit 0) |
| `--calendar` month has no entries | The grid still prints, ending `0 entries in <month> <year>` (exit 0) |
| Journal does not exist | `no journal named "<journal>" yet; ...` (exit 1) |
| Invalid day | `invalid date "<value>": ...` (exit 1) |
| Invalid month | `invalid month "<value>": use YYYY-MM` (exit 1) |

```bash
jot show -j work -d 2026-11-05
```

```
-- work . 2026-11-05 (Thu) . 2 entries -----------------------------------------

## 08:55:40

shipped the parser rewrite

## 11:02:13

next: docs, then the release
```

---

## edit

```
jot edit [flags] [day]
```

Opens one day file in an editor. The file is created empty if it does not
exist. Editor resolution, first match wins: `JOT_EDITOR`, the `editor` field in
the config file, `$VISUAL`, `$EDITOR`, then the first of `vim`, `vi` or `nano`
found on `PATH`. If none is found, the command fails with
`no editor found: set JOT_EDITOR or $EDITOR`.

The editor command is split on whitespace, so arguments work (`code --wait`),
but shell quoting does not: a path containing spaces cannot be used directly.

A file created by `edit` stays on disk if you quit without writing. It then
counts as a day with 0 entries in `jot ls`.

| Flag | Default | Meaning |
| --- | --- | --- |
| `-j`, `--journal <name>` | configured default | Journal to edit |
| `-d`, `--date <day>` | `today` | Day to open |

```bash
jot edit -j ideas
```

Opens the editor on `<root>/ideas/<today>.md` and prints nothing itself.

---

## ls

```
jot ls [flags] [journal]
```

With no journal, lists every journal with its entry count, day count and the
time its most recently changed day file was modified. With a journal name,
lists that journal's days with entry count and size. An entry is a line
starting with `## `.

| Flag | Default | Meaning |
| --- | --- | --- |
| `-j`, `--journal <name>` | none | Same as naming the journal as an argument |

With no journals at all, prints
``no journals yet; add an entry with `jot add "..."` ``. A named journal that does
not exist fails with `no journal named "<journal>" yet`.

```bash
jot ls work
```

```
DAY         ENTRIES  SIZE
2026-11-04  4        612 B
2026-11-05  2        301 B
```

---

## search

```
jot search [flags] <query...>
```

Case-insensitive substring search across every journal, or one journal with
`-j`. Words are joined with spaces, so quoting is optional. Blank lines are
skipped; timestamp headings are searched like any other line. This is a
line-based substring search; use `--fuzzy` to match whole entries.

Matches are grouped by journal and day:

```
ideas . 2026-11-03 . 2 hits
  ideas/2026-11-03.md:3: parser bug: empty input
  ideas/2026-11-03.md:7: the parser should accept empty input
work . 2026-11-05 . 1 hit
  work/2026-11-05.md:3: shipped the parser rewrite
work . 2026-11-04 . 1 hit
  work/2026-11-04.md:3: standup: parser is done

4 matches in 2 journals
```

The header is `<journal> . <day> . <N> hit(s)`. Each hit line is indented two
spaces and keeps the `journal/day.md:line:` prefix, so a terminal or editor can
open it directly. Journals are alphabetical and days newest first. A blank line
separates the last group from `<N> matches in <M> journals`.

No matches prints `no matches for "<query>"` and exits `0`. An empty query
fails with `usage: jot search <query>`.

`--fuzzy` searches whole entries instead of lines. It matches query characters
in order anywhere in the entry body or its time, scores each entry, and prints
the best first; ties go to the newest day, then the earlier time:

```
ideas . 2026-11-03 . 14:33:00
parser bug: empty input

ideas . 2026-11-03 . 15:00:00
the parser should accept empty input

4 matches, showing first 2
```

Each result is a `<journal> . <day> . <time>` header, the first three lines of
the entry body (truncated to 80 characters with `...`), then a blank line. An entry
with no `## HH:MM:SS` heading shows `-` for the time. The closing summary is
`<N> matches`, or `<N> matches, showing first <limit>` when the list was cut.
`--limit` defaults to 20 and applies only to `--fuzzy`; `0` means no limit.

Without `-j`, both modes search every journal. With `-j`, only that journal is
searched. Flags must come before the query; anything after the first query word
is part of the query.

| Flag | Default | Meaning |
| --- | --- | --- |
| `-j`, `--journal <name>` | all journals | Limit the search to one journal |
| `--fuzzy` | off | Entry-level fuzzy search with previews |
| `--limit <n>` | `20` | Maximum fuzzy results; `0` means unlimited |

```
jot search --fuzzy --limit 2 parser
```

```
ideas . 2026-11-03 . 14:33:00
parser bug: empty input

ideas . 2026-11-03 . 15:00:00
the parser should accept empty input

4 matches, showing first 2
```

---

## todo

```
jot todo [flags]
```

Lists markdown checkbox items found in journal files, without opening an
editor. Lines such as `- [ ] call the dentist` (or `*`, and indented variants)
are collected with the journal, day and entry time that contain them. The
command is strictly read-only: jot never rewrites entries, and toggling happens
in your editor.

Without `-j`, every journal is scanned. Items print newest day first, then by
entry time, then in file order. The columns are the box, journal, day, time and
item text; an item outside any timestamped entry shows `-` for the time.

| Flag | Default | Meaning |
| --- | --- | --- |
| `-j`, `--journal <name>` | all journals | Limit the list to one journal |
| `--open` | off | Show unchecked items only |
| `--done` | off | Show completed items only |
| `--all` | off | Accepted for compatibility; it has no effect |
| `--limit <n>` | `0` (no limit) | Show at most N items |

`--open` and `--done` filter the list; combining them shows everything. `--all`
is accepted but has no additional effect. The final `N open . M done` counts
every item, not just the printed or limited ones. When the list is empty after filtering, it prints `no todo items`, a
generic edit hint, and those totals.

```
jot todo -j work
```

```
[ ]  work  2026-11-20  14:33:00  review the release notes
[x]  work  2026-11-20  14:33:00  tag v0.2.0
toggle with jot edit -d 2026-11-20 -j work
1 open . 1 done
```

Change a box by editing the day file:

```
jot edit -d 2026-11-20 -j work
```

## heatmap

```
jot heatmap [flags]
```

Prints one year of a journal as a terminal heatmap: two half-year grids,
weeks as rows, Monday-first weekdays as columns, and a month label on each
row where a month begins. Every day with a file is a two-character cell
shaded by its entry count:

| Glyph | Entries |
| --- | --- |
| `··` | 0 |
| `▄▄` | 1-2 |
| `▓▓` | 3-5 |
| `██` | 6+ |

| Flag | Default | Meaning |
| --- | --- | --- |
| `-j`, `--journal <name>` | configured journal | Journal to chart |
| `--year <yyyy>` | current year | Year to chart |

Output is plain glyphs for pipes and `NO_COLOR`; in a terminal the cells are
colored (truecolor), with a lighter ramp when `JOT_THEME` names a light
theme.

```
jot heatmap -j work --year 2025
```

```
2025 · work · 96d 120e
    Mo Tu We Th Fr Sa Su       Mo Tu We Th Fr Sa Su
Jan          ▄▄ ▄▄ ·· ··   Jul       ▄▄ ·· ▄▄ ·· ▄▄
    ·· ·· ·· ▄▄ ·· ·· ··       ·· ·· ▄▄ ▄▄ ▄▄ ·· ▄▄
…
legend: ·· 0 · ▄▄ 1-2 · ▓▓ 3-5 · ██ 6+ entries
```

---

## remind

```
jot remind MESSAGE DURATION
jot remind list
jot remind clear
```

Sets a transient countdown reminder, lists the active reminders, or clears
them. There is no daemon and no state that survives a reboot: the reminder is
handed to the first available provider, and a reboot or logout drops it.

The duration may come first or last. Go duration syntax is accepted (`90s`,
`20m`, `2h`, `1h30m`), and a bare positive number means minutes. The message
is the remaining words, joined with spaces; quote multi-word text.

Providers, first available wins:

- `omarchy`: runs `omarchy reminder <minutes> <message>`. Omarchy is
  minute-granular, so durations under one minute are rejected and partial
  minutes round up (`90s` becomes `omarchy-reminder-2m`). The status line
  still prints the delay you asked for, so `90s` reports `fires in 1m30s`
  even though omarchy fires at two minutes.
- `systemd-run` and `notify-send`: creates a transient user timer named
  `jot-reminder-*` that shows the message with `notify-send` when it fires.
  `90s` stays 90 seconds here.
- Neither: the command fails with
  `reminders need systemd (user) with notify-send, or omarchy` and a hint to
  install one of them.

`jot remind list` prints the remaining time and message for jot's systemd
timers, for example `in 19m59s  stretch`; once the fire time has passed it
prints the clock time instead. `jot remind clear` stops and removes those
timers and prints `cleared N reminders` or `no reminders to clear`. Omarchy
owns its own reminders, so ones created through omarchy do not appear in
`jot remind list` and are not cleared by `jot remind clear`.

```bash
jot remind "stretch" 20m
```

```
reminder set: stretch
  fires in 20m0s via omarchy-reminder-20m
```

With the systemd fallback the provider label is a `jot-reminder-...` unit
name instead. `jot remind list` with no reminders prints `no reminders`.

---

## watch

```
jot watch [flags]
```

Prints today's entries, then follows today's file and prints text as it is
appended. At midnight it switches to the new day's file and prints a new
`--- <journal>/<day>.md ---` header. If the file is truncated or rewritten,
watching restarts from the top of the new content.

Ctrl+C (SIGINT or SIGTERM) stops it and prints `stopped`. Watching is
read-only. The journal directory must exist; today's file may be missing and is
picked up once it is created.

| Flag | Default | Meaning |
| --- | --- | --- |
| `-j`, `--journal <name>` | configured default | Journal to follow |
| `--poll <duration>` | `750ms` | How often to check for new text |

```bash
jot watch -j work
```

```
watching <root>/work/ (Ctrl+C to stop)
--- work/2026-11-05.md ---
## 09:41:07

shipped the parser rewrite
```

---

## backup

```
jot backup [flags] [dir]
```

Writes a gzip-compressed tar archive, `jot-backup-YYYYMMDD-HHMMSS.tar.gz`
(local time, to the second), containing every `.md` file under the root. The
default `backups` directory is skipped. Entries are stored relative to the
root, for example `work/2026-11-05.md`.

The default destination is `<root>/backups`. That directory is never archived
or listed as a journal. The journal name `backups` is reserved and is rejected
with an error, so every journal is archived. A positional `dir` overrides
`--out`; the directory is created if missing. Two backups in the same second
overwrite each other.

Restore with `tar -xzf <archive> -C <root>`.

| Flag | Default | Meaning |
| --- | --- | --- |
| `-o`, `--out <dir>` | `<root>/backups` | Destination directory |

```bash
jot backup ~/Dropbox/jot-backups
```

```
backup written to <dir>/jot-backup-20261105-150405.tar.gz (2.1 KB)
```

---

## export

```
jot export [flags] [dir]
```

Writes one markdown file per journal, `<journal>.md`, into `dir`. The default
is `./jot-export`, relative to the current directory. Each file starts with
`# <journal>`, then a `## YYYY-MM-DD` heading for each day followed by that
day's entries. Days with no content are skipped.

Exported files are created with mode `0644` (subject to your umask). Export
includes every journal; review the output before sharing it. The
directory is created even when there is nothing to export. With no journals,
no files are written and the command prints `nothing to export yet`.

A positional `dir` overrides `--out`.

| Flag | Default | Meaning |
| --- | --- | --- |
| `-o`, `--out <dir>` | `./jot-export` | Destination directory |

```bash
jot export --out ~/Documents/journal-export
```

```
wrote <dir>/default.md
wrote <dir>/work.md
```

---

## journal

```
jot journal <new|rm|rename|ls> [arguments]
```

Manages journals without touching their directories by hand. `new` creates a
journal, `ls` is the same output as `jot ls`, `rename` uses the same name
sanitising rules as `-j`, and `rm` archives a journal before deleting it.

| Command | Meaning |
| --- | --- |
| `jot journal new NAME` | Create `<root>/NAME`; fails if it already exists |
| `jot journal ls` | List journals, same as `jot ls` |
| `jot journal rename OLD NEW` | Rename a journal directory |
| `jot journal rm [--yes] NAME` | Archive, then delete a journal |

`journal rm` writes a gzip archive to
`<root>/backups/jot-<name>-YYYYMMDD-HHMMSS.tar.gz` first. Without `--yes` it
asks you to type the exact journal name; anything else prints
`aborted; <name> is untouched (archive kept at <path>)` and leaves both the
journal and the archive in place. A piped stdin is refused before anything is
archived:

```
refusing to delete "<name>": stdin is not a terminal; re-run with --yes to skip the prompt
```

The default journal cannot be removed or renamed, and `backups` is always
rejected. `--yes` (or `-y`) skips the typed confirmation but still writes the
archive.

Flags for `journal rm`:

| Flag | Default | Meaning |
| --- | --- | --- |
| `-y`, `--yes` | off | Delete without the typed confirmation |

```bash
jot journal new scratch
jot journal rename scratch notes
jot journal rm --yes notes
```

```
created <root>/scratch
renamed scratch to notes
archived notes to <root>/backups/jot-notes-20261105-150405.tar.gz
deleted notes
```

Flags go before the journal name: `jot journal rm --yes notes`, not
`jot journal rm notes --yes` (the latter is a usage error).

---

## init

```
jot init [flags]
```

Creates the root directory and the default journal. It is safe to run
repeatedly and never overwrites or deletes anything. `init` does not write the
config file; use `jot setup` for that.

Nothing requires `init`, because `jot add` creates directories as needed. It is
useful in scripts, or to create a journal before writing to it.

| Flag | Default | Meaning |
| --- | --- | --- |
| `-j`, `--journal <name>` | none | Also create this journal |
| `--root <dir>` | configured root | Initialise this root for this run only |

```bash
jot init -j work
```

```
initialized <root>
default journal: default
```

---

## setup

```
jot setup [flags]
```

First-run wizard. It asks for the journal root, the default journal name and
the editor, writes the config file, creates the root and the default journal,
and optionally copies the binary into a bin directory.

With a terminal on stdin, each prompt shows its default in brackets, and Enter
accepts it. Without a terminal, or with `--yes`, the defaults are used and the
binary is installed unless `--no-install` is given. `--install` and
`--no-install` together is an error.

Existing root, journal and editor settings become the defaults, and the config
file is then rewritten with the answers. It is written atomically with mode
`0600`. The `tui` block is not preserved. If the bin directory is not on
`PATH`, a `Heads up` line after the summary says so.

| Flag | Default | Meaning |
| --- | --- | --- |
| `--root <dir>` | `JOT_ROOT`, the config file, or `~/.jot` | Journal root |
| `--journal <name>` | current default journal, or `default` | Default journal name |
| `--editor <cmd>` | current editor, else the `jot edit` detection, else `vi` | Editor command |
| `--install-dir <dir>` | `JOT_INSTALL_DIR`, or `~/.local/bin` | Where to copy the binary |
| `--install` | off | Copy the binary without asking |
| `--no-install` | off | Do not copy the binary |
| `-y`, `--yes` | off | Accept defaults; no prompts |

```bash
jot setup --root ~/Documents/journal --journal daily --no-install
```

```
config:  <config file>
journals: <root>
default:  daily
```

The prompts that appear before this summary are omitted here. With `--root`
and `--journal` given, `--no-install` leaves at most the editor prompt.

---

## install

```
jot install [flags] [dir]
```

Copies the running binary to `<dir>/jot` with mode `0755`. The default
directory is `JOT_INSTALL_DIR`, or `~/.local/bin`. A positional `dir` overrides
`--dir`.

If the destination is the running binary or already holds identical bytes,
nothing is copied and the command reports `jot is already installed at ...`.
Afterwards it reports whether the directory is on `PATH`. If it is not, it
prints a bash line to add it and notes the zsh and fish config files.

| Flag | Default | Meaning |
| --- | --- | --- |
| `--dir <dir>` | `JOT_INSTALL_DIR`, or `~/.local/bin` | Destination directory |

```bash
jot install /usr/local/bin
```

```
installed jot to <dir>/jot
<dir> is on your PATH; run `jot doctor` to confirm
```

---

## update

```
jot update [flags]
```

Checks the published releases, compares the latest (or a pinned) release with
the running binary, shows what changed, and after confirmation installs the
`jot-<os>-<arch>` asset for this platform. The download is verified against the
release's `SHA256SUMS` and atomically replaces `<dir>/jot`.

The release API is
`https://api.github.com/repos/moneytosms/jot/releases`; set `JOT_RELEASES_API`
to the full base URL, including `/releases`, to use a mirror or a test server.
Versions compare component by component after an optional leading `v`, so
`v1.2` equals `v1.2.0`; pre-releases sort before their final release, and a
development build such as `0.0.0-dev` sorts before every release. The download must be published for this OS and
architecture, and the release must include `SHA256SUMS`; a missing asset or
checksum entry is an error.

`--check` reports the available release and stops without downloading.
`--json` prints one machine-readable object and stops, so it can be combined
with `--check`. `--notes` prints the full release notes instead of the default
first 15 lines or 1200 characters. `--version <tag>` targets that release
instead of the latest; it still installs only when that tag is newer than the
running version.

Without `--yes`, a terminal is asked `Update now? [Y/n]`; Enter or `y` installs
and anything else prints `aborted`. With no terminal it prints
`re-run with --yes to install` and exits `0` without changing anything.

An install prints `downloading ...`, `verifying SHA-256...`, `installed <path>`,
`installed jot <version> at <path>`, a short `<- what changed in <version>:`
recap, `restart jot (or the TUI) to use it`, and then the new binary's version.
An unwritable install directory adds a hint to use `--dir` or sudo.

`--source` keeps the build-from-checkout path: it runs
`go build -trimpath -o <dir>/jot ./cmd/jot` in `--src` (or `JOT_SRC`, or the
current directory and its parents) and runs the new binary's `version`. It
cannot be combined with `--check`, `--json` or `--version`.

| Flag | Default | Meaning |
| --- | --- | --- |
| `--check` | off | Report without installing |
| `--json` | off | Print one JSON status object |
| `--notes` | off | Print the full release notes |
| `--version <tag>` | latest release | Install a specific release |
| `-y`, `--yes` | off | Install without asking |
| `--dir <dir>` | `JOT_INSTALL_DIR`, or `~/.local/bin` | Install directory |
| `--source` | off | Rebuild from a local source checkout |
| `--src <dir>` | `JOT_SRC`, then the current directory and its parents | Source checkout, with `--source` |

```bash
jot update --check
```

```
update available: v0.1.0 -> v0.2.0 (released 2026-11-01)
What's new:
  - fixed the parser
  - added fuzzy search
  - new calendar in show
  - reminders with durations
```

When there is nothing newer, the same command prints
`<-- up to date: jot v0.2.0 (released 2026-11-01)`.

```bash
jot update --check --json
```

```
{"current":"v0.1.0","latest":"v0.2.0","update_available":true,"released":"2026-11-01","url":"https://github.com/moneytosms/jot/releases/download/v0.2.0/jot-linux-amd64"}
```

---

## uninstall

```
jot uninstall [flags]
```

Removes one or more of three components: journal data (the root, including
`backups`), the config file, and the running binary. With no component flag and
a terminal, it prints an inventory and asks what to remove:

```
data:   /home/you/.jot (2 journals, 43 days, 812.4 KB)
binary: /home/you/.local/bin/jot
config: /home/you/.config/jot/config.json
remove [d]ata, [b]inary, [c]onfig, [a]ll, or [q]uit?
```

`d`/`data`, `b`/`binary`, `c`/`config` and `a`/`all` select one or all; empty
input, `q` or `quit` prints `nothing removed`; anything else fails with
`unknown choice "<value>"; nothing removed`. With no component flag and no
terminal, the command refuses instead of guessing:

```
refusing to remove anything without an explicit component; pass --data, --binary, --config or --all (add --yes for scripts)
  jot uninstall --data --yes
  jot uninstall --binary
  jot uninstall --all --yes
```

`jot uninstall --yes` with no component remains data only, for compatibility
with older versions. Use `--all --yes` to remove data, config and binary.

Without `--yes`, removing data requires a terminal and typing `delete`; a
wrong answer prints `aborted; nothing was removed`. Piped stdin is refused with
`refusing to delete journal data without confirmation; re-run with --yes`.
Config and binary prompts are plain `[y/N]` questions, and a non-terminal is
refused with `refusing to remove files without confirmation; re-run with --yes`.
`--yes` skips every prompt. `--dry-run` prints the plan for the selected
components and removes nothing. With no component and no terminal it still
refuses; with no component and `--yes` it plans data only, matching the
compatibility rule above.

Removal runs config first, then data, then the running binary, so the process
can finish after its executable is unlinked. Each component prints
`<- removed <path>`; a missing path prints `<- skipped <path> (not found)`, or
`<- skipped <label> (no path)` when the path cannot be determined, and is not
an error. A failure on one component does not stop the others, and the
summary reports what was removed, left or skipped:

```
jot uninstall --all --yes
```

```
<- removed /home/you/.config/jot/config.json
<- removed /home/you/.jot
<- removed /home/you/.local/bin/jot
summary: deleted config (/home/you/.config/jot/config.json), data (/home/you/.jot), binary (/home/you/.local/bin/jot)
```

The summary verb is `deleted` when data was removed, otherwise `removed`;
`summary: removed nothing` means nothing was removed, whether because it did
not exist, was skipped, or could not be deleted.
When the binary still exists afterwards, it adds
`to remove the binary: rm "$(command -v jot)"`.

Data is removed with `RemoveAll` and is not shredded. Data removal refuses `/`,
`.` and the home directory itself. Back up first if the entries matter;
`jot backup ~/somewhere-else` keeps copies outside the root.

| Flag | Default | Meaning |
| --- | --- | --- |
| `--data` | off | Remove the journal root, including backups |
| `--binary` | off | Remove the running jot executable |
| `--config` | off | Remove the config file |
| `--all` | off | Remove data, config and binary |
| `--dry-run` | off | Print the plan; remove nothing |
| `-y`, `--yes` | off | Skip all confirmations |

---

## doctor

```
jot doctor
```

Checks the installation and prints one line per check, then exits. `[warn]`
means the check works but needs attention. `[fail]` means something is broken;
the command then exits `1` with `N check(s) failed`. A config file that is not
valid JSON stops `jot doctor` before any check runs, as it does every other
command that reads config; the error is printed as `jot: config file <path>: ...`.
`jot version` and `jot help` still run.

| Check | ok | warn | fail |
| --- | --- | --- | --- |
| `root` | Directory exists | Does not exist yet (created on first entry) | Exists but is not a directory, or cannot be read |
| `writable` | A probe file can be written to the root | (none) | Root is not writable |
| `config` | File parses, or none exists yet | No config directory available | File cannot be read (malformed JSON stops the command earlier) |
| `journals` | Journal and entry counts | (none) | Root cannot be listed |
| `default journal` | Directory exists | Does not exist yet (`jot add` creates it) | (none) |
| `editor` | Configured or detected editor is found | No editor found | Configured editor is not executable |
| `binary` | `jot` resolves on `PATH` | `jot` is not on `PATH` | (none) |
| `reminders` | A provider is available (`omarchy`, or `systemd-run` with `notify-send`) | No provider available | (none) |
| `omarchy` | Found on `PATH` | Not found (optional) | (none) |
| `notify-send` | Found on `PATH` | Not found (optional) | (none) |
| `systemd-run` | Found on `PATH` | Not found (optional) | (none) |
| `vendor` | `vendor/` directory present | No `vendor/` directory | (none) |

```bash
jot doctor
```

```
jot 0.0.0-dev

[ok]    root             <root>
[ok]    writable         root is writable
[ok]    config           none yet; using defaults
[ok]    journals         2 journals, 215 entries
[ok]    default journal  default
[ok]    editor           nvim
[warn]  binary           `jot` is not on your PATH
[ok]    reminders        available via omarchy
[ok]    omarchy          found on PATH
[ok]    notify-send      found on PATH
[ok]    systemd-run      found on PATH
[warn]  vendor           no vendor/ directory
```

---

## version

```
jot version
```

Prints `jot <version>`. The aliases are `jot -v` and `jot --version`. A plain
`go build` reports `0.0.0-dev`. Builds set the version with
`-ldflags "-X main.version=<version>"`.

```bash
jot version
```

```
jot 0.0.0-dev
```

---

## help

```
jot help [command]
```

Prints the overview help, or the help page for one command. `jot <command> -h`
prints the same page for commands that have one; `version` and `help` do not.
An unknown command name fails with
``no help for "<name>"; run `jot help` ``.

```bash
jot help backup
```

Prints the help page for `backup`, the same output as `jot backup -h`.

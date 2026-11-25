# Guide

Task-based usage: daily logging, reviewing, editors, backups, sync and
configuration. Flag-level detail is in [commands.md](commands.md).

## Quickstart

From the repository root, with Go installed. The commands below assume
`~/.local/bin` is on `PATH`:

```bash
./install.sh                    # builds jot into ~/.local/bin
jot setup --yes                 # defaults: root ~/.jot, journal "default"
jot add "first entry"
jot add -j work "standup: parser is done"
jot show
jot search parser
jot doctor
```

If `jot` is not found afterwards, `install.sh` prints the line that adds
`~/.local/bin` to `PATH`. Add it to your shell config and open a new shell.

Run `jot` with no command in a terminal to open the TUI; see
[The TUI](#the-tui).

## Daily workflow

### Capture a line

```bash
jot add "coffee with Sam about the migration"
```

Each `jot add` appends a `## HH:MM:SS` entry to today's file. Existing entries
are never rewritten.

Shorter aliases:

```bash
# ~/.bashrc or ~/.zshrc
alias j='jot add'
alias jt='jot show'
```

```fish
# ~/.config/fish/config.fish
alias j 'jot add'
alias jt 'jot show'
```

### Capture from other commands

`jot add` reads stdin when it gets no text, so command output can become an
entry:

```bash
git log -1 --format='%h %s' | jot add -j work
make test 2>&1 | tail -n 3 | jot add -j work
pbpaste | jot add                        # macOS clipboard
xclip -o | jot add                       # X11 clipboard
wl-paste | jot add                       # Wayland clipboard
```

Multi-line input is one entry. Blank lines inside the body are kept; leading
and trailing whitespace is trimmed.

### Write longer notes

`jot edit` opens the whole day in your editor, so you can keep adding to the
file by hand:

```markdown
## 09:00:00

standup

- parser merged
- docs next

## 14:30:00

Decision: keep the file format plain markdown.
```

`jot show` prints the file as written below a one-line bar naming the journal,
day, weekday and entry count. `jot ls` counts only `## ` headings.

### Track small todos

Checkboxes are ordinary markdown, so write them in the day file:

```markdown
## 09:00:00

- [ ] book the room
- [x] send the agenda
```

`jot todo` gathers every box across your journals, with the journal, day and
entry time. Filter and limit the list, then open the day to tick a box:

```bash
jot todo --open
jot todo -j work --limit 10
jot edit -d 2026-11-20 -j work
```

There is no toggle command: jot never rewrites entries, so the editor is where
a box changes. Use `--done` to review what is finished; without a filter, both
open and done items appear.

### Backdate an entry

```bash
jot add -d yesterday "forgot to log this"
```

The text goes into yesterday's file. The timestamp is the time you ran the
command.

### Keep separate journals

| Journal | Use it for |
| --- | --- |
| `default` | Whatever happens; the catch-all |
| `work` | Standups, decisions, meeting notes |
| `ideas` | Half-formed thoughts, free-form editing |
| `reading` | Quotes and takeaways |

Journals are plain directories under the root. Name one with `-j`. The name
`backups` is reserved for the backup directory and is rejected with an error:

```bash
jot add -j reading "Finished 'The Art of Doing Science and Engineering'"
jot edit -j ideas
```

To change the journal used when `-j` is not given, see
[Configuration](#configuration).

### Tidy journals

`jot journal` creates, renames and removes journals without touching the
directories by hand:

```bash
jot journal new scratch
jot journal rename scratch notes
jot journal rm --yes notes
```

`journal rm` writes an archive to `<root>/backups` before deleting anything.
Without `--yes` it asks you to type the journal name; a wrong answer leaves
the journal and the archive alone. The default journal cannot be removed or
renamed. Flags go before the name: `jot journal rm --yes notes`, not
`jot journal rm notes --yes`.

To switch journals for one command, use `-j`; in the TUI, `J` and `K` cycle
journals.

### Follow entries live

Keep a pane open while you work:

```bash
jot watch -j work
```

It prints the current day, then streams new text and switches to the next
day's file at midnight. It is read-only. Press Ctrl+C to stop. For a slow
filesystem, raise the poll interval with `--poll 2s`.

### Set a quick reminder

```bash
jot remind "stretch" 20m
jot remind 1h30m "review the draft"
jot remind list
jot remind clear
```

Durations accept `90s`, `20m`, `2h` and `1h30m`; a bare number means minutes.
The reminder shows up as a desktop notification. Nothing runs in the
background, and reminders do not survive a reboot. How reminders are delivered
on each system is in the [command reference](commands.md#remind).

## Searching and reviewing

### Read a day

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

### Read a whole journal

```bash
jot show -j work -a
```

`-a` prints every day, oldest first, under a `# <journal> · <day>` heading.

### See what exists

```bash
jot ls                     # journals, with entry and day counts
jot ls work                # days in one journal, with sizes
```

### Find a day without dates

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

### Search

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

### Find an old note

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

### Use other tools

Journal files are plain markdown in the root directory, so ordinary tools
work on them:

```bash
jot show -j work | glow -
rg -n --glob '*.md' "TODO" ~/.jot/work
```

If the root is not `~/.jot`, substitute the path shown on the `root` line of
`jot doctor`.

## The TUI

Run `jot` with no command in a terminal, or `jot tui`, to open the full-screen
interface: a month calendar, the selected day's rendered entries, a journal
sidebar and a one-line quick-add box. Bare `jot` in a pipe or redirect still
prints help.

- `h`/`l` move a day, `j`/`k` a week, `H`/`L` a month, and `t` returns to today.
- `a` adds one line without leaving the view; `e` opens the day in your editor.
- `/` fuzzy-searches every entry, `T` browses todos, and `r` sets a reminder.
- `J`/`K` switch journals, `n` creates one, `R` renames it, and `D` archives
  then deletes it after you type its name. `?` lists every keybinding, and
  `q` quits.

The quick-add box only appends a line. Editing an entry opens the day in your
editor, and jot never rewrites entries behind your back. The calendar colours
days by how many entries they hold. See [tui.md](tui.md) for the complete
keybinding reference.

## Editors

`jot edit` launches an editor. It is chosen in this order, first match wins:

1. `JOT_EDITOR`
2. the `editor` field in the config file
3. `$VISUAL`
4. `$EDITOR`
5. the first of `vim`, `vi` or `nano` found on `PATH`

```bash
export JOT_EDITOR=nvim
export JOT_EDITOR='code --wait'     # GUI editors must block until you close the file
export JOT_EDITOR='subl --wait'
export JOT_EDITOR='hx'
```

The editor command is split on whitespace. Arguments work, but shell quoting
does not, so an editor path containing spaces needs a symlink or a wrapper
script on `PATH`.

Check which editor is in use with the `editor` line of `jot doctor`.

If the editor returns at once and the file stays empty, the editor is usually a
GUI program without a wait flag. `jot edit` returns as soon as the editor
process exits, which for a launcher is immediately. Add `--wait` (or your
editor's equivalent).

## Backups and export

### Back up

```bash
jot backup                              # <root>/backups/jot-backup-<timestamp>.tar.gz
jot backup /mnt/usb/journal-backups     # somewhere else
```

Each archive holds every `.md` file under the root, stored as paths such as
`work/2026-11-05.md`. The default `backups` directory is skipped.

Inspect or restore an archive:

```bash
tar -tzf ~/.jot/backups/jot-backup-20261105-150405.tar.gz
tar -xzf ~/.jot/backups/jot-backup-20261105-150405.tar.gz -C ~/.jot
```

To run backups nightly, add a cron entry. Cron does not load your shell
profile, so use absolute paths:

```cron
15 3 * * * $HOME/.local/bin/jot backup $HOME/backups >> $HOME/.jot-backup.log 2>&1
```

A backup only helps if it leaves the machine. Copy archives to another disk or
host regularly.

### Export

```bash
jot export --out ~/Documents/journal-export
```

This writes one `<journal>.md` file per journal. Each file has a `# <journal>`
heading, a `## YYYY-MM-DD` heading per day, and the day's entries below it.
Exported files are created with mode `0644` (subject to your umask). Export
includes every entry in every journal. To share a single journal, use
`jot show -j reading -a > reading.md` instead.

### Import from other tools

**From a folder of markdown notes:** copy the files into a journal directory.
Files named `YYYY-MM-DD.md` work with `-d`. Files with other names still appear
in `jot show -a`, `jot ls` and `jot search`.

```bash
mkdir -p ~/.jot/imported
cp ~/notes/*.md ~/.jot/imported/
jot show -j imported -a | head
```

**From another tool's plain-text export:** each line becomes one entry dated
to the day you pass. Entries get the time of import, not the original time.

```bash
while IFS= read -r line; do
  [ -n "$line" ] || continue
  jot add -d 2026-01-01 -- "$line" </dev/null
done < old-journal.txt
```

## Updating and removing jot

### Update

`jot update` reads the latest GitHub release, compares it with the running
binary, prints what changed and asks before installing. The download is the
`jot-<os>-<arch>` asset, checked against the release's `SHA256SUMS` and swapped
in atomically.

```bash
jot update --check       # report without installing
jot update               # ask, then install
jot update --notes       # full release notes, not the first 15 lines
```

`--json` prints one machine-readable object for scripts, and `--version <tag>`
targets a specific release; it installs only when that tag is newer than the
running binary. For a source checkout, use
`jot update --source --src ~/code/jot --dir ~/.local/bin`. Set
`JOT_RELEASES_API` to a mirror when GitHub is unreachable. Restart jot, or the
TUI, after an update.

### Remove jot

`jot uninstall` removes selected components instead of everything:

```bash
jot uninstall --dry-run --all   # show the plan
jot uninstall --data            # root, including backups; asks for confirmation
jot uninstall --all --yes       # data, config and binary for a script
```

With no component flag and a terminal, it prints what exists and asks
`remove [d]ata, [b]inary, [c]onfig, [a]ll, or [q]uit?`. `--data` is the journal
root and its `backups`; `--binary` removes the running executable; `--config`
removes the config file. Before deleting data without `--yes`, it makes you
type `delete`; config and binary use `[y/N]` prompts.

`jot uninstall --yes` on its own still means data only, for compatibility with
older versions. Back up somewhere else first if the entries matter, because
data removal also deletes the default `backups` directory.

## Sync with git

The root is a plain directory, so git works without integration:

```bash
cd ~/.jot
git init
echo "backups/" >> .gitignore    # keep archives out of the repository
git add -A
git commit -m "journal"
```

Commit whenever you like, for example from cron:

```bash
cd ~/.jot && git add -A && git commit -m "$(date +%F)" --allow-empty
```

On a second machine, `jot setup` or `jot init` creates `~/.jot/default` first, and
`git clone` needs an empty target. Clone to a temporary path and move the `.git`
directory into the root:

```bash
git clone you@example.com:journal ~/journal-clone
mv ~/journal-clone/.git ~/.jot/
git -C ~/.jot status    # inspect the state before changing anything
```

jot only appends to journal files and never rewrites them, so diffs stay
small. If you also edit files by hand on two machines, resolve conflicts as
you would for any markdown file.

### Other sync tools

Syncthing, Dropbox, iCloud Drive and `rsync` all work on the plain files. Point
jot at the synced folder with `JOT_ROOT`:

```bash
export JOT_ROOT=~/Sync/journal
```

Three common layouts:

1. **Synced root.** `JOT_ROOT` points at a synced folder. The sync tool does the
   work.
2. **Git root.** Clone the journal repository to the root and commit
   periodically.
3. **Separate journals per machine.** Keep each machine's root local and share
   snapshots with `jot export`.

## Configuration

### Config file

The config file is `jot/config.json` in the user config directory: on Linux,
`~/.config/jot/config.json`; on macOS, `~/Library/Application Support/jot/config.json`.
Set `JOT_CONFIG` to use another path.

```json
{
  "root": "~/.jot",
  "default_journal": "daily",
  "editor": "code --wait",
  "reminders": "auto",
  "tui": {
    "theme": "nord"
  }
}
```

Every key is optional. A missing file means the defaults apply. `~` is expanded
in `root`. `reminders` picks how reminders are delivered: `auto` (the default)
tries Omarchy, then a systemd timer, then a background process with the
system's notifier; `omarchy`, `systemd` or `background` pins one; `off` turns
reminders off. See [`jot remind`](commands.md#remind). The `tui` block holds interface preferences: `theme` and the journal
last selected with `J`/`K` in the TUI. `jot setup` writes this file from its
answers, replacing the previous contents; it does not preserve the `tui` block.

### Environment variables

| Variable | Meaning |
| --- | --- |
| `JOT_ROOT` | Journal root (default `~/.jot`) |
| `JOT_JOURNAL` | Default journal name |
| `JOT_EDITOR` | Editor command for `jot edit` |
| `JOT_CONFIG` | Path of the config file |
| `JOT_INSTALL_DIR` | Default directory for `jot install`, `jot update` and `jot setup` |
| `JOT_SRC` | Source checkout for `jot update --source` |
| `JOT_THEME` | TUI colour theme, above `tui.theme` and below `--theme`; `show --render` uses it for light/dark |
| `JOT_RELEASES_API` | Full release API base URL for `jot update`, including `/releases` |

For the command-line settings, precedence highest first: flags, `JOT_*`
variables, the config file, built-in defaults. The TUI theme follows the same
order: `--theme`, then `JOT_THEME`, then `tui.theme` in the config file, then
`auto`.

To change the default journal for one shell session:

```bash
export JOT_JOURNAL=daily
```

### Files and permissions

- Journal files are created `0600` and journal directories `0700`, so only the
  owner can read them.
- The config file is written with mode `0600`.
- Exports are created with mode `0644` (subject to your umask). Backup
  archives get the default mode for new files (`0644` with a common umask);
  move them somewhere private if that matters.
- `jot update` contacts the GitHub releases API (or the mirror in
  `JOT_RELEASES_API`) and downloads the release binary; reminders call your
  existing notification tools. There is no account, telemetry or background
  daemon.
- jot does not encrypt data. Use full-disk encryption (LUKS, FileVault,
  BitLocker) or an encrypted volume for the root and the backup medium. Deleting
  a file does not guarantee its contents are gone from an SSD.

## Troubleshooting

Run `jot doctor` first. It checks the root, config file, journals, editor,
`PATH`, the reminder provider and optional helper binaries.

| Symptom | Cause and fix |
| --- | --- |
| ``nothing to add: pass text, pipe it in, or use `jot edit` `` | No text was given and stdin is a terminal. Pass the text as arguments or pipe it in. |
| `entry body is empty` | The text was blank and was piped in. Nothing was written. At a terminal, `jot add ""` prints `nothing to add` instead. |
| `no journal named "x" yet` | The journal does not exist. Check `jot ls`, then create it with `jot journal new x`, `jot add -j x "..."` or `jot init -j x`. |
| `invalid date "..."` | Use `today`, `yesterday`, `tomorrow`, or `YYYY-MM-DD`. |
| Entry is on the wrong day | Entries use local time. Check `date`, and use `-d` to pick the day. |
| Editor does not block | Use `--wait` (or the equivalent flag) for GUI editors. |
| `no editor found: set JOT_EDITOR or $EDITOR` | Set `JOT_EDITOR`, or install `vim`, `vi` or `nano`. |
| `jot` not found after install | Add the bin directory to `PATH`. `jot install` prints the line. |
| `no matches for "..."` | Plain search is a line-based substring match. Try a shorter query, or `jot search --fuzzy "..."` for entry-level matching. |
| A flag is treated as part of the query or name | Flags go before arguments: `jot search --fuzzy parser`, `jot journal rm --yes notes`. |
| `invalid month "..."` | `show --calendar` takes `YYYY-MM`, or no value for the current month. |
| `jot: config file <path>: ...` on commands that read config | The config file is not valid JSON. Fix or remove it. Every command that reads config stops at this error, `jot doctor` included. `jot version` and `jot help` still run. |
| Backup is missing a journal | Only `.md` files are archived. Check that the files end in `.md`. |
| `refusing to delete journal data without confirmation; re-run with --yes` | `jot uninstall --data` ran without a terminal. Pass `--yes` only if you mean it. |
| `refusing to remove anything without an explicit component` | `jot uninstall` had no terminal to show its menu. Pass `--data`, `--binary`, `--config` or `--all`; `--yes` alone means data only. |
| `no reminder provider: ...` | Run `jot doctor` to see what was found. Install a notifier (`notify-send` on Linux), or set `reminders` in the config file. |
| `jot tui needs an interactive terminal` | Run `jot tui`, or bare `jot`, in a real terminal rather than through a pipe. |
| `jot update` cannot reach the release API | Check the network, or point `JOT_RELEASES_API` at a mirror. |
| Empty day files (`0 entries`) | `jot edit` was quit without writing. Delete them with `find ~/.jot -name '*.md' -size 0 -delete`. |

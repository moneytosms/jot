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
# 2026-11-05

## 09:00 standup

- parser merged
- docs next

## 14:30 design review

Decision: keep the file format plain markdown.
```

`jot show` prints the file as written. `jot ls` counts only `## ` headings.

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

### Follow entries live

Keep a pane open while you work:

```bash
jot watch -j work
```

It prints the current day, then streams new text and switches to the next
day's file at midnight. It is read-only. Press Ctrl+C to stop. For a slow
filesystem, raise the poll interval with `--poll 2s`.

## Searching and reviewing

### Read a day

```bash
jot show                   # today
jot show yesterday
jot show 2026-10-01
jot show -j work -d all    # every day, same as -a
```

Add `--render` for a terminal-friendly view: headings get `→` and list items
get `•`.

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

### Search

```bash
jot search parser
jot search -j work standup
jot search "next steps"
```

Matching is case-insensitive substring search. Results look like
`work/2026-11-05.md:3: shipped the parser rewrite`, so most terminals and
editors can open them directly. Journals are listed alphabetically and days
newest first. If nothing matches, the output is `no matches for "..."`.

### Use other tools

Journal files are plain markdown in the root directory, so ordinary tools
work on them:

```bash
jot show -j work | glow -
rg -n --glob '*.md' "TODO" ~/.jot/work
```

If the root is not `~/.jot`, substitute the path shown on the `root` line of
`jot doctor`.

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

Each archive holds every `.md` file of every journal, stored as paths such as
`work/2026-11-05.md`. The default `backups` directory is not archived.

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
Exported files are `0644`, so other users on the machine can read them. Export
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

### Remove all data

```bash
jot uninstall              # asks first, deletes the root and every entry in it
rm "$(command -v jot)"     # remove the binary as well
```

`jot uninstall` deletes the root, including the default `backups` directory.
Back up somewhere else first if the entries matter. Without a terminal it
refuses unless `--yes` is given.

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

jot only appends to files and never rewrites them, so diffs stay small. If you
also edit files by hand on two machines, resolve conflicts as you would for
any markdown file.

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
  "editor": "code --wait"
}
```

All three keys are optional. A missing file means the defaults apply. `~` is
expanded in `root`. `jot setup` writes this file, replacing its previous
contents with the resolved values.

### Environment variables

| Variable | Meaning |
| --- | --- |
| `JOT_ROOT` | Journal root (default `~/.jot`) |
| `JOT_JOURNAL` | Default journal name |
| `JOT_EDITOR` | Editor command for `jot edit` |
| `JOT_CONFIG` | Path of the config file |
| `JOT_INSTALL_DIR` | Default directory for `jot install`, `jot update` and `jot setup` |
| `JOT_SRC` | Source checkout for `jot update` |

Precedence, highest first: command-line flags, `JOT_*` variables, the config
file, built-in defaults.

To change the default journal for one shell session:

```bash
export JOT_JOURNAL=daily
```

### Files and permissions

- Journal files are created `0600` and journal directories `0700`, so only the
  owner can read them.
- The config file is written with mode `0600`.
- Exports are `0644`. Backup archives get the default mode for new files
  (`0644` with a common umask); move them somewhere private if that matters.
- jot makes no network calls and has no account or telemetry.
- jot does not encrypt data. Use full-disk encryption (LUKS, FileVault,
  BitLocker) or an encrypted volume for the root and the backup medium. Deleting
  a file does not guarantee its contents are gone from an SSD.

## Troubleshooting

Run `jot doctor` first. It checks the root, config file, journals, editor and
`PATH`.

| Symptom | Cause and fix |
| --- | --- |
| ``nothing to add: pass text, pipe it in, or use `jot edit` `` | No text was given and stdin is a terminal. Pass the text as arguments or pipe it in. |
| `entry body is empty` | The text was blank and was piped in. Nothing was written. At a terminal, `jot add ""` prints `nothing to add` instead. |
| `no journal named "x" yet` | The journal does not exist. Check `jot ls`, then create it with `jot add -j x "..."` or `jot init -j x`. |
| `invalid date "..."` | Use `today`, `yesterday`, `tomorrow`, or `YYYY-MM-DD`. |
| Entry is on the wrong day | Entries use local time. Check `date`, and use `-d` to pick the day. |
| Editor does not block | Use `--wait` (or the equivalent flag) for GUI editors. |
| `no editor found: set JOT_EDITOR or $EDITOR` | Set `JOT_EDITOR`, or install `vim`, `vi` or `nano`. |
| `jot` not found after install | Add the bin directory to `PATH`. `jot install` prints the line. |
| `no matches for "..."` | Try a shorter query. Search is a case-insensitive substring match. |
| `jot: config file <path>: ...` on commands that read config | The config file is not valid JSON. Fix or remove it. Every command that reads config stops at this error, `jot doctor` included. `jot version` and `jot help` still run. |
| Backup is missing a journal | Only `.md` files are archived. Check that the files end in `.md`. |
| `refusing to delete journal data without confirmation; pass --yes` | `jot uninstall` ran without a terminal. Pass `--yes` only if you mean it. |
| Empty day files (`0 entries`) | `jot edit` was quit without writing. Delete them with `find ~/.jot -name '*.md' -size 0 -delete`. |

# Writing entries

Everything that puts text into a journal: one-liners, piped output, longer notes, todos, other days, other journals, reminders and editors.

## Capture a line

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

## Capture from other commands

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

## Write longer notes

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

## Track small todos

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

## Backdate an entry

```bash
jot add -d yesterday "forgot to log this"
```

The text goes into yesterday's file. The timestamp is the time you ran the
command.

## Keep separate journals

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
[Configuration](configuration.md).

## Tidy journals

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

## Follow entries live

Keep a pane open while you work:

```bash
jot watch -j work
```

It prints the current day, then streams new text and switches to the next
day's file at midnight. It is read-only. Press Ctrl+C to stop. For a slow
filesystem, raise the poll interval with `--poll 2s`.

## Set a quick reminder

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

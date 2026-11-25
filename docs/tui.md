# The TUI

The jot TUI is the full-screen view of your journals: a month calendar, a
journal sidebar, the selected day rendered as Markdown, and a one-line quick
add. It is read-mostly. The only change it makes to journal content is a
quick-add entry; edits, deletions, and checkbox changes happen in your editor.

For the command line, see the [command reference](commands.md). For first-run
setup, see [Getting started](getting-started.md).

## Starting the TUI

Run `jot` with no arguments in a terminal, or name the command:

```sh
jot
jot tui
```

Both forms use the same defaults. Add a journal or a theme:

```sh
jot tui -j work
jot tui --theme nord
jot tui -j work --theme gruvbox
```

| Flag | Meaning |
| --- | --- |
| `-j`, `--journal NAME` | Journal to open. |
| `--theme NAME` | Theme to start with. |

The journal is chosen in this order: `--journal`, then the last journal saved
in `tui.journal` if that journal still exists, then `default_journal`. A
journal name is sanitized before it is used as a directory.

The theme is chosen in this order: `--theme`, then `JOT_THEME`, then
`tui.theme`, then `auto`. See [Themes](#themes) for what `auto` does.

Both standard input and standard output must be terminals. jot does not look
at `TERM` to decide.

### The non-terminal fallback

Bare `jot` prints the overview help and exits 0 when either stream is not a
terminal. That is the safe behavior for pipes, redirects, scripts, and tests.

`jot tui` is stricter. It exits 1 with:

```text
jot: jot tui needs an interactive terminal; run jot help for the command line
```

Use the CLI in that situation. There is no headless TUI mode.

## The screen

The screen has five regions: the header, the journal sidebar, the month
calendar, the day pane, and the footer.

```text
jot  default  November 2026  7d 23e
 JOURNALS              +--------------------------------------+
 > default   12d 84e   | Wednesday, 25 November 2026          |
   work       4d 19e   |                                      |
                       | 09:41  shipped the parser rewrite    |
 <   November 2026  >  |                                      |
 Mo Tu We Th Fr Sa Su  |                                      |
                    1  |                                      |
  2  3  4  5  6  7  8  |                                      |
  9 10 11 12 13 14 15  |                                      |
 16 17 18 19 20 21 22  +--------------------------------------+
 / search - ctrl+p palette - T todos - S settings - ? help - q quit
 [status line: quick add, toast, or error]
```

The sketch is a layout, not a screenshot. The real TUI draws rounded borders,
uses theme colors, and sets the date with a middle dot; the sketch uses ASCII
punctuation and clips the calendar.

- **Header.** `jot`, the open journal, the displayed month, and two counts:
  days with a file in that month (`d`) and entries in that month (`e`). A
  short ellipsis appears while data loads. Clicking anywhere in the header
  jumps to today.
- **Sidebar.** The `JOURNALS` list, with the active journal marked by a
  pointer (the sketch uses `>`). Each row shows the journal name plus its day
  and entry counts. Names longer than ten characters are shortened. The list
  is a fixed region of at most eight rows: with more journals it scrolls
  (mouse wheel over the list, or `J`/`K`, which keep the open journal in
  view) and the header shows the visible range, e.g. `▼ 1-8/14`.
- **Calendar.** A month label (`‹ November 2026 ›`; click the arrows to change
  month) over a Monday-first month grid. Only days in the displayed month
  are drawn; leading and trailing cells are blank. The selected day is shown
  in reverse video. Today is underlined in the accent color unless it is also
  selected.
- **Day pane.** A bordered pane titled with the selected date and the day's
  entry and word counts, then a rule and the entries as a timeline: each
  entry's `HH:MM` in a left gutter with its rendered Markdown beside it. If the day is empty, it says
  `No entries on this day.` and `Press a to add a note, e to open the editor.`
- **Footer.** A context-sensitive hint line, then a status line. The status
  line holds the quick-add input, the new-journal prompt, a toast, or an
  error. Toasts clear after about three seconds. Errors are sticky: a later
  toast can cover one briefly, but it returns until another error replaces it
  or you restart the TUI.

`Tab` hides the whole left column (journals, calendar and shortcuts) so the
day pane takes the full width; press it again to bring it back. The journal
list is also hidden when the terminal is narrower than 70 columns. Below the
calendar, when there is room, a legend explains the colours (none, 1-5 and 6+
entries), then small widgets that appear only when they have something to show
and fit whole:

- **On this day:** the first line of entries written on today's date in
  earlier years; click one to open that day.
- **Open todos:** the count plus the first three unchecked items; click one to
  jump to its day.
- **Reminders:** the next two pending reminders.

Each can be hidden in Settings → General. Last comes a list of clickable
shortcuts for settings, search, todos, the year view and help.

Journals given a colour in Settings show their name in it. Entries render
`#tags` highlighted and `- [ ]` / `- [x]` items as ☐ / ☑. Terminals five rows tall or shorter show only the header and hint
line.

## Calendar navigation

| Key | Action |
| --- | --- |
| `h`, Left | previous day |
| `l`, Right | next day |
| `k`, Up | previous week |
| `j`, Down | next week |
| `H`, `<` | previous month |
| `L`, `>` | next month |
| `g` | first day file in the current journal |
| `G` | last day file in the current journal |
| `t` | today |
| `Tab` | hide or show the left column |
| `ctrl+r` | reload journals, counts, and the selected day |

Moving past the first or last day of a month loads the neighbouring month
automatically. `H` and `L` keep the same day number where possible and clamp
to the last day of a shorter month; they reset the day-pane scroll.

`g` and `G` jump to the first and last day that has a file in the current
journal. A day file can exist with no entries, and `g`/`G` do not skip those
files. If the journal has no day files, a toast says there are no entries yet.

### Heat levels

Calendar cells are colored by entry count:

| Entries | Level | Cell color |
| --- | --- | --- |
| 0 | 0 | muted |
| 1-2 | 1 | green |
| 3-5 | 2 | green |
| 6-9 | 3 | yellow |
| 10 or more | 4 | yellow |

The exact green and yellow come from the active theme. Selection overrides
the heat color; today is underlined in the accent color when it is not
selected. The header totals make the same counts for the whole month.

## The day pane

The pane title is the selected date, for example `Wednesday, 25 November 2026`.
Below it, the day's Markdown is rendered for the terminal: headings are
emphasized, lists get bullets, code and quotes are styled, and lines wrap to
the pane width.

The pane scrolls with the mouse wheel, three lines per notch. Browse mode has
no keyboard scroll keys. Search jumps attempt to scroll to the matching entry,
but that is best effort.

Quick add, the editor handoff, `ctrl+r`, and calendar movement all refresh the
pane. A day with a file but no entries looks the same as a day with no file.

## Quick add

Press `a` from the calendar. The status line shows the selected date and a
prompt for the note; `>` below stands in for the prompt glyph:

```text
add 2026-11-25 > note for 2026-11-25
```

Type the note and press Enter. jot appends a `## HH:MM:SS` entry to the
selected day in the open journal. The timestamp is the current time, even when
you have selected another day. The input clears and stays open for the next
note; press Esc to close it.

An empty or whitespace-only line is ignored: Enter does nothing and the input
stays open. If the write fails, the error appears on the status line.

The line editor supports typing, Backspace, Delete, Left/Right, Home/End, and
`ctrl+a`/`ctrl+e` for the start and end of the line. It accepts up to 512
runes.

Quick add is the only place the TUI writes journal content. It never rewrites
or deletes an existing entry.

## Opening a day in your editor

Press `e` from the calendar, or choose `Open day in editor` from the command
palette. The TUI suspends, launches your editor on the selected day's path,
and reloads the day when the editor exits.

The editor is chosen in this order:

1. `JOT_EDITOR`
2. the `editor` setting in the config file
3. `VISUAL`
4. `EDITOR`
5. the first of `vim`, `vi`, or `nano` on `PATH`

`JOT_EDITOR` outranks the config file because the environment overrides the
file when the config is loaded. The TUI runs the editor command through
`sh -c`, so flags and wrappers work, for example `nvim -u NONE`.

The TUI creates the journal directory and the day file if either is missing,
then passes the path to the editor. An editor that refuses to open new files is
not a problem: the file already exists.

If no editor can be found, the status line shows
`no editor found: set JOT_EDITOR or $EDITOR`. If the editor exits with an
error, the status line shows the error. The TUI itself stays running.

## Journals

`J` and `K` switch to the next and previous journal in sorted order. The
sidebar marks the open journal with a pointer (shown as `>` in the sketch) and
shows each journal's day and entry counts. Clicking a visible sidebar row
switches to that journal too.

`n` opens a new-journal prompt. Type a name and press Enter. The name is
sanitized to letters, digits, `-`, and `_`; other characters become `-`,
repeated dashes collapse, and leading or trailing dashes are trimmed. An empty
name, or one with no usable characters, becomes `default`. If the sanitized
name matches the open journal, a toast asks you to pick a different name.
Creating a journal switches to it.

The TUI saves the open journal in `tui.journal` when you switch journals,
create one, or jump to a hit or todo in another journal. Starting with
`-j NAME` does not by itself rewrite the saved value. On the next start it uses
that journal only if the directory still exists; otherwise it falls back to
`default_journal`. The CLI's own default is unchanged.

Journals are directories under the root. From the browse screen:

| Key | Action |
| --- | --- |
| `R` | rename the open journal: type a new name and press Enter |
| `D` | delete the open journal: type its exact name and press Enter |

`R` renames the directory and stays on it. `D` archives the journal to
`<root>/backups/jot-NAME-TIMESTAMP.tar.gz` first, then removes the
directory; the toast names the backup folder. A wrong confirmation name shows
a toast and deletes nothing. The configured `default_journal` is protected in
both prompts: the status line asks you to change `default_journal` first,
matching `jot journal rm` and `jot journal rename` on the CLI.

Deleting the open journal falls back to `default_journal` when it still
exists, else the first surviving journal. The new open journal is saved in
`tui.journal` like any other switch. The command palette offers the same two
actions as `Rename journal` and `Delete journal`.

## Searching all journals

Press `/` to open the fuzzy search. Type to query. The search is debounced by
150 ms, then runs over every entry in every journal. It matches entry text and
entry times, not journal names or day labels.

| Key | Action |
| --- | --- |
| other printable keys | edit the query |
| `Tab` | switch between all journals and this journal |
| `j` / `k` | typed into the query; next / previous result once Down/Up or `ctrl+j`/`ctrl+k` moved into the list |
| Down / Up, `ctrl+j` / `ctrl+k` | next / previous result |
| PageDown / PageUp, `ctrl+d` / `ctrl+u` | page down / page up |
| Home / End | first / last result |
| `g` / `G` | first / last result when the result list has focus |
| `/` | return focus to the query line |
| `alt+1` … `alt+8` | toggle the filter chip with that position |
| Enter | jump to the highlighted entry's day |
| Esc | close search |

While results are on screen, `j` and `k` move the selection even after you
press `/`; the result list has to clear first. Typing any other character, or
pressing Backspace, clears the results and resumes normal editing, after which
`j` and `k` are text again.

The result list shows journal, day, time, and the first line of each hit. When
the panel is wide enough, a preview of the highlighted entry appears beside the
list. `Tab` switches the scope and reloads immediately, without the debounce. A
scope line shows `all journals` or `this journal (NAME)` and the match count,
followed by filter chips: `[has:todo] [is:open] [after:7d] [after:30d]` and
your four most used `#tags`. Click a chip (or `alt+N`) to add that token to the
query, again to remove it; the two date chips replace each other. You can also
type any filter from [`jot search`](commands.md#search): `#tag`, `tag:`,
`journal:`, `after:`, `before:`, `has:todo`, `is:open`.

Enter jumps to the entry's day, switching journals first when the hit belongs
to another journal. The TUI remembers that journal in `tui.journal`. An empty
query shows a prompt to type; a query with no hits shows
`No matches. Try a shorter query.`

`Esc` closes search and returns to the calendar. Clicking a result activates
it; clicking outside the panel closes search. The mouse wheel scrolls the
result list.

### How fuzzy scoring works

Every rune of the query must appear in the entry, in order, ignoring case.
Scoring then rewards, in rough order of weight:

- a match at the start of the text
- matches at word starts (after a space, `-`, `_`, or `/`, or at a camelCase
  boundary)
- consecutive query runes
- shorter entry text

Each matched rune adds to the base score. Ties are broken by newest day first,
then by later time within a day. The match runs against the entry body plus
its time.

The command palette uses the same fuzzy matcher, with extra bonuses for exact,
prefix, and substring label matches.

## Command palette

Press `ctrl+p` to open the palette. It starts with the action list focused, so
`j`/`k` and the arrows move, PageUp/PageDown or `ctrl+u`/`ctrl+d` page, Home/End
jump to the ends, Enter runs the highlighted action, and Esc closes.

Typing filters the list. If the first character you want is `j` or `k`, press
`/` first to give the filter focus; otherwise that key moves the selection.
After you type, the arrow, page, Home, and End keys return focus to the list.

Clicking an action runs it. The mouse wheel scrolls the action list.

The actions are:

- Go to today
- Jump to a day
- Switch journal: NAME (one action per loaded journal)
- New journal
- Rename journal
- Delete journal
- Search entries
- Browse todos
- Year heatmap
- Set a reminder
- Settings
- Toggle theme (cycles the theme list)
- Open day in editor
- Backup journals
- Export journals
- Help
- Quit

`Jump to a day` opens an inline prompt that accepts `today`, `yesterday`,
`tomorrow`, or `YYYY-MM-DD`. `New journal` opens an inline name prompt;
`Rename journal` and `Delete journal` close the palette and open the same
prompt as `R` and `D`. The inline prompts validate on Enter and show the
error inside the palette; Esc closes the whole palette. Tab does nothing in a
prompt.

`Backup journals` and `Export journals` run in the background and report the
archive path or export count in a toast.

## Todos

Press `T` to collect checkbox items into one overlay. By default the scope is
every journal; `Tab` switches to the current journal and back.

| Key | Action |
| --- | --- |
| `j` / `k`, Down / Up | next / previous item |
| PageDown / PageUp, `ctrl+d` / `ctrl+u` | page down / page up |
| Home / End, `g` / `G` | first / last item |
| `Tab` | switch scope |
| Enter | open that item's day in your editor |
| `r` | open the reminder form prefilled with the item text |
| Esc | close |

Open items are listed before done items; within each group the newest day
comes first. Each row shows `[ ]` or `[x]`, journal, day, time, and text.

Enter switches to the item's journal when needed, selects its day, and opens
the editor. Closing the editor returns to the calendar, not the todo overlay.
Press `T` again to rebuild the list. `r` carries the item text into the
reminder message field, with the usual 20-minute default duration.

**The overlay is read-only.** jot never toggles a checkbox. To complete an
item, edit the day and change `- [ ]` to `- [x]` yourself, then reopen the
overlay. This is deliberate: the Markdown file is the source of truth, and
jot does not rewrite entries.

Clicking a todo row opens it in the editor. The mouse wheel scrolls the list.

## Reminders

Press `r` from the calendar to open the reminder form. From the calendar, the
message starts with the newest non-empty entry line on the selected day, when
there is one. From the todo overlay, `r` prefills the message with the selected
item's text. The form has two fields, message and duration, and starts with a
20-minute duration.

| Key | Action |
| --- | --- |
| `Tab`, Up, Down | switch between message and duration |
| Enter | set the reminder |
| Esc | cancel |

The message must not be empty. Durations accept Go duration syntax and a bare
number of minutes:

- `20m` - twenty minutes
- `2h` - two hours
- `90s` - ninety seconds
- `1h30m` - ninety minutes
- `45` - forty-five minutes

Zero, negative, and malformed durations are rejected in the form. If the
message or duration is invalid, the form stays open with the error.

On success the form closes and a toast reports the delay and fire time, for
example `reminder set for 20m at 15:04`.

### Providers and lifetime

The Reminders row in Settings (the `reminders` config key) picks the
provider. With `auto`, jot uses the first available one:

1. **Omarchy.** If `omarchy` is on `PATH`, jot runs
   `omarchy reminder MINUTES MESSAGE`. Omarchy is minute-granular, so partial
   minutes round up, and `auto` skips it for reminders under a minute.
2. **systemd.** On Linux with `systemd-run` and `notify-send`, jot creates a
   transient systemd user timer that shows the message with `notify-send`.
3. **Background.** A detached process waits, then shows the message with
   `notify-send`, `osascript` on macOS, or PowerShell on Windows.
4. **Nothing.** The form shows `no reminder provider: ...`.

Pinning `omarchy`, `systemd` or `background` uses only that provider; `off`
turns reminders off. If a provider's command fails, jot reports that error and
does not retry through another provider.

Reminders are transient. There is no jot daemon and no database of reminders;
nothing is restored after a reboot, and a logout normally stops a systemd
timer. The TUI has no list or clear command;
the CLI's `jot remind list` and `jot remind clear` operate on systemd and
background reminders, so Omarchy reminders do not appear there.

Provider errors stay in the form. If you close the form before the result
arrives, the error goes to the status line instead. Successful reminders
produce a toast.

## Settings

Press `S`, or click `S settings` under the calendar, to open the settings
panel. It has four tabs; `Tab` and `Shift+Tab` (or a click on a tab name)
switch between them, Up and Down move between rows, Esc closes.

**General**

| Row | Behavior |
| --- | --- |
| Theme | Enter or Left/Right cycles the ten themes; saves `tui.theme`. |
| Editor command | Enter saves the `editor` key. Empty means auto-detect. |
| Default journal | Enter saves `default_journal` and warns when that journal does not exist yet. |
| Reminders | Enter or Left/Right cycles `auto`, `omarchy`, `systemd`, `background`, `off`; saves `reminders` and shows which provider it resolves to. |
| Git sync | Enter or Left/Right cycles `off`, `commit`, `push`; saves `git`. |
| On this day, Open todos, Reminders | Enter shows or hides that widget under the calendar; saves `tui.hide`. |
| Config file | Read-only path of the config file in use. |

**Journals** lists every journal with its days, notes, last write and
`open`/`default`/`encrypted`/colour tags; the list scrolls with Up/Down, `j`/`k` or the wheel.

| Key | Action on the highlighted journal |
| --- | --- |
| Enter, `o` | open it and close settings |
| `r` | rename (prompt in the status line) |
| `a` | archive to `<root>/backups/jot-<name>-<stamp>.tar.gz`, journal untouched |
| `d` | delete: type the name to confirm; it is archived first |
| `s` | make it the default journal |
| `x` | press twice: encrypt with age, or decrypt back to markdown (see [`jot journal encrypt`](commands.md#journal)) |
| `t` | edit its daily template (`.template`) in your editor |
| `c` | cycle its sidebar colour: none, green, yellow, cyan, magenta, red, blue |
| `n` | create a new journal |

The default journal cannot be renamed or deleted.

**Storage** changes where journals live. Type a path in `New root` (`~` is
expanded), then pick a row and press Enter:

| Row | Behavior |
| --- | --- |
| Move journals | Press Enter twice. Moves the whole root (journals and backups) to the new path, which must be missing or empty and not inside the current root, then saves `root`. Across filesystems it copies, keeping file modes, and removes the old root only after the copy succeeds. |
| Use path only | Saves `root` without moving anything, for pointing jot at an existing journal folder. |
| Back up .tar.gz / .zip | Writes a full backup (journals, templates, config; never the key) into `<root>/backups`, the same as `jot backup [--zip]`. The command palette has both too. |

Both refuse while `JOT_ROOT` is set, since the variable overrides the config.

**Archives** lists the archives and backups in `<root>/backups`, newest first.
Enter restores the highlighted one; it refuses when a journal in it already
exists, so rename or delete that journal first.

Saving the editor writes `editor`, which affects `jot edit` as well as the
TUI's `e` handoff. Setting the default journal does not switch the journal
currently open; it changes what commands use when no journal is named. The TUI
remembers its own open journal in `tui.journal`.

Settings saves load the existing config, change one field, and write the whole
file back atomically with mode `0600`, so unrelated settings are preserved. The
config path honors `JOT_CONFIG`; when that is unset, jot uses the user config
directory, usually `~/.config/jot/config.json` on Linux.

Clicking an editable row focuses it; clicking the Theme or Reminders row also
cycles it. The two path rows are informational and cannot be focused. Clicking
outside the panel closes it.

## Themes

The available names are:

- `auto`
- `default`
- `light`
- `dark`
- `gruvbox`
- `nord`
- `dracula`
- `catppuccin`
- `tokyo-night`
- `omarchy`

`default` follows the terminal background; `dark` always uses the fixed dark
palette, even on a light terminal. Unknown names fall back to the built-in
palette for the terminal background; jot does not reject them. `auto` resolves
in this order:

1. If `NO_COLOR` is set, use an uncolored theme.
2. If the Omarchy theme file exists and parses, use it. On Linux the path is
   `$XDG_CONFIG_HOME/omarchy/current/theme/colors.toml`, normally
   `~/.config/omarchy/current/theme/colors.toml`.
3. Otherwise use the built-in dark or light palette based on the terminal's
   reported background.

The Omarchy file must define valid `background` and `foreground` colors.
Missing semantic colors are filled from the built-in dark palette, and accent
and selection foregrounds are chosen for contrast. A missing, unreadable, or
invalid file is ignored silently by `auto`.

`default` is the built-in palette matched to the terminal background. `light`
is always the built-in light palette. The named dark themes are fixed
palettes. `--theme omarchy` uses the Omarchy file when it parses; otherwise it
falls back to the built-in dark palette.

Theme precedence is `--theme`, then `JOT_THEME`, then `tui.theme`, then
`auto`, which matches how the other `JOT_*` variables override the config
file.

Cycling the theme in Settings applies it immediately and persists `tui.theme`.
`NO_COLOR` only affects `auto`; naming a colored theme explicitly still uses
that theme.

## Mouse

Mouse reporting is enabled when the TUI starts. If your terminal uses the
mouse for text selection, hold Shift while selecting, as usual for
terminal applications.

| Area | Action |
| --- | --- |
| Header | jump to today |
| Calendar day | select that day; each day is a four-column cell, two rows tall when there is room |
| Shortcut under the calendar | acts like its key: settings, search, todos, year, help |
| Sidebar journal | switch to that journal |
| Day pane | wheel scrolls three lines per notch |
| Search | click a result to jump; wheel scrolls; click outside closes |
| Palette | click an action to run it; wheel scrolls; click outside closes |
| Todos | click a row to open it in the editor; wheel scrolls |
| Settings | click an editable row to focus it; Theme and Reminders also cycle; click outside closes |
| Reminder | click a field to focus it; click outside closes |
| Help | wheel scrolls; click outside closes |

Clicks only act on visible items. If the sidebar is hidden because the
terminal is narrow, there is nothing to click there.

## Key reference

These are the keys the code actually handles. Letter keys are case-sensitive.

### Browse

| Key | Action |
| --- | --- |
| `h`, Left | previous day |
| `l`, Right | next day |
| `k`, Up | previous week |
| `j`, Down | next week |
| `H`, `<` | previous month |
| `L`, `>` | next month |
| `g` | first day file |
| `G` | last day file |
| `t` | today |
| `Tab` | hide or show the left column |
| `J` / `K` | next / previous journal |
| `n` | new journal |
| `R` | rename the open journal |
| `D` | delete the open journal (archive + typed confirmation) |
| `a` | quick add |
| `e` | open selected day in editor |
| `ctrl+r` | refresh |
| `r` | reminder |
| `/` | search |
| `ctrl+p` | command palette |
| `T` | todos |
| `y` | year heatmap of the open journal |
| `S` | settings |
| `?` | help |
| `q` | quit |
| `ctrl+c` | quit from anywhere |

### Quick add and new journal

| Key | Action |
| --- | --- |
| printable keys | type |
| Backspace, Delete | delete |
| Left, Right | move the cursor |
| Home, End, `ctrl+a`, `ctrl+e` | start / end of line |
| Enter | append the note / create the journal |
| Esc | cancel |

### Search

| Key | Action |
| --- | --- |
| other printable keys | query |
| `Tab` | all journals / current journal |
| `j`, `k` | next / previous result while results are visible |
| Down, Up, `ctrl+j`, `ctrl+k` | next / previous result |
| PageDown, PageUp, `ctrl+d`, `ctrl+u` | page |
| Home, End | first / last result |
| `g`, `G` | first / last result when the list has focus |
| `/` | return focus to the query |
| Enter | jump to the result |
| Esc | close |

### Command palette

| Key | Action |
| --- | --- |
| other printable keys | filter |
| `j`, `k` | next / previous action while the list has focus |
| Down, Up | next / previous action |
| PageDown, PageUp, `ctrl+d`, `ctrl+u` | page |
| Home, End | first / last action |
| `/` | return focus to the filter when the list has focus |
| Enter | run the action, or submit a prompt |
| Esc | close |

### Todos

| Key | Action |
| --- | --- |
| `j`, `k`, Down, Up | next / previous item |
| PageDown, PageUp, `ctrl+d`, `ctrl+u` | page |
| Home, End, `g`, `G` | first / last item |
| `Tab` | all journals / current journal |
| Enter | open the item's day in the editor |
| `r` | reminder prefilled with the item text |
| Esc | close |

### Reminder

| Key | Action |
| --- | --- |
| printable keys | edit the focused field |
| `Tab`, Up, Down | switch field |
| Enter | set the reminder |
| Esc | cancel |

### Settings

| Key | Action |
| --- | --- |
| `Tab`, Down | next editable row |
| `Shift+Tab`, Up | previous editable row |
| Enter | cycle theme or save the editor/journal row |
| Left, Right | cycle theme on the Theme row |
| Esc | close |

### Help

| Key | Action |
| --- | --- |
| `j`, `k`, Down, Up | scroll |
| PageDown, PageUp, `ctrl+d`, `ctrl+u` | page |
| Home, End, `g`, `G` | first / last line |
| `?`, Esc | close |

## Environment variables

| Variable | Meaning |
| --- | --- |
| `JOT_ROOT` | Journal root directory. |
| `JOT_CONFIG` | Alternative config file path. |
| `JOT_EDITOR` | Editor command, above `VISUAL` and `EDITOR`. |
| `JOT_THEME` | Theme, above `tui.theme` and below `--theme`. |
| `JOT_JOURNAL` | Default journal name. |
| `NO_COLOR` | Makes `auto` use an uncolored theme. |
| `VISUAL`, `EDITOR` | Editor fallbacks after `JOT_EDITOR` and the config. |

`XDG_CONFIG_HOME` controls where the Omarchy theme file and the default
config file are looked up, through the platform's user config directory.

## Troubleshooting

- **`jot tui needs an interactive terminal`** - Run it directly in a terminal,
  not through a pipe or redirect. Use the CLI otherwise.
- **Bare `jot` prints help instead of opening the TUI** - One of the streams
  is not a terminal. Run it in a terminal; scripts get help by design.
- **No colors with `auto`** - Check whether `NO_COLOR` is set. `auto`
  intentionally drops color when it is. Unset it or choose a named theme.
- **No colors in a pipe** - Expected. The TUI refuses non-terminals;
  `jot show --render` also drops ANSI when stdout is not a terminal or
  `NO_COLOR` is set.
- **`--theme omarchy` looks like the default dark theme** - The Omarchy file
  is missing or invalid. Check `~/.config/omarchy/current/theme/colors.toml`
  and make sure `background` and `foreground` are valid hex colors.
- **Reminders unavailable** - Run `jot doctor`; it shows which provider the
  `reminders` setting resolves to. Install a notifier, or pick another
  provider in Settings.
- **An Omarchy reminder fails** - jot does not retry through another
  provider. Check that `omarchy notification send test` works (it fails with
  `The name is not activatable` when no notification service is running), or
  pick `systemd` in Settings.
- **Busy days look uncoloured** - Some themes define a dim grey `yellow`. jot
  replaces a heat colour that blends into the empty-day colour with the next
  distinct palette colour; the legend under the calendar shows the result.
- **The sidebar is missing** - The terminal is under 70 columns. Widen it, or
  accept the calendar-only layout. If the whole left column is gone, press
  `Tab`.
- **`no editor found`** - Set `JOT_EDITOR` or `EDITOR`, or install `vim`,
  `vi`, or `nano`.
- **The screen is mostly empty** - Terminals five rows tall or shorter show
  only the header and hint line. Make the window taller and wider.
- **Config errors on startup** - Run `jot doctor`, fix the JSON, or point
  `JOT_CONFIG` at another file.

## What the TUI does not do

- It does not edit existing entries inline. Quick add appends one line;
  everything else is handed to your editor.
- It does not delete entries or days. Journal deletion is the one exception:
  `D` archives the journal first and asks for its exact name. Content changes
  still go through your editor.
- It does not toggle todo checkboxes. The overlay is read-only.
- It does not sync, serve, or watch a remote. jot has no server and no sync.
- It does not run a background daemon. Reminders are transient system timers
  or Omarchy reminders.
- It does not preserve reminders across a reboot.
- It does not provide search-and-replace, tags, attachments, or encryption.

For journal content changes, the editor is the interface. The TUI is for
finding the day, reading it, adding a quick note, and getting out of the way.

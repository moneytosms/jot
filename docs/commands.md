# Command reference

Every command in the order `jot help` lists them. Each section gives the
synopsis, what the command does, its flags, and one example with its output.

```
jot <command> [flags] [arguments]
```

## Global forms

| Invocation | Result |
| --- | --- |
| `jot`, `jot help`, `jot -h`, `jot --help` | Overview help |
| `jot help <command>` | Help page for one command |
| `jot <command> -h` or `--help` | Help page for that command, except `version` and `help`, which do not accept `-h` |
| `jot version`, `jot -v`, `jot --version` | Prints `jot <version>` |

Exit status is `0` on success and `1` on any error (unknown command, invalid
date, missing journal, failed check). `jot doctor` exits `1` when a check
fails. `install.sh` exits `2` for an unknown option and `1` when Go is missing;
a failed build exits with the status of `go build`.

Flags accept one or two dashes: `-journal` and `--journal` are the same flag.

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
terminal, `jot add ""` prints `nothing to add` instead. Entries are appended,
never rewritten. Missing directories are created.

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

Prints one day of a journal as stored (the raw markdown), or every day with
`-a`. The day can be given with `-d` or as the first argument, so
`jot show yesterday` works.

With `-a` (or `-d all`), every day prints oldest first under a
`# <journal> · <day>` heading. `--render` converts headings to `→` and list
items to `•` for reading in a terminal.

| Flag | Default | Meaning |
| --- | --- | --- |
| `-j`, `--journal <name>` | configured default | Journal to read |
| `-d`, `--date <day>` | `today` | Day to print, or `all` |
| `-a`, `--all` | off | Print every day, with headings |
| `--render` | off | Render markdown for the terminal |

Messages:

| Situation | Output |
| --- | --- |
| Day has no entries | `no entries in <journal> for <day>` (exit 0) |
| `-a` and the journal has no day files | `no entries in <journal> yet` (exit 0) |
| Journal does not exist | `no journal named "<journal>" yet; ...` (exit 1) |
| Invalid day | `invalid date "<value>": ...` (exit 1) |

```bash
jot show -j work -a
```

```
# work · 2026-11-04

## 09:12:01

standup notes

# work · 2026-11-05

## 08:55:40

...
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
skipped; timestamp headings are searched like any other line.

Results print as `journal/day.md:line: text`, journals in alphabetical order
and days newest first within each journal. No matches prints
`no matches for "<query>"` and exits `0`. An empty query fails with
`usage: jot search <query>`.

| Flag | Default | Meaning |
| --- | --- | --- |
| `-j`, `--journal <name>` | all journals | Limit the search to one journal |

```bash
jot search parser
```

```
work/2026-11-05.md:3: shipped the parser rewrite
default/2026-11-03.md:7: parser bug: empty input
```

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
(local time, to the second), containing every `.md` file of every journal.
Entries are stored relative to the root, for example `work/2026-11-05.md`.

The default destination is `<root>/backups`. That directory is never archived
or listed as a journal. The journal name `backups` is reserved and is rejected
with an error, so every journal is archived. A positional `dir` overrides `--out`; the directory is
created if missing. Two backups in the same second overwrite each other.

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

Exported files are mode `0644`, so other users on the machine can read them.
Export includes every journal; review the output before sharing it. The
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

Existing settings become the defaults, and the config file is then rewritten
with the answers. It is written atomically with mode `0600`. If the bin
directory is not on `PATH`, the summary says so.

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

The prompts that appear before this summary are omitted here. With
`--no-install`, the editor prompt is the only one shown.

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

Rebuilds jot from a local source checkout and replaces the installed copy. This
is the upgrade path for a source install; there are no downloaded releases.

The checkout is taken from `--src`, then `JOT_SRC`, then the current directory.
From that directory, the command walks up until it finds a `go.mod` declaring
`module github.com/moneytosms/jot`. It runs
`go build -trimpath -o <dir>/jot ./cmd/jot`, then runs the new binary's
`version` command to show that it works. Requires `go` on `PATH`.

| Flag | Default | Meaning |
| --- | --- | --- |
| `--src <dir>` | `JOT_SRC`, then the current directory and its parents | Source checkout |
| `--dir <dir>` | `JOT_INSTALL_DIR`, or `~/.local/bin` | Install directory |

```bash
cd ~/code/jot && jot update
```

```
rebuilt and installed <dir>/jot
jot 0.0.0-dev
```

---

## uninstall

```
jot uninstall [flags]
```

Deletes the journal root and everything in it, including the default backup
directory. Back up first if the entries matter; `jot backup ~/somewhere-else`
keeps copies outside the root. The binary is left in place.

Safety:

- Without `--yes` and without a terminal on stdin, it refuses and exits `1`.
- In a terminal it asks `Delete <root> and every entry in it? [y/N]`. Only `y`
  or `yes` proceeds.
- It refuses to remove `/`, `.` and the home directory itself.
- A missing root is an error.
- Files are removed, not shredded.

| Flag | Default | Meaning |
| --- | --- | --- |
| `-y`, `--yes` | off | Skip the confirmation prompt |

```bash
jot uninstall --yes
```

```
deleted <root>
the jot binary was left in place; remove it with: rm "$(command -v jot)"
```

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
prints the same page. An unknown command name fails with
``no help for "<name>"; run `jot help` ``.

```bash
jot help backup
```

Prints the help page for `backup`, the same output as `jot backup -h`.

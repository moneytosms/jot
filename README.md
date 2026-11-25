<div align="center">
  <img src="assets/readme/hero.svg" alt="jot: a plaintext journal you can read with cat. A day file dated 2026-11-25 with timestamped entries beside a heat-dot month strip." width="100%">
</div>

# jot

jot is a fast, plaintext journal: one Markdown file per day, a scriptable CLI,
and a full-screen terminal interface.

[Website](https://moneytosms.github.io/jot/) ·
[Usage guide](https://moneytosms.github.io/jot/guide/) ·
[Command reference](docs/commands.md)

A 55-second tour: quick adds from the CLI, the TUI's calendar, fuzzy search and
year heatmap, then todos, tags, reminders, encryption, PDF export, backups and
git sync. Every line of terminal output comes from the real binary:

![A tour of jot: quick adds, the TUI calendar and search, then todos, tags, reminders, encryption, PDF export, backups and git sync](assets/readme/tour.mp4)

## Why

Your notes are ordinary Markdown files in a directory you own. Read them with
`cat`, edit them with any editor, search them with `grep`, back them up with
`tar`, and sync them with whatever you already use. There is no database, no
background process, and no lock-in.

A day, as written on disk:

```markdown
## 09:41:07

shipped the parser rewrite
the whole day is one file and one diff

## 15:02:33

quick add is the only write the TUI makes
```

Entries are appended under a `## HH:MM:SS` heading; earlier bytes are never
rewritten.

## Features

- **One file per day.** Every day is `<root>/<journal>/YYYY-MM-DD.md`,
  appended under `## HH:MM:SS` headings and never rewritten.
- **Calendar TUI.** A month with entry heat, the selected day rendered as
  Markdown, quick add (`a`), editor handoff (`e`), a year heatmap (`y`), and
  widgets for open todos, reminders and "on this day".
- **Journals.** Keep separate journals, switch with `-j` or `J`/`K`, give
  each a sidebar colour and a daily `.template`.
- **Search.** Substring or fuzzy, across every journal, with filters:
  `#tag`, `has:todo`, `is:open`, `after:7d`, `before:2026-01-01`.
- **Tags.** Write `#work` anywhere; `jot tags` counts them.
- **Todos.** `- [ ]` boxes in any entry show up in `jot todo` and the TUI's
  `T` overlay. jot never toggles them; your editor does.
- **Reminders.** `jot remind "stretch" 20m` pops a desktop notification via
  Omarchy, a systemd timer or a background process.
- **Encryption.** `jot journal encrypt private` turns a journal into
  [age](https://age-encryption.org) ciphertext; jot still reads and searches it.
- **Git sync.** Set `"git": "commit"` (or `"push"`) and every write is a commit.
- **Backups and export.** `jot backup` writes a `.tar.gz` or `.zip` of
  everything; `jot journal restore` brings it back. `jot export` writes
  Markdown, HTML or a PDF with a cover page and contents.
- **Ten themes.** `auto`, `default`, `light`, `dark`, `gruvbox`, `nord`,
  `dracula`, `catppuccin`, `tokyo-night`, and `omarchy`.
- **Checked self-update.** `jot update` verifies the release against
  `SHA256SUMS` before replacing the binary.
- **Tight permissions.** Journal directories are `0700`; day files and the
  config file are `0600`.

## Install

### Release script

```sh
curl -fsSL https://raw.githubusercontent.com/moneytosms/jot/main/install-release.sh | sh
```

The script downloads the right release for your machine, verifies it, and
installs it to `~/.local/bin`. Set
`JOT_INSTALL_DIR` to choose another directory, or `JOT_VERSION` to pin a
release.

### Build from source

Go 1.24 or newer is required.

```sh
go install github.com/moneytosms/jot/cmd/jot@latest
```

Or from a checkout:

```sh
git clone https://github.com/moneytosms/jot
cd jot
make install                    # build and copy to ~/.local/bin
BINDIR=$HOME/bin make install   # or choose another directory
```

`make build` writes `bin/jot` without installing it. `make check` runs vet
and the test suite.

### Upgrade

```sh
jot update --check   # show the available release without installing
jot update           # download, verify and install
```

`jot update` verifies the download against the release's `SHA256SUMS` and
replaces the binary only after the checksum matches. See
[docs/commands.md](docs/commands.md) for the full set of update flags.

### First run

```sh
jot setup    # config, root, default journal, optional binary install
jot doctor   # check root permissions, config, journals, editor and PATH
```

## Layout and keys

A layout sketch, not a screenshot; punctuation is simplified and the calendar
is clipped here:

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

| Key | Action |
| --- | --- |
| `h`, `l`, Left, Right | previous / next day |
| `j`, `k`, Up, Down | previous / next week |
| `H`, `L`, `<`, `>` | previous / next month |
| `g`, `G` | first / last day file in the journal |
| `t` | today |
| `Tab` | toggle the journal sidebar |
| `J` `K` | next / previous journal |
| `n` / `R` / `D` | new / rename / delete journal |
| `a` / `e` | quick add / open in editor |
| `/` / `ctrl+p` | search / command palette |
| `T` / `S` / `?` | todos / settings / help |
| `r` / `ctrl+r` | reminder / reload |
| `q`, `ctrl+c` | quit |

Press `?` inside the TUI for the full cheatsheet, or read
[The TUI](docs/tui.md) for the complete reference.

## Everyday use

```sh
jot add "shipped the parser rewrite"   # append to today
jot show --calendar                    # month grid with entry density
jot show -d yesterday                  # yesterday's entries
jot search --fuzzy parser              # scored search across journals
jot search "#work is:open"             # tag and todo filters
jot todo --open                        # unchecked boxes, every journal
jot tags                               # tags by use
jot remind "stretch" 20m               # desktop notification in 20 minutes
jot journal new ideas                  # create a journal
jot journal encrypt ideas              # age-encrypt it
jot export --format pdf -j work        # a PDF to print or send
jot backup --zip                       # everything, in one archive
jot tui                                # open the interactive interface
```

`jot show` prints today's entries. `jot edit` opens today in `$JOT_EDITOR`,
the configured editor, `$VISUAL`, `$EDITOR`, or the first of `vim`, `vi`, or
`nano` found on `PATH`.

## Storage and config

```text
~/.jot/
  default/
    2026-11-25.md
  work/
    .template
    2026-11-25.md
  backups/
    jot-backup-20261125-150405.tar.gz
```

The config file is `~/.config/jot/config.json` (`$JOT_CONFIG` overrides).
Every key is optional:

```json
{
  "root": "~/.jot",
  "default_journal": "default",
  "editor": "nvim",
  "reminders": "auto",
  "git": "commit",
  "tui": { "theme": "auto", "colors": { "work": "cyan" } }
}
```

All keys and `JOT_*` variables: [Configuration](docs/configuration.md).
Backups, export, encryption and sync: [Your data](docs/your-data.md).
`jot update` and `jot uninstall`: [Getting started](docs/getting-started.md#update).

## Compatibility and scope

- The CLI is the stable surface. Existing flags and output are preserved.
- jot never rewrites an entry. Edits, deletions and checkbox toggles go
  through your editor.
- Journals are flat directories: no nested journals, no database.
- No server, no account, no telemetry. Sync is git or whatever already syncs
  your files.
- Building from source needs Go 1.24 or newer.

## Documentation

- [Usage guide](https://moneytosms.github.io/jot/guide/) — the repo's own
  docs, rendered and searchable
- [Getting started](docs/getting-started.md) · [Writing entries](docs/writing.md) ·
  [Finding and reviewing](docs/finding.md) · [The TUI](docs/tui.md)
- [Your data](docs/your-data.md) · [Configuration](docs/configuration.md) ·
  [Troubleshooting](docs/troubleshooting.md)
- [Command reference](docs/commands.md) · [Development](docs/development.md)

## Development

```sh
make check   # go vet + go test ./...
make build   # write bin/jot
```

Dependencies are vendored in `vendor/`, so builds and tests work offline. Go
1.24 or newer is required. See [docs/development.md](docs/development.md) for
layout, testing, and release details.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

MIT. See [LICENSE](LICENSE).

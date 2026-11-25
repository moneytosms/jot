<div align="center">
  <img src="assets/readme/hero.svg" alt="jot: a plaintext journal you can read with cat. A day file dated 2026-11-25 with timestamped entries beside a heat-dot month strip." width="100%">
</div>

# jot

jot is a fast, plaintext journal: one Markdown file per day, a scriptable CLI,
and a full-screen terminal interface.

[Website](https://moneytosms.github.io/jot/) ·
[Usage guide](https://moneytosms.github.io/jot/guide/) ·
[Command reference](docs/commands.md)

## Why

Your notes are ordinary Markdown files in a directory you own. Read them with
`cat`, edit them with any editor, search them with `grep`, back them up with
`tar`, and sync them with whatever you already use. There is no database, no
jot daemon, and no lock-in. The core Go library uses only the standard
library; the TUI is built with Bubble Tea.

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

- **One file per day.** Every day is `<root>/<journal>/YYYY-MM-DD.md`.
- **Calendar with entry heat.** The TUI shows a month at a glance. Cell color
  reflects how many entries a day has, and the header totals the displayed
  month's days and entries.
- **Rendered day pane.** The selected day is rendered as Markdown in the
  terminal, with a scrollable entry pane.
- **Quick add.** Press `a`, type one line, press Enter. The entry is appended
  to the selected day and the input stays open for the next note.
- **Editor handoff.** Press `e` to open the selected day in your editor, then
  return to the TUI. Long notes, edits, deletions, and checkbox changes all
  happen there.
- **Journals.** Keep separate journals, switch them with `-j` or `J`/`K`, and
  create one from the TUI with `n`.
- **Fuzzy search across journals.** Search every entry by default, or limit
  the search to the current journal with Tab.
- **Read-only todos.** Collect every `- [ ]` and `- [x]` item in one overlay.
  jot never toggles boxes; you do that in your editor.
- **Countdown reminders.** Set a transient reminder through Omarchy when it is
  available, or through a systemd user timer plus `notify-send`.
- **Ten themes.** `auto`, `default`, `light`, `dark`, `gruvbox`, `nord`,
  `dracula`, `catppuccin`, `tokyo-night`, and `omarchy`. `dark` is the
  fixed dark palette; `default` follows the terminal background. The Omarchy
  theme is read from the current Omarchy theme file.
- **Backups and exports.** `jot backup` writes a compressed archive of every
  journal; `jot export` writes one Markdown file per journal.
- **Checked self-update.** `jot update` downloads the release asset and
  verifies it against `SHA256SUMS` before replacing the binary.
- **Selective uninstall.** Remove journal data, the binary, the config file,
  or all three. Data removal is explicit and confirmed.
- **Plain storage and tight permissions.** Journal directories are `0700`;
  day files and the config file are `0600`.
- **Scriptable CLI.** `add`, `show`, `edit`, `ls`, `search`, `watch`, `todo`,
  `remind`, `backup`, `export`, `journal`, `init`, `setup`, `install`,
  `update`, `uninstall`, `doctor`, `version`, and `help`.

## Install

### Release script

```sh
curl -fsSL https://raw.githubusercontent.com/moneytosms/jot/main/install-release.sh | sh
```

The script downloads the Linux or macOS release for x86-64 or ARM64,
verifies it against `SHA256SUMS`, and installs it to `~/.local/bin`. Set
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
                       | ## 09:41:07                          |
                       | shipped the parser rewrite           |
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
[docs/tui.md](docs/tui.md) for the complete reference.

## Everyday use

```sh
jot add "shipped the parser rewrite"   # append to today
jot show --calendar                    # month grid with entry density
jot show -d yesterday                  # yesterday's entries
jot search --fuzzy parser              # scored search across journals
jot todo                               # checkbox items, read-only
jot remind "stretch" 20m               # transient countdown reminder
jot journal new ideas                  # create a journal
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
    2026-11-25.md
  backups/
    jot-backup-20261125-150405.tar.gz
```

jot only appends to day files; the format is the plain Markdown shown above,
so you can still edit it by hand.

Journal directories are created with mode `0700`; day files and the config
file with mode `0600`. `jot backup` writes a `.tar.gz` under `backups/`.
`jot journal rm` archives a journal before deleting it.

The config file is `$JOT_CONFIG` when set, otherwise
`~/.config/jot/config.json` on Linux (the user config directory on other
systems). Every key is optional:

```json
{
  "root": "~/.jot",
  "default_journal": "default",
  "editor": "nvim",
  "tui": {
    "theme": "auto",
    "journal": "work"
  }
}
```

| Key | Meaning |
| --- | --- |
| `root` | Journal root directory. `~` is expanded. |
| `default_journal` | Journal used when a command names none. |
| `editor` | Editor command for `jot edit` and the TUI. |
| `tui.theme` | TUI theme name. |
| `tui.journal` | Journal last selected inside the TUI; a convenience only. |

Environment variables:

| Variable | Meaning |
| --- | --- |
| `JOT_ROOT` | Override the journal root. |
| `JOT_CONFIG` | Use an alternative config file. |
| `JOT_EDITOR` | Editor command, above the config file. |
| `JOT_THEME` | TUI theme, above `tui.theme` and below `--theme`. |

CLI precedence is flags, then `JOT_*` variables, then the config file, then
built-in defaults. The TUI theme follows the same shape: `--theme`, then
`JOT_THEME`, then `tui.theme`, then `auto`. `JOT_JOURNAL` sets the default
journal; `JOT_INSTALL_DIR` changes the install directory.

## Updating and uninstalling

`jot update` checks the latest release, shows the available version, verifies
the download against `SHA256SUMS`, and replaces the binary. Useful flags:

| Flag | Effect |
| --- | --- |
| `--check` | Report whether an update is available; install nothing. |
| `--json` | Print one machine-readable JSON object. |
| `--notes` | Print the full release notes. |
| `--version TAG` | Target a specific release tag. |
| `--yes` | Do not ask for confirmation. |
| `--source` | Rebuild from a local checkout instead of downloading. |

`--src DIR` selects the source checkout for `--source`; `--dir DIR` selects
the install directory.

`jot uninstall` removes only the components you name. With no component
flags and a terminal it prints an inventory and asks for `d`, `b`, `c`, `a`,
or `q`.

| Flag | Effect |
| --- | --- |
| `--data` | Remove the journal root, including journals and backups. |
| `--binary` | Remove the running jot executable. |
| `--config` | Remove the config file. |
| `--all` | Remove data, binary, and config. |
| `--dry-run` | Print the plan and remove nothing. |
| `--yes` | Skip confirmations. Without it, data removal requires typing `delete`. |

Data removal deletes the journal root and does not archive it at that point.
Run `jot backup` first if you want a copy. The removal order is config, data,
then the running binary.

## Compatibility and scope

- The CLI is the stable surface. Existing flags and output are preserved.
- The TUI is read-mostly. Quick add is the only journal write the TUI
  itself makes; edits, deletions, and checkbox toggles go through your editor.
- Journals are flat: no nested journals, tags, or database.
- There is no sync and no server. Your files are yours to move, copy, or
  version however you like.
- Go 1.24 or newer. Linux and macOS are the release targets; Windows is
  not a release target.

## Documentation

- [Usage guide](https://moneytosms.github.io/jot/guide/) — the repo's own
  docs, rendered and searchable
- [Command reference](docs/commands.md)
- [TUI guide](docs/tui.md)
- [Development](docs/development.md)

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

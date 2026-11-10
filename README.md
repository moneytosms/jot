# jot

<p align="center"><img src="assets/readme/hero.svg" alt="jot: a terminal journal with one plain Markdown file per day" width="100%"></p>

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go 1.22](https://img.shields.io/badge/Go-1.22-00ADD8?logo=go&logoColor=white)](go.mod)
[![stdlib only](https://img.shields.io/badge/dependencies-stdlib%20only-brightgreen)](go.mod)

**jot is a fast command-line journal that stores your notes as plain Markdown files you can read and edit with any tool.**

It keeps one terminal journal with one plain Markdown file per day, per journal.
Entries are timestamped and appended, never rewritten. There is no database, no
daemon and no proprietary format. jot is written in Go and uses only the standard
library.

```sh
$ jot add "shipped the parser rewrite"
added to default/2026-11-10.md

$ jot show
## 09:41:07

shipped the parser rewrite
```

## Contents

- [Features](#features)
- [Install](#install)
- [First run](#first-run)
- [Usage](#usage)
- [Configuration](#configuration)
- [Storage](#storage)
- [Platform support](#platform-support)
- [Development](#development)
- [Contributing](#contributing)
- [License](#license)

## Features

- **Plain Markdown storage.** Each day is a file at `<root>/<journal>/YYYY-MM-DD.md`.
- **Append-only writes.** New entries are appended to the day's file. Earlier entries are not rewritten.
- **Timestamped entries.** Each entry starts with a `## HH:MM:SS` heading in local time.
- **Named journals.** Keep separate journals with `-j`/`--journal`. Names are sanitised before they become directories.
- **Any day.** Use `today`, `yesterday`, `tomorrow` or `YYYY-MM-DD` with `-d`/`--date`, or as a positional argument to `show` and `edit`.
- **Flexible input.** Pass text as arguments, pipe it on stdin, or write it in your editor with `jot edit`.
- **Search.** `jot search` does a case-insensitive substring match and prints `journal/day.md:line: text`.
- **Live tail.** `jot watch` prints new entries as they are appended and follows the file across midnight.
- **Rendering.** `jot show --render` prints Markdown with a simple terminal layout.
- **Backup and export.** `jot backup` writes a `.tar.gz` of all journals. `jot export` writes one Markdown file per journal.
- **Guided setup and checks.** `jot setup` writes the configuration. `jot doctor` checks the installation.
- **Restrictive permissions.** Journal directories are created with mode `0700` and files with mode `0600`. Export and install directories are `0755`.

## Install

### Quick install (Linux and macOS)

```sh
curl -fsSL https://raw.githubusercontent.com/moneytosms/jot/main/install-release.sh | sh
```

Downloads the release binary for your OS and architecture, verifies it against `SHA256SUMS`, and installs it to `~/.local/bin` (override with `JOT_INSTALL_DIR`).

To pin a version, set `JOT_VERSION`, e.g. `JOT_VERSION=v0.1.0 sh`. Windows users: download `jot-windows-amd64.exe` from the [release page](https://github.com/moneytosms/jot/releases/tag/v0.1.0).

The options below build jot from source. You need **Go 1.22 or newer** ([download](https://go.dev/dl/)).

### Build from source: `install.sh`

From a checkout of this repository:

```sh
cd jot
./install.sh                        # installs to ~/.local/bin/jot
./install.sh --prefix /usr/local    # installs to /usr/local/bin/jot
./install.sh --bin-dir ~/bin        # installs to ~/bin/jot
```

The script builds with the version from `git describe` when available, copies the
binary into place, and prints the line to add to your shell configuration if the
install directory is not on `PATH`. Run `./install.sh --help` for all options.

### Alternative: `go install`

```sh
go install github.com/moneytosms/jot/cmd/jot@latest
```

The binary is placed in the Go bin directory (`go env GOBIN`, or
`$(go env GOPATH)/bin` when `GOBIN` is unset). That directory must be on `PATH`.

### Alternative: Make

```sh
cd jot
make build                          # writes bin/jot
make install                        # copies bin/jot to ~/.local/bin/jot
make install BINDIR="$HOME/bin"     # copies bin/jot to ~/bin/jot
```

### Verify

```sh
jot version
jot doctor
```

`jot doctor` reports `[ok]`, `[warn]` or `[fail]` for the root directory, its
permissions, the config file, journals, the editor and `PATH`. It exits non-zero
when a check fails. Warnings do not cause a failure.

## First run

```sh
jot setup          # interactive wizard
jot setup --yes    # accept all defaults without prompts
```

The wizard asks for the journal root (default `~/.jot`), the default journal
(default `default`) and the editor. It writes the config file, creates the root
and can install the binary. If you skip setup, jot works with `~/.jot`, the
`default` journal and your editor.

Add an entry and read it back:

```sh
jot add "first entry"
```

```text
added to default/2026-11-10.md
```

```sh
jot show
```

```text
## 09:41:07

first entry
```

## Usage

| Command | Purpose |
| --- | --- |
| `jot add [text...]` | Append a timestamped entry from arguments, stdin (`-` or piped input) |
| `jot show [day]` | Print entries for a day (default today), or every day with `-a` |
| `jot edit [day]` | Open a day's file in your editor, creating it if needed |
| `jot ls [journal]` | List journals with counts, or the days in one journal |
| `jot search <query>` | Case-insensitive search across journals, with file and line numbers |
| `jot watch` | Follow today's entries live until Ctrl+C |
| `jot backup [dir]` | Write a `.tar.gz` of every journal (default `<root>/backups`) |
| `jot export [dir]` | Write one Markdown file per journal (default `./jot-export`) |
| `jot init` | Create the journal root and the default journal |
| `jot setup` | First-run wizard for config, root and optional binary install |
| `jot install [dir]` | Copy the running binary into a bin directory |
| `jot update` | Rebuild and reinstall from a local source checkout |
| `jot uninstall` | Delete all journal data. The binary is kept |
| `jot doctor` | Check the installation |
| `jot version` | Print the version |
| `jot help [command]` | Show overview help or the help page for one command |

Common flags:

| Flag | Meaning |
| --- | --- |
| `-j, --journal <name>` | Journal to act on |
| `-d, --date <day>` | `today`, `yesterday`, `tomorrow` or `YYYY-MM-DD` |

Each command has a help page, for example `jot help backup`. The full reference
is in [docs/commands.md](docs/commands.md). Workflows such as editor setup,
sync, restore and migration are in [docs/guide.md](docs/guide.md).

Backups and exports:

```sh
jot backup ~/Dropbox/jot-backups                     # archive every journal
tar -tzf ~/.jot/backups/jot-backup-*.tar.gz          # list an archive
tar -xzf ~/.jot/backups/jot-backup-20261105-150405.tar.gz -C ~/.jot   # restore
jot export --out ~/Documents/journal-export          # one .md per journal
```

## Configuration

Configuration is optional. Without any configuration jot uses `~/.jot`, the
`default` journal and your editor.

Precedence, highest first: command-line flags, `JOT_*` environment variables,
the config file, built-in defaults.

### Config file

`jot setup` writes this file. You can also edit it by hand:

```json
{
  "root": "~/.jot",
  "default_journal": "default",
  "editor": "nvim"
}
```

| Key | Meaning |
| --- | --- |
| `root` | Journal root directory. `~` is expanded |
| `default_journal` | Journal used when `-j` is not given |
| `editor` | Editor command for `jot edit`. Split on whitespace |

The config file location is:

- `$JOT_CONFIG`, if set.
- Otherwise `<user config dir>/jot/config.json`. On Linux the user config dir is
  `$XDG_CONFIG_HOME` or `~/.config`, so the default is `~/.config/jot/config.json`.
  On macOS it is `~/Library/Application Support/jot/config.json`.

A malformed config file is reported by `jot doctor` and by commands that read it.

### Environment variables

| Variable | Effect |
| --- | --- |
| `JOT_ROOT` | Journal root directory (overrides the config file) |
| `JOT_JOURNAL` | Default journal name (overrides the config file) |
| `JOT_EDITOR` | Editor for `jot edit` (overrides the config file) |
| `JOT_CONFIG` | Path to an alternative config file |
| `JOT_INSTALL_DIR` | Default directory for `jot install` (default `~/.local/bin`) |
| `JOT_SRC` | Source checkout used by `jot update` |
| `VISUAL`, `EDITOR` | Editor fallbacks used by `jot edit` |

The editor for `jot edit` is chosen in this order: `JOT_EDITOR`, the `editor`
config key, `$VISUAL`, `$EDITOR`, then the first of `vim`, `vi` or `nano` found
on `PATH`. Terminal editors are recommended. `jot edit` waits for the editor to
exit, so GUI editors need a blocking flag such as `code --wait`.

## Storage

```text
~/.jot/                                   root (JOT_ROOT or config "root")
├── default/
│   ├── 2026-11-04.md
│   └── 2026-11-05.md
├── work/
│   └── 2026-11-05.md
└── backups/
    └── jot-backup-20261105-150405.tar.gz
```

| Path | Contents |
| --- | --- |
| `<root>/<journal>/YYYY-MM-DD.md` | Entries for one day, oldest first |
| `<root>/backups/jot-backup-YYYYMMDD-HHMMSS.tar.gz` | Archive from `jot backup`. Never included in archives or listed as a journal |
| `./jot-export/<journal>.md` | Output of `jot export` (or the directory given to `--out`) |
| `~/.config/jot/config.json` | Config file (see [Config file](#config-file) for other platforms) |

Journal names are sanitised: surrounding whitespace is trimmed, ASCII letters,
digits, `-` and `_` are kept, and every other character becomes `-`. Repeated
dashes collapse, leading and trailing dashes are removed, and an empty result
becomes `default`. `backups` is a reserved name and is rejected as a journal name
with an error. Day files are named by local date.

Journal directories are created with mode `0700` and files with mode `0600`.
Export and install directories are `0755`. The root is
plain files, so backing up or syncing the root captures all journal data. The
config file is the only data kept outside the root.

## Platform support

Linux and macOS. The config path rules above cover both, and the release build
targets (see [docs/development.md](docs/development.md)) include both.

Windows: builds cross-compile but is untested.

## Development

```sh
make check     # go vet ./... and go test ./...
make build     # bin/jot
make fmt       # gofmt -w .
make clean     # remove build output
```

Project layout, test notes, the storage format and release builds are described
in [docs/development.md](docs/development.md).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

jot is released under the [MIT License](LICENSE).

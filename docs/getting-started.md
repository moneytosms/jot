# Getting started

Install jot, write the first entries, then keep it updated or remove it.

## Install and first entries

Install the latest release (Linux, macOS; it verifies the checksum and puts
`jot` in `~/.local/bin`):

```bash
curl -fsSL https://raw.githubusercontent.com/moneytosms/jot/main/install-release.sh | sh
```

With Go instead: `go install github.com/moneytosms/jot/cmd/jot@latest`, or
`./install.sh` from a checkout. Then:

```bash
jot setup --yes                 # defaults: root ~/.jot, journal "default"
jot add "first entry"
jot add -j work "standup: parser is done"
jot show
jot search parser
jot doctor
```

If `jot` is not found afterwards, the installer prints the line that adds
`~/.local/bin` to `PATH`. Add it to your shell config and open a new shell.

Run `jot` with no command in a terminal to open the TUI; see
[The TUI](tui.md). Next: [Writing entries](writing.md).

## Update

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

## Remove jot

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

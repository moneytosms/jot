# Development

How jot is put together, how to run it, and how to change it without breaking
the file format.

## Layout

```
jot/
├── cmd/jot/
│   ├── main.go             dispatch, flag helpers, bare-jot TUI launch
│   ├── help.go             overview and per-command help text
│   ├── add.go              add
│   ├── show.go             show, including --render and --calendar
│   ├── edit.go             edit, init, ls
│   ├── search.go           search, backup, export
│   ├── todo.go             todo
│   ├── remind.go           remind
│   ├── journal.go          journal new/rm/rename/ls
│   ├── watch.go            watch, install, update, uninstall
│   ├── doctor.go           doctor, setup
│   ├── cli_test.go         end-to-end tests that call run()
│   ├── cli_ext_test.go     tests for the newer commands
│   ├── tui_wiring_test.go  bare-jot and `jot tui` launch behaviour
│   └── update_uninstall_test.go
├── internal/tui/
│   ├── api.go              Options and Run (the only entry point)
│   ├── app.go              root model, modes, key routing, disk commands
│   ├── view.go             layout() and the render tree
│   ├── calendar.go         pure month grid and entry heat
│   ├── theme.go            themes, Omarchy parsing, markdown styles
│   ├── input.go            single-line editor used by every prompt
│   ├── overlay.go          overlay lifecycle and panel drawing
│   ├── search.go           fuzzy search overlay
│   ├── palette.go          command palette
│   ├── todos.go            todo overlay
│   ├── reminder.go         reminder form
│   ├── settings.go         theme, editor and default-journal settings
│   ├── help.go             keybinding cheatsheet
│   └── *_test.go           headless model tests
├── calendar.go             month counts for the calendar views
├── config.go               config file, env vars, precedence
├── entries.go              entry parsing (time, body, line number)
├── exec.go                 $EDITOR resolution and launching
├── fuzzy.go                fuzzy matching and scoring
├── journal_ops.go          journal create/archive/delete/rename
├── jot.go                  core: Jot handle, journals, entries, listing, search
├── ops.go                  watch, backup, export, install, doctor
├── release.go              release lookup, checksums, self-update
├── reminder.go             reminders through omarchy/systemd-run
├── render.go               markdown to styled terminal text
├── search_entries.go       cross-journal entry search
├── setup.go                first-run wizard
├── todo.go                 inline checkbox extraction
├── utils.go                day parsing, name sanitising, atomic write
├── go.mod, go.sum
├── vendor/                 committed dependencies (bubbletea, lipgloss)
├── install.sh              source installer
├── install-release.sh      release binary installer
├── Makefile                build/test/install/vendor targets
├── README.md
├── .gitignore
└── docs/
    ├── README.md           guide home: what to read, in order
    ├── getting-started.md  install, first entries, update, remove
    ├── writing.md          adding entries, todos, journals, reminders, editors
    ├── finding.md          reading, search, tags, other tools
    ├── tui.md              the interactive interface
    ├── your-data.md        backup, export, import, encryption, sync
    ├── configuration.md    config file, environment, permissions
    ├── troubleshooting.md  symptoms and fixes
    ├── commands.md         command reference
    └── development.md      this file
```

The module is `github.com/moneytosms/jot`. The CLI lives in `cmd/jot`, so
`go install ./cmd/jot` produces a binary named `jot`.

Dependencies are committed under `vendor/`: Bubble Tea for the terminal loop
and Lip Gloss for colour and layout. Everything else is the standard library.
`go build` and `go test` use `vendor/` automatically, so the repository builds
offline; `make vendor` refreshes the tree after changing `go.mod`.

## The three layers

**`package jot`** owns everything that touches disk: path resolution, sanitising,
appending, reading, searching, archives, installs, release downloads and health
checks. It never writes to `os.Stdout` directly and never calls `os.Exit`.
Output goes to writers passed in, and methods return values or errors.

**`cmd/jot`** owns presentation: flag parsing, table output, prompts, signals
and exit codes. Handlers are methods on `app`, which carries `in`, `out` and
`errw`, so a test can exercise a command without a real terminal:

```go
code := run([]string{"add", "hello"}, strings.NewReader(""), &out, &err)
```

That split is the main reason `cmd/jot/cli_test.go` can cover the whole CLI
without shelling out.

**`internal/tui`** is a pure Bubble Tea model over the library. `Update` never
touches disk: it returns `tea.Cmd`s (`loadCmd`, `dayCmd`, `addCmd`,
`editorCmd`, search and preference saves) that call the library and come back
as messages. `View` is a pure function of the model, and `layout()` is shared
by rendering and mouse hit-testing. `api.go` is the whole public surface:
`Options{Cfg, Journal, Theme, In, Out}` and `Run`, which blocks until quit.

## Storage format

```
<root>/<journal>/YYYY-MM-DD.md
```

```markdown
## 09:14:32

standup: parser shipped, docs next

## 17:02:10

retro notes
```

Invariants worth preserving:

- One file per day, named `YYYY-MM-DD.md`, in a directory named after the
  journal.
- An entry starts with `## HH:MM:SS`; the heading is followed by a blank line and
  the body.
- `jot add` only appends. It adds one separating blank line when the file is not
  empty, and never rewrites existing bytes.
- Journal files are created `0600` and journal directories `0700`. Exports are
  `0644`.
- `<root>/backups/` holds archives; it is skipped by `List` and by `backup`.
- Journal names go through `SanitizeName` (surrounding whitespace is trimmed;
  ASCII letters, digits, `-` and `_` survive; other characters become `-`;
  repeated dashes collapse; leading and trailing dashes are removed; `default`
  when empty) before they are joined to paths. `backups` is reserved and is
  rejected as a journal name.

Reading is deliberately tolerant: `jot show` prints whatever is in the file, so
hand-edited markdown is fine. Entry counts and search treat `## ` lines as
headings, and blank lines are skipped by search.

The TUI follows the same rule: it appends quick notes and hands everything else
(edits, deletions, checkbox toggles) to `$EDITOR`. It never rewrites a day file
itself.

## Command flow

1. `main` calls `run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)`.
2. `run` handles `help`/`version`, then opens the TUI for bare `jot` when both
   streams are terminals, and dispatches every other argument to
   `app.<command>Cmd`.
3. Each command builds a `flag.FlagSet` via `a.flags(name)`, which discards Go's
   default usage output (jot prints its own help page from `help.go`).
4. `a.loadJot()` resolves config (`DefaultConfig`) and returns a `*jot.Jot`.
5. The command calls the library, formats output to `a.out`, and returns an
   error.
6. `run` prints `jot: <error>` to stderr and returns exit code `1` for any error,
   or `0`. `flag.ErrHelp` is treated as success; an unknown flag returns the
   usage sentinel so the help page exits `2` without a `jot:` prefix.

Shared flags are built by `journalFlag` and `dateFlag`, which bind both the long
name and the single-letter shorthand to the same variable. Because both flags
share a target, `-j work` and `--journal work` are equivalent and the last one
wins.

## Adding a command

1. Write the handler as a method on `app` in `cmd/jot/<name>.go`, or in an
   existing file whose group it fits.
2. Add a `case "<name>":` in `run`.
3. Add an entry to `commandHelp` in `help.go`, plus a line in `printHelp`.
4. Put behaviour in the `jot` package if it touches disk, so it can be tested
   directly; keep the CLI file about flags and formatting.
5. Add a CLI test in `cli_test.go` or `cli_ext_test.go` and, if you added
   library behaviour, a unit test next to it.
6. Run `make check` and `gofmt -w .`.
7. Add the command to the command tables in `README.md` and `docs/commands.md`,
   and to the matching task page in `docs/` if a task needs it. New guide pages
   also go in the `nav` list in `site/site.config.mjs`.

Things to keep consistent:

- Dates go through `ParseDay`, journal names through `SanitizeName` (or
  `Jot.journalName`).
- Missing journals are `fmt.Errorf("%w: %s", ErrNoJournal, name)`, so the CLI can
  turn them into a friendly hint.
- Destructive operations must be confirmable and must refuse dangerous paths.
- New files use `writeFileAtomic` when a partial write would be harmful.
- `jot update` verifies `SHA256SUMS` and renames into place; never leave a
  partial binary behind when a download or checksum fails.

## Adding a TUI screen

Each overlay lives in its own file with four pieces: a state struct on the
model, a key handler called from the mode switch in `Update.handleKey`, a
render function called from `View`, and tests that feed synthetic key messages
to `model.Update`. Rules:

- `Update` returns `tea.Cmd`s for anything that touches disk or the clock.
- Clamp every list height and width through the helpers in `view.go`; the
  interface must not panic at 0x0.
- Use the `Theme` accessors (`accent()`, `muted()`, `dim()`, `border()`, ...)
  instead of literal colours.
- Overlays are exclusive: opening one closes the others, `esc` always returns
  to browse mode, and `ctrl+c` quits from everywhere.
- Update the cheatsheet in `internal/tui/help.go` and the keybinding reference
  in `docs/tui.md` in the same commit.

## Tests

```bash
make test      # go test ./...
make vet       # go vet ./...
make check     # both
```

Coverage is organised as:

- `jot_test.go`: config precedence and config file handling, day parsing, name
  sanitising, rendering, add/show/list/search/export/backup/install/uninstall,
  doctor, setup, watch (with a cancellable context and a locked buffer).
- One file per newer library area: `entries_test.go`, `fuzzy_test.go`,
  `search_entries_test.go`, `todo_test.go`, `calendar_test.go`,
  `reminder_test.go`, `render_test.go`, `journal_ops_test.go`, `release_test.go`
  and `config_tui_test.go`.
- `cmd/jot/cli_test.go` and `cli_ext_test.go`: dispatch, help, add (arguments,
  stdin, dates), show and its filters, ls, search scoping, edit file creation,
  export/backup, install, uninstall confirmation, doctor, setup, error paths and
  exit codes.
- `cmd/jot/tui_wiring_test.go`: bare `jot` help fallback and non-terminal
  `jot tui` refusal.
- `cmd/jot/update_uninstall_test.go`: update status/install and the selective
  uninstall paths.
- `internal/tui/*_test.go`: headless model tests for layout, themes, calendar
  math, every overlay, and the preference saves.

Tests use `t.TempDir()` plus `JOT_ROOT`/`JOT_CONFIG` overrides, so they never
read or write your real journals. The CLI tests set `JOT_EDITOR=true`, which
makes `edit` a no-op. Network tests use `httptest` and `JOT_RELEASES_API`;
nothing in the suite contacts GitHub.

The `watch` test relies on time: it appends, polls for the text with a deadline,
then cancels the context and asserts the goroutine exits. Keep that shape if you
change the watcher.

## Building and versioning

```bash
go build ./cmd/jot
go build -ldflags "-X main.version=v1.0.0" -o bin/jot ./cmd/jot
make build        # bin/jot; version from git describe, falls back to 0.0.0-dev
make install      # copies to ~/.local/bin; override with BINDIR=
make vendor       # go mod tidy + go mod vendor after dependency changes
```

`main.version` is a plain string in `cmd/jot/main.go`, default `0.0.0-dev`.
`install.sh` computes it from `git describe --tags --always --dirty` when the
checkout is a git repository and passes it the same way. `jot version` prints it.

Because `vendor/` is committed, an offline build is:

```bash
GOFLAGS=-mod=vendor GOPROXY=off go build ./...
```

Cross-compilation needs only the Go toolchain, since nothing uses cgo:

```bash
mkdir -p dist
for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64; do
  GOOS=${target%/*} GOARCH=${target#*/} go build -trimpath \
    -ldflags "-s -w -X main.version=v1.0.0" -o "dist/jot-${target%/*}-${target#*/}" ./cmd/jot
done
```

## Releases and self-update

A release is a git tag `vX.Y.Z` plus these assets:

- `jot-linux-amd64`, `jot-linux-arm64`, `jot-darwin-amd64`, `jot-darwin-arm64`
- `SHA256SUMS`, one `<sha256>  jot-<os>-<arch>` line per binary

`install-release.sh` downloads one of these and verifies it. `jot update` does
the same thing from inside the binary: it reads
`https://api.github.com/repos/moneytosms/jot/releases/latest`, compares the tag
with `main.version`, prints the release notes, downloads the asset for
`runtime.GOOS`/`runtime.GOARCH`, verifies the checksum and atomically replaces
the running executable's directory entry. `--check` only reports.
`JOT_RELEASES_API` overrides the API base (mirrors, tests). `jot update
--source` keeps the old rebuild-from-checkout path.

The Windows target is a cross-compile output only. It is built by the loop
above but is not tested on Windows, so treat it as unverified.

## Concurrency and failure behaviour

- Entries are appended with `O_APPEND`; nothing rewrites existing content.
  There is no lock file, so two shells writing the same day at the same instant
  rely on normal append semantics rather than coordination.
- A crash mid-entry loses at most the entry being written.
- Config and other whole-file writes use a temp file plus rename.
- Backups stream through gzip/tar with explicit closes on every path; a failed
  walk removes the partial archive.
- The watcher tolerates truncation and day rollover and is read-only.
- `jot update` downloads to a temp file in the destination directory, fsyncs,
  verifies, then renames; a failed or corrupted download leaves the existing
  binary untouched.

## Ideas deliberately left out

These are not implemented. They are the likely next steps if the scope grows:

- optional encryption or a `jot lock` command
- regex or tag-aware search, and `--json` output for every command
- shell completion generation
- a `jot import` for common journal exports
- sync, or any daemon
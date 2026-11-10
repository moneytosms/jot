# Development

How jot is put together, how to run it, and how to change it without breaking
the file format.

## Layout

```
jot/
├── cmd/jot/
│   ├── main.go      dispatch, flag helpers, shared output helpers
│   ├── help.go      overview and per-command help text
│   ├── add.go       add
│   ├── show.go      show
│   ├── edit.go      edit, init, ls
│   ├── search.go    search, backup, export
│   ├── watch.go     watch, install, update, uninstall
│   ├── doctor.go    doctor, setup
│   └── cli_test.go  end-to-end tests that call run()
├── config.go        config file, env vars, precedence
├── jot.go           core: Jot handle, journals, entries, listing, search
├── ops.go           edit, watch, backup, export, install, update, doctor
├── setup.go         first-run wizard
├── utils.go         day parsing, name sanitising, rendering, atomic write
├── jot_test.go      library tests
├── go.mod
├── install.sh       source installer
├── Makefile         build/test/install targets
├── README.md
├── .gitignore
└── docs/
    ├── commands.md  command reference
    ├── guide.md     task-based usage guide
    └── development.md
```

The module is `github.com/moneytosms/jot`. The CLI lives in `cmd/jot`, so
`go install ./cmd/jot` produces a binary named `jot`. Requires Go 1.22 or newer
(`go.mod`).

There are no external dependencies: `go.mod` lists none, and `go build` works
offline.

## The two layers

**`package jot`** owns everything that touches disk: path resolution, sanitising,
appending, reading, searching, archives, installs and health checks. It never
writes to `os.Stdout` directly and never calls `os.Exit`. Output goes to writers
passed in, and methods return values or errors.

**`cmd/jot`** owns presentation: flag parsing, table output, prompts, signals
and exit codes. Handlers are methods on `app`, which carries `in`, `out` and
`errw`, so a test can exercise a command without a real terminal:

```go
code := run([]string{"add", "hello"}, strings.NewReader(""), &out, &err)
```

That split is the main reason `cmd/jot/cli_test.go` can cover the whole CLI
without shelling out.

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

## Command flow

1. `main` calls `run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)`.
2. `run` handles `help`/`version`, then dispatches to `app.<command>Cmd`.
3. Each command builds a `flag.FlagSet` via `a.flags(name)`, which discards Go's
   default usage output (jot prints its own help page from `help.go`).
4. `a.loadJot()` resolves config (`DefaultConfig`) and returns a `*jot.Jot`.
5. The command calls the library, formats output to `a.out`, and returns an
   error.
6. `run` prints `jot: <error>` to stderr and returns exit code `1` for any error,
   or `0`. `flag.ErrHelp` is treated as success.

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
5. Add a CLI test in `cli_test.go` and, if you added library behaviour, a unit
   test next to it.
6. Run `make check` and `gofmt -w .`.
7. Add the command to the command tables in `README.md` and `docs/commands.md`,
   and to `docs/guide.md` if a task needs it.

Things to keep consistent:

- Dates go through `ParseDay`, journal names through `SanitizeName` (or
  `Jot.journalName`).
- Missing journals are `fmt.Errorf("%w: %s", ErrNoJournal, name)`, so the CLI can
  turn them into a friendly hint.
- Destructive operations must be confirmable and must refuse dangerous paths.
- New files use `writeFileAtomic` when a partial write would be harmful.

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
- `cmd/jot/cli_test.go`: dispatch, help, add (arguments, stdin, dates), show and
  its filters, ls, search scoping, edit file creation, export/backup, install,
  uninstall confirmation, doctor, setup, error paths and exit codes.

Tests use `t.TempDir()` plus `JOT_ROOT`/`JOT_CONFIG` overrides, so they never
read or write your real journals. The CLI tests set `JOT_EDITOR=true`, which
makes `edit` a no-op.

The `watch` test relies on time: it appends, polls for the text with a deadline,
then cancels the context and asserts the goroutine exits. Keep that shape if you
change the watcher.

## Building and versioning

```bash
go build ./cmd/jot
go build -ldflags "-X main.version=v1.0.0" -o bin/jot ./cmd/jot
make build        # bin/jot; version from git describe, falls back to 0.0.0-dev
make install      # copies to ~/.local/bin; override with BINDIR=
```

`main.version` is a plain string in `cmd/jot/main.go`, default `0.0.0-dev`.
`install.sh` computes it from `git describe --tags --always --dirty` when the
checkout is a git repository and passes it the same way.

Cross-compilation needs only the Go toolchain, since the module has no cgo
dependencies:

```bash
mkdir -p dist
for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64; do
  GOOS=${target%/*} GOARCH=${target#*/} go build -trimpath \
    -ldflags "-s -w -X main.version=v1.0.0" -o "dist/jot-${target%/*}-${target#*/}" ./cmd/jot
done
```

The Windows target is a cross-compile output only. It is built by the loop
above but is not tested on Windows, so treat it as unverified.

If prebuilt releases are added later, `jot update` is the place to teach about
them. Today it rebuilds from a local checkout and prints the new binary's
version as proof.

## Concurrency and failure behaviour

- Entries are appended with `O_APPEND`; nothing rewrites existing content.
  There is no lock file, so two shells writing the same day at the same instant
  rely on normal append semantics rather than coordination.
- A crash mid-entry loses at most the entry being written.
- Config and other whole-file writes use a temp file plus rename.
- Backups stream through gzip/tar with explicit closes on every path; a failed
  walk removes the partial archive.
- The watcher tolerates truncation and day rollover and is read-only.

## Ideas deliberately left out

These are not implemented. They are the likely next steps if the scope grows:

- optional encryption or a `jot lock` command
- regex or tag-aware search, and `--json` output
- shell completion generation
- a `jot import` for common journal exports
- release downloads behind `jot update`

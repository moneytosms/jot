# Your data

Back up, export, import, encrypt and sync. The journal root is a folder of markdown files, so every one of these works on plain files.

## Back up

```bash
jot backup                              # <root>/backups/jot-backup-<timestamp>.tar.gz
jot backup /mnt/usb/journal-backups     # somewhere else
jot backup --zip                        # a .zip instead of .tar.gz
```

Each archive holds every journal under the root, stored as paths such as
`work/2026-11-05.md`, with templates, encryption markers and the config file
(as `.jot/config.json`). The default `backups` directory is skipped. The age
key is left out unless you pass `--with-key`; anyone holding such an archive
can read your encrypted journals. The TUI has the same action under
Settings, Storage.

Restore journals from an archive (journals that already exist are refused), or
unpack by hand:

```bash
jot journal archives    # list saved archives
jot journal restore ~/.jot/backups/jot-backup-20261105-150405.tar.gz
tar -xzf ~/.jot/backups/jot-backup-20261105-150405.tar.gz -C ~/.jot
```

To run backups nightly, add a cron entry. Cron does not load your shell
profile, so use absolute paths:

```cron
15 3 * * * $HOME/.local/bin/jot backup $HOME/backups >> $HOME/.jot-backup.log 2>&1
```

A backup only helps if it leaves the machine. Copy archives to another disk or
host regularly.

## Export

```bash
jot export --out ~/Documents/journal-export
```

This writes one `<journal>.md` file per journal. Each file has a `# <journal>`
heading, a `## YYYY-MM-DD` heading per day, and the day's entries below it.
Exported files are created with mode `0644` (subject to your umask). Export
includes every entry in every journal. To share a single journal, use
`jot show -j reading -a > reading.md` instead.

For something to read or print, export HTML or PDF: a cover page with the date
range and counts, a contents page by month, then every day rendered. PDF is
printed by a headless Chrome, Chromium, Edge or Brave.

```bash
jot export --format html
jot export --format pdf -j work --from 2026-01-01 --to 2026-03-31
```

To share one day, or just a few notes from it, pass `-d`. A single day has no
cover page, just the date and its entries; `--entry` keeps only the entries at
the times you name. The TUI palette has "Export this day as HTML / PDF" for
the selected day.

```bash
jot export --format pdf -j work -d yesterday
jot export --format html -d 2026-11-25 --entry 09:41,14:30
```

## Encrypt a journal

```bash
jot journal encrypt private     # age-encrypt every day file; new writes stay encrypted
jot journal decrypt private     # back to plain markdown
```

Files keep their `.md` names; their content becomes armored
[age](https://age-encryption.org) ciphertext. jot reads and searches them as
usual. The key is created on first use at `age-key.txt` next to the config
file (`JOT_KEY` overrides). Back it up separately: without it an encrypted
journal cannot be read. In the TUI, press `x` twice on a journal row in
Settings.

## Import from other tools

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

jot only appends to journal files and never rewrites them, so diffs stay
small. If you also edit files by hand on two machines, resolve conflicts as
you would for any markdown file.

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

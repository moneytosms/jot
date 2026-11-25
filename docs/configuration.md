# Configuration

The config file, environment variables, and the permissions jot gives the files it writes.

## Config file

The config file is `jot/config.json` in the user config directory: on Linux,
`~/.config/jot/config.json`; on macOS, `~/Library/Application Support/jot/config.json`.
Set `JOT_CONFIG` to use another path.

```json
{
  "root": "~/.jot",
  "default_journal": "daily",
  "editor": "code --wait",
  "reminders": "auto",
  "git": "commit",
  "tui": {
    "theme": "nord",
    "hide": ["reminders"],
    "colors": { "work": "cyan" }
  }
}
```

Every key is optional. A missing file means the defaults apply. `~` is expanded
in `root`. `reminders` picks how reminders are delivered: `auto` (the default)
tries Omarchy, then a systemd timer, then a background process with the
system's notifier; `omarchy`, `systemd` or `background` pins one; `off` turns
reminders off. See [`jot remind`](commands.md#remind). `git` turns on git
sync of the root: `off` (default), `commit` after every write (the root becomes
a git repository on first use, with `backups/` ignored), or `push` to also push
when a remote is set; a failed sync is a warning, never a lost write. The `tui`
block holds interface preferences: `theme`, the journal last selected with
`J`/`K`, `hide` (below-calendar widgets turned off: `onthisday`, `todos`,
`reminders`) and `colors` (a sidebar colour per journal). `jot setup` writes this file from its
answers, replacing the previous contents; it does not preserve the `tui` block.

## Environment variables

| Variable | Meaning |
| --- | --- |
| `JOT_ROOT` | Journal root (default `~/.jot`) |
| `JOT_JOURNAL` | Default journal name |
| `JOT_EDITOR` | Editor command for `jot edit` |
| `JOT_CONFIG` | Path of the config file |
| `JOT_KEY` | age key file for encrypted journals (default `age-key.txt` next to the config) |
| `JOT_INSTALL_DIR` | Default directory for `jot install`, `jot update` and `jot setup` |
| `JOT_SRC` | Source checkout for `jot update --source` |
| `JOT_THEME` | TUI colour theme, above `tui.theme` and below `--theme`; `show --render` uses it for light/dark |
| `JOT_RELEASES_API` | Full release API base URL for `jot update`, including `/releases` |

For the command-line settings, precedence highest first: flags, `JOT_*`
variables, the config file, built-in defaults. The TUI theme follows the same
order: `--theme`, then `JOT_THEME`, then `tui.theme` in the config file, then
`auto`.

To change the default journal for one shell session:

```bash
export JOT_JOURNAL=daily
```

## Files and permissions

- Journal files are created `0600` and journal directories `0700`, so only the
  owner can read them.
- The config file is written with mode `0600`.
- Exports are created with mode `0644` (subject to your umask). Backup
  archives get the default mode for new files (`0644` with a common umask);
  move them somewhere private if that matters.
- `jot update` contacts the GitHub releases API (or the mirror in
  `JOT_RELEASES_API`) and downloads the release binary; reminders call your
  existing notification tools. There is no account, telemetry or background
  daemon.
- Journals are plain text unless you encrypt them with
  [`jot journal encrypt`](your-data.md#encrypt-a-journal). Full-disk encryption
  (LUKS, FileVault, BitLocker) still protects everything else, including
  backups. Deleting
  a file does not guarantee its contents are gone from an SSD.

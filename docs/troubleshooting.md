# Troubleshooting

Run `jot doctor` first. It checks the root, config file, journals, editor,
`PATH`, the reminder provider and optional helper binaries.

| Symptom | Cause and fix |
| --- | --- |
| ``nothing to add: pass text, pipe it in, or use `jot edit` `` | No text was given and stdin is a terminal. Pass the text as arguments or pipe it in. |
| `entry body is empty` | The text was blank and was piped in. Nothing was written. At a terminal, `jot add ""` prints `nothing to add` instead. |
| `no journal named "x" yet` | The journal does not exist. Check `jot ls`, then create it with `jot journal new x`, `jot add -j x "..."` or `jot init -j x`. |
| `invalid date "..."` | Use `today`, `yesterday`, `tomorrow`, or `YYYY-MM-DD`. |
| Entry is on the wrong day | Entries use local time. Check `date`, and use `-d` to pick the day. |
| Editor does not block | Use `--wait` (or the equivalent flag) for GUI editors. |
| `no editor found: set JOT_EDITOR or $EDITOR` | Set `JOT_EDITOR`, or install `vim`, `vi` or `nano`. |
| `jot` not found after install | Add the bin directory to `PATH`. `jot install` prints the line. |
| `no matches for "..."` | Plain search is a line-based substring match. Try a shorter query, or `jot search --fuzzy "..."` for entry-level matching. |
| A flag is treated as part of the query or name | Flags go before arguments: `jot search --fuzzy parser`, `jot journal rm --yes notes`. |
| `invalid month "..."` | `show --calendar` takes `YYYY-MM`, or no value for the current month. |
| `jot: config file <path>: ...` on commands that read config | The config file is not valid JSON. Fix or remove it. Every command that reads config stops at this error, `jot doctor` included. `jot version` and `jot help` still run. |
| Backup is missing a journal | Only `.md` files are archived. Check that the files end in `.md`. |
| `refusing to delete journal data without confirmation; re-run with --yes` | `jot uninstall --data` ran without a terminal. Pass `--yes` only if you mean it. |
| `refusing to remove anything without an explicit component` | `jot uninstall` had no terminal to show its menu. Pass `--data`, `--binary`, `--config` or `--all`; `--yes` alone means data only. |
| `no reminder provider: ...` | Run `jot doctor` to see what was found. Install a notifier (`notify-send` on Linux), or set `reminders` in the config file. |
| `jot tui needs an interactive terminal` | Run `jot tui`, or bare `jot`, in a real terminal rather than through a pipe. |
| `jot update` cannot reach the release API | Check the network, or point `JOT_RELEASES_API` at a mirror. |
| Empty day files (`0 entries`) | `jot edit` was quit without writing. Delete them with `find ~/.jot -name '*.md' -size 0 -delete`. |

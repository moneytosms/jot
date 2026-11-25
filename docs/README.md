# jot guide

jot is a plaintext journal for the terminal: one Markdown file per day, a CLI
to add and search, and a TUI with a calendar. These pages go from installing
it to everything it does. Read them in order the first time, or jump to the
task you have.

## Start here

1. [Getting started](getting-started.md): install, first entries, update, remove.

## Using jot

2. [Writing entries](writing.md): one-liners, piped output, longer notes,
   todos, backdating, separate journals, reminders, editors.
3. [Finding and reviewing](finding.md): read a day, search by text, `#tag` or
   date, find a day without knowing its date.
4. [The TUI](tui.md): the calendar, search, todos, settings, themes and every
   key.
5. [Your data](your-data.md): backups, HTML/PDF export, import, age
   encryption, git sync.
6. [Configuration](configuration.md): the config file, environment variables,
   file permissions.

## Reference

- [Command reference](commands.md): every command and flag.
- [Troubleshooting](troubleshooting.md): error messages and their fixes.

## Contributing

- [Development](development.md): building, testing and the code layout.
- [Contributing](../CONTRIBUTING.md): how to send a change.

## Five commands to know

```bash
jot add "shipped the parser #work"   # write a line
jot                                  # open the TUI
jot search "#work after:7d"         # find it again
jot todo --open                      # every unchecked box
jot backup --zip                     # take it all with you
```

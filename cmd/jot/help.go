package main

import (
	"fmt"
	"io"
)

func printHelp(w io.Writer) {
	fmt.Fprint(w, `jot: a fast, plaintext journal that lives in markdown files

Usage:
  jot <command> [flags] [arguments]

Commands:
  add        Append a timestamped entry
  show       Print entries
  edit       Open a day in your editor
  ls         List journals, or the days inside one journal
  search     Search across journals
  watch      Follow today's entries live
  backup     Write a compressed archive of every journal
  export     Write one markdown file per journal
  init       Create the journal root
  setup      Interactive first-run configuration
  install    Copy the jot binary into a bin directory
  update     Rebuild and reinstall from a source checkout
  uninstall  Delete journal data (the binary stays)
  doctor     Check the installation
  version    Print the version
  help       Show this help, or help for one command

Common flags:
  -j, --journal <name>   Journal to act on (default: the configured journal)
  -d, --date <day>       today (default), yesterday or YYYY-MM-DD

Examples:
  jot setup
  jot add "shipped the first release"
  jot add -j work "standup: on track"
  echo "note from a script" | jot add
  jot show
  jot show -d yesterday
  jot show -j work -a
  jot edit -j ideas
  jot search parser
  jot backup ~/Dropbox/jot-backups

Storage:
  ~/.jot/<journal>/YYYY-MM-DD.md     change with JOT_ROOT or the config file
  ~/.config/jot/config.json          root, default journal and editor

Run `+"`jot help <command>`"+` for details on any command.
`)
}

var commandHelp = map[string]string{
	"add": `jot add [flags] [text...]

Append one timestamped entry to a journal. The text can be given as
arguments, piped on stdin, or written interactively with ` + "`jot edit`" + `.

Flags:
  -j, --journal <name>  journal to append to (default: configured journal)
  -d, --date <day>      entry date: today (default), yesterday, YYYY-MM-DD

Examples:
  jot add "coffee with Sam about the migration"
  jot add -j work "standup: parser is done"
  jot add -d yesterday "forgot to log this"
  git log -1 | jot add -j work

Entries are appended, never rewritten, so a crash or full disk cannot
corrupt earlier notes.`,
	"show": `jot show [flags] [day]

Print entries. With no flags this prints today's entries for the default
journal. A day can be passed as a flag or as the first argument.

Flags:
  -j, --journal <name>  journal to read (default: configured journal)
  -d, --date <day>      today (default), yesterday, YYYY-MM-DD, or all
  -a, --all             print every day, with day headings
      --render          render markdown with a simple terminal layout

Examples:
  jot show
  jot show yesterday
  jot show -j work -a
  jot show -d 2026-10-01 --render`,
	"edit": `jot edit [flags] [day]

Open a journal day in $JOT_EDITOR, then the config editor setting, then
$VISUAL, then $EDITOR, then the first of vim, vi or nano found on PATH.
The file is created if it does not exist yet, so this is the natural way
to write longer, free-form notes.

Flags:
  -j, --journal <name>  journal to edit (default: configured journal)
  -d, --date <day>      today (default), yesterday or YYYY-MM-DD

Examples:
  jot edit
  jot edit -j ideas
  jot edit -d 2026-11-01`,
	"ls": `jot ls [flags] [journal]

With no argument, list journals with entry and day counts and the last
time each was touched. With a journal name, list that journal's days.

Flags:
  -j, --journal <name>  journal to inspect

Examples:
  jot ls
  jot ls work`,
	"search": `jot search [flags] <query>

Case-insensitive substring search across every journal, or one journal
when -j is given. Results are printed as journal/day.md:line: text.

Flags:
  -j, --journal <name>  limit the search to one journal

Examples:
  jot search parser
  jot search -j work standup
  jot search "next steps"`,
	"watch": `jot watch [flags]

Follow today's file and print entries as they are appended, until Ctrl+C.
Watching keeps working across midnight: the file for the new day is
picked up automatically. Useful in a second terminal while you work.

Flags:
  -j, --journal <name>  journal to follow
      --poll <duration> how often to check for new text (default 750ms)

Examples:
  jot watch
  jot watch -j work`,
	"backup": `jot backup [flags] [dir]

Write a .tar.gz of every journal into dir. Defaults to <root>/backups,
which is excluded from archives and from journal listings.

Flags:
      --out, -o <dir>  destination directory

Examples:
  jot backup
  jot backup ~/Dropbox/jot-backups

Restore with:
  tar -xzf jot-backup-*.tar.gz -C ~/.jot`,
	"export": `jot export [flags] [dir]

Write one markdown file per journal into dir (default ./jot-export), with
a heading per day: a flat snapshot for publishing or importing elsewhere.

Flags:
      --out, -o <dir>  destination directory

Examples:
  jot export
  jot export --out ~/notes-export`,
	"init": `jot init [flags]

Create the journal root and the default journal. Safe to run repeatedly;
it never overwrites anything.

Flags:
  -j, --journal <name>  also create this journal
      --root <dir>      use this root instead of the configured one

Examples:
  jot init
  jot init -j work`,
	"setup": `jot setup [flags]

Interactive first-run wizard: picks the journal root, default journal and
editor, writes the config file, creates the root and installs the binary.
Run it once after installing.

Flags:
      --root <dir>       journal root (default ~/.jot)
      --journal <name>   default journal (default "default")
      --editor <cmd>     editor command (default: current setting, then detected, then vi)
      --install-dir <dir> where to copy the binary (default ~/.local/bin)
      --install          copy the binary without asking
      --no-install       do not copy the binary
  -y, --yes              accept defaults, no prompts

Examples:
  jot setup
  jot setup --yes
  jot setup --root ~/Documents/journal --journal daily --no-install`,
	"install": `jot install [flags] [dir]

Copy the running binary into dir (default ~/.local/bin, or
JOT_INSTALL_DIR) and report whether that directory is on PATH. Running it
from the installed copy is a no-op.

Flags:
      --dir <dir>  destination directory

Examples:
  jot install
  jot install /usr/local/bin
  JOT_INSTALL_DIR=~/bin jot install`,
	"update": `jot update [flags]

Rebuild the binary from a local source checkout and install it, replacing
the copy in your bin directory. Requires Go and the repository.

Flags:
      --src <dir>  source checkout (default: JOT_SRC, then the current
                   directory and its parents)
      --dir <dir>  install directory (default: JOT_INSTALL_DIR, else ~/.local/bin)

Examples:
  jot update
  jot update --src ~/code/jot --dir ~/.local/bin`,
	"uninstall": `jot uninstall [flags]

Delete the journal root and every entry in it. The binary is left in
place; remove it with rm $(command -v jot) if you want it gone too.
Refuses to run without confirmation unless -y is given.

Flags:
  -y, --yes  do not ask for confirmation

Examples:
  jot uninstall
  jot uninstall --yes`,
	"doctor": `jot doctor

Check the installation: root directory and its permissions, config file,
journal counts, editor, and whether jot is on PATH. Exits non-zero if a
check fails.
`,
	"version": `jot version

Print the version of this binary.
`,
	"help": `jot help [command]

Print the overview help, or the help page for one command.

Examples:
  jot help
  jot help add
  jot help backup`,
}

func printCommandHelp(name string, w io.Writer) error {
	text, ok := commandHelp[name]
	if !ok {
		return fmt.Errorf("no help for %q; run `jot help`", name)
	}
	fmt.Fprint(w, text+"\n")
	return nil
}

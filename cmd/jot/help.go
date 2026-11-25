package main

import (
	"fmt"
	"io"
)

func printHelp(w io.Writer) {
	fmt.Fprint(w, `jot: a fast, plaintext journal that lives in markdown files

Usage:
  jot                    Open the TUI when run in a terminal, else this help
  jot <command> [flags] [arguments]

Commands:
  tui        Open the interactive interface (bare `+"`jot`"+` in a terminal)
  add        Append a timestamped entry
  show       Print entries
  edit       Open a day in your editor
  ls         List journals, or the days inside one journal
  search     Search across journals
  todo       List checkbox todos inside entries
  tags       List #tags and how often each is used
  heatmap    Print a year heatmap of entry density
  watch      Follow today's entries live
  remind     Set and manage countdown reminders
  backup     Write a compressed archive of every journal
  export     Write one markdown file per journal
  journal    Create, delete or rename journals
  init       Create the journal root
  setup      Interactive first-run configuration
  install    Copy the jot binary into a bin directory
  update     Update to the latest release, or rebuild from source
  uninstall  Remove journal data, the binary and/or config
  doctor     Check the installation
  version    Print the version
  help       Show this help, or help for one command

Common flags:
  -j, --journal <name>   Journal to act on (default: the configured journal)
  -d, --date <day>       today (default), yesterday or YYYY-MM-DD

Examples:
  jot
  jot tui
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
  ~/.config/jot/config.json          root, default journal, editor and the
                                     tui block (theme, last journal)

Run `+"`jot help <command>`"+` for details on any command.
`)
}

var commandHelp = map[string]string{
	"tui": `jot tui [-j NAME] [--theme NAME]

Open the interactive interface: a month calendar, the rendered entries of
the selected day, a journal sidebar, and a one-line quick-add box.

Press ? inside the TUI for every keybinding.

The interface is read-mostly. Quick add appends an entry; editing or
removing content always happens in your editor (press e), and jot never
rewrites entries itself. R renames the open journal and D deletes it after
archiving it under <root>/backups and asking you to type its name; the
default journal is protected.

Flags:
  -j, --journal <name>  journal to open (default: the last journal you
                        opened, else the configured journal)
      --theme <name>    color theme: auto, default, light, dark, gruvbox,
                        nord, dracula, catppuccin, tokyo-night or omarchy
                        (default: JOT_THEME, then tui.theme, then auto)

Examples:
  jot tui
  jot tui -j work
  jot tui --theme nord

Running bare ` + "`jot`" + ` in a terminal opens the TUI with the same
defaults.`,
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
      --render          render markdown for the terminal; honours NO_COLOR
                        and picks light styles when JOT_THEME names a light
                        theme
      --calendar [YYYY-MM]  print a month grid for YYYY-MM (default: the
                        current month); a dot marks 1-2 entries, a star 3+

Examples:
  jot show
  jot show yesterday
  jot show -j work -a
  jot show -d 2026-10-01 --render
  jot show --calendar 2026-01
  jot show --calendar`,
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
when -j is given. Hits are grouped by journal and day under a header
line, each hit shows its line number, and a summary closes the list.

Flags:
  -j, --journal <name>  limit the search to one journal
      --fuzzy           fuzzy entry-level search with a scored preview per
                        entry, best first
      --limit <n>       maximum fuzzy results (default 20)

Filters (any filter switches to entry-level results):
  #tag or tag:name      entries carrying a #tag
  journal:name          one journal
  after:DAY before:DAY  inclusive; DAY is YYYY-MM-DD, today, yesterday or Nd
  has:todo  is:open     entries with a checkbox / an unchecked one

Examples:
  jot search parser
  jot search "#work after:7d"
  jot search -j work standup
  jot search "next steps"
  jot search --fuzzy --limit 10 parser`,
	"tags": `jot tags [flags]

List every #tag (a '#' then a word, e.g. #work or #work/infra) with the number
of entries using it, most used first. Search with jot search "#tag".

Flags:
  -j, --journal <name>  count one journal only`,
	"todo": `jot todo [flags]

List checkbox items ("- [ ]" / "- [x]") found in entry bodies across every
journal, newest day first, one line per item with its journal, day and
time, and an "N open . M done" summary.

The command is read-only: toggling a box is done by editing the day, e.g.
jot edit -d 2026-11-01 -j work.

Flags:
  -j, --journal <name>  limit the list to one journal
      --open            only unchecked items
      --done            only completed items
      --all             everything (the default)
      --limit <n>       show at most n items

Examples:
  jot todo
  jot todo --open -j work
  jot todo --limit 10`,
	"heatmap": `jot heatmap [flags]

Print one year of a journal as a heatmap: two half-year grids, weeks as
rows, Monday-first weekdays as columns, and a month label on each row
where a month begins. Every day with a file is a cell shaded by how many
entries it holds:

  .. none   .. 1-2   .. 3-5   .. 6+ entries   (a block-glyph ramp)

Color follows the terminal: truecolor shades when out is a terminal,
plain glyphs for pipes and NO_COLOR, and a lighter ramp under
JOT_THEME=light.

Flags:
  -j, --journal <name>  journal to chart (default: the configured journal)
      --year <yyyy>     year to chart (default: the current year)

Examples:
  jot heatmap
  jot heatmap -j work
  jot heatmap --year 2025`,
	"remind": `jot remind <MESSAGE DURATION | list | clear>

Set a countdown reminder, or manage existing ones. Durations take Go-style
units (90s, 20m, 2h, 1h30m); a bare number means minutes. Reminders fire
through omarchy when it is present, otherwise through systemd user timers
with notify-send.

  jot remind MESSAGE DURATION  set a reminder; the duration may come first
  jot remind list              show active reminders
  jot remind clear             stop and remove every jot reminder

Examples:
  jot remind "stretch" 20m
  jot remind 2h "review the draft"
  jot remind list`,
	"journal": `jot journal <new|rm|rename|ls> [arguments]

Manage journals.

  jot journal new NAME         create a journal and print its directory
  jot journal rm [--yes] NAME  archive to <root>/backups, confirm, then delete
  jot journal rename OLD NEW   rename a journal
  jot journal ls               list journals (same output as jot ls)
  jot journal encrypt NAME     age-encrypt every day file; new writes stay encrypted
  jot journal decrypt NAME     turn an encrypted journal back into plain markdown
  jot journal archives         list archives and backups in <root>/backups
  jot journal restore ARCHIVE  unpack an archive back into the root

` + "`journal rm`" + ` always writes an archive into <root>/backups before deleting,
refuses the default journal and the backups name, and asks you to type the
journal name exactly; anything else aborts with the journal intact. Pass
--yes to skip the prompt in scripts.

Encryption uses the age key at ~/.config/jot/age-key.txt (JOT_KEY overrides),
created on first use. Back it up: without it encrypted journals are unreadable.
Files keep their .md names; their content is armored age ciphertext.

Examples:
  jot journal new ideas
  jot journal encrypt private
  jot journal rm --yes old-notes
  jot journal rename work deep-work`,
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

Write one archive of everything jot keeps into dir (default <root>/backups,
which is itself left out): every journal with its templates and encryption
markers, a root template, and the config file as .jot/config.json.

Flags:
      --out, -o <dir>  destination directory
      --zip            write a .zip instead of a .tar.gz
      --with-key       also bundle the age key as .jot/age-key.txt; anyone
                       holding the archive can then read encrypted journals

Examples:
  jot backup
  jot backup --zip ~/Dropbox/jot-backups
  jot backup --with-key /media/usb

Restore journals with jot journal restore ARCHIVE (refuses journals that
exist), or unpack by hand: tar -xzf jot-backup-*.tar.gz -C ~/.jot
(.jot/ holds the config and key; copy them to ~/.config/jot yourself).`,
	"export": `jot export [flags] [dir]

Write one file per journal into dir (default ./jot-export). The default
markdown format is a flat snapshot with a heading per day. html and pdf
write a styled document: a cover page with the date range and counts, a
month-by-month contents page, then every day with its entries rendered.
PDF is printed by a headless Chrome, Chromium, Edge or Brave.

Flags:
      --out, -o <dir>    destination directory
      --format <fmt>     md (default), html or pdf
  -j, --journal <name>   html/pdf: one journal (default: every journal)
      --from, --to <day> html/pdf: limit the days (YYYY-MM-DD, today, yesterday)

Examples:
  jot export
  jot export --format pdf -j work --from 2026-01-01
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

Check the published releases and install the build for this platform,
replacing the copy in your bin directory. By default it shows the
available release and asks before installing. The download is verified
against the release SHA256SUMS before it is renamed into place.

Flags:
      --check          report the release status without installing
      --source         rebuild from a local source checkout instead
      --src <dir>      source checkout for --source (default: JOT_SRC,
                       then the current directory and its parents)
      --dir <dir>      install directory (default: JOT_INSTALL_DIR, else
                       ~/.local/bin)
      --version <tag>  install a specific release, e.g. v0.2.0
  -y, --yes            install without asking
      --notes          print the full release notes
      --json           print one machine-readable status object

Without a terminal and without --yes, update only reports the status and
exits 0. Set JOT_RELEASES_API to use a mirror or a test server.

Examples:
  jot update
  jot update --check
  jot update --yes
  jot update --version v0.2.0
  jot update --check --json
  jot update --source --src ~/code/jot`,
	"uninstall": `jot uninstall [flags]

Choose what to remove: journal data, the running binary, the config file,
or all three. With no component flag and an interactive terminal, jot
lists what exists and asks. Component flags remove only what is named;
--yes skips the confirmation prompts.

Data removal deletes the journal root, including journals, every entry
and every backup. It warns first and asks you to type "delete". Removal
runs in a safe order: config, then data, then the binary last.

Flags:
      --data      remove the journal root (journals, backups, all entries)
      --binary    remove the running jot executable
      --config    remove the config file (when it exists)
      --all       remove data, binary and config
      --dry-run   print what would be removed, remove nothing
  -y, --yes       do not ask for confirmation

Without a terminal and without a component flag, uninstall is a usage
error: it never guesses what to delete. jot uninstall --yes keeps the old
behaviour and removes journal data only.

Examples:
  jot uninstall
  jot uninstall --dry-run --all
  jot uninstall --data --yes
  jot uninstall --binary
  jot uninstall --config --yes`,
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

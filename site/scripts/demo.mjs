#!/usr/bin/env node
// Regenerate src/data/demo.json: a deterministic synthetic journal, the
// terminal frames captured from the real binary, and its real CLI output.
// Requires Go and tmux. Never touches ~/.jot or the user config.
//
//   node scripts/demo.mjs            # seed, capture, write JSON
//   node scripts/demo.mjs --check    # verify the JSON is current
import { execFileSync, execSync } from "node:child_process";
import { existsSync, mkdirSync, readFileSync, readdirSync, rmSync, writeFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const site = resolve(here, "..");
const repo = resolve(site, "..");
const cache = join(site, ".cache");
const bin = join(cache, "jot");
const root = join(cache, "demo-root");
const cfgPath = join(cache, "demo-config.json");
const outPath = join(site, "src", "data", "demo.json");
const TODAY = new Date(2026, 10, 25); // 25 November 2026

const pad = (n) => String(n).padStart(2, "0");
const iso = (d) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
const WEEKDAY = ["Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"];

// Deterministic PRNG so every run produces the same journal.
function mulberry32(a) {
  return function () {
    a |= 0; a = (a + 0x6d2b79f5) | 0;
    let t = Math.imul(a ^ (a >>> 15), 1 | a);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}
const rnd = mulberry32(20261125);
const pick = (arr) => arr[Math.floor(rnd() * arr.length)];

const TIMES = ["07:42:10", "08:05:33", "08:31:04", "09:14:52", "09:41:07", "10:22:18",
  "11:26:40", "12:19:05", "13:58:21", "14:30:12", "15:12:44", "16:04:09", "17:05:58",
  "18:22:31", "19:48:16", "21:07:40", "22:33:02"];

const NOTES = [
  "walked to the harbour before standup; fog all the way out",
  "the day file is 11 KB and has never needed a migration",
  "grep is still my search; fuzzy is for when I forget the word",
  "fixed the heat levels: one entry is faint, five is solid",
  "spent the afternoon reading the bubbletea tea.Model docs",
  "coffee with Sam about the migration",
  "rain. read three days of old entries instead of writing one",
  "the calendar is a month of small squares; density reads at a glance",
  "rewrote the renderer to take a width, not a terminal",
  "took the long way home past the market",
  "decided against tags. journals are enough",
  "a week of short notes, nothing worth a heading",
  "the export is one file per journal, plain markdown",
  "checked the mode bits by hand: 0700 for the root, 0600 for the file",
  "read the install script twice before running it",
  "the TUI does not rewrite entries. that constraint is the feature",
  "early train; wrote the day plan on paper first",
  "the doctor command is a checklist, and that is fine",
  "deleted nothing today",
  "tried a theme with too much blue and went back to the default",
  "the search scoring: prefix bonus, then word start, then substring",
  "wrote the troubleshooting page from the bug reports",
  "the best feature is that the files are still there after uninstalling",
  "one entry is enough to mark the day",
  "split the calendar math out into its own package",
  "finally understood why the month grid needs six rows",
  "kept the todo overlay read-only and slept better",
  "backdated an entry for yesterday; the timestamp is honest about the edit",
  "sun all afternoon. wrote nothing until nine",
  "the editor handoff is three lines of code and the whole product",
  "checked a year of files with find and wc. no surprises",
  "renamed the second journal from notes to reading",
  "the README is longer than the program",
  "two entries before breakfast, then a quiet day",
  "moved the root to a new disk with rsync and diffed the trees",
  "the reminder only fires once. transient by design",
  "a long entry about a short bug",
  "the config file is JSON because everything else already reads JSON",
  "kept the daemon out. there is nothing to run",
  "the install script verifies the checksum before it writes the binary",
  "asked what happens when the directory is read-only; wrote it down",
  "an old entry mentioned a problem I had forgotten. that is the point",
  "five minutes in the terminal, then back to work",
  "the month totals are the only statistic the TUI shows",
  "opened last year journal by mistake and read it for an hour",
  "the file format has not changed since the first commit",
];

const TODOS = [
  "- [ ] decide whether themes belong in config or in the theme file",
  "- [x] back up ~/.jot before the disk swap",
  "- [ ] send the agenda",
  "- [x] book the room for Thursday",
  "- [ ] write the migration note",
  "- [x] check the release checksums",
  "- [ ] try the light theme for a week",
  "- [x] update the troubleshooting section",
  "- [ ] add the missing key to the help overlay",
  "- [x] rename the second journal",
];

const WORK = [
  "standup: parser merged, docs next",
  "reviewed the interrupt handling patch",
  "paired with Ana on the fuzzy scorer",
  "release 0.1.0 tagged",
  "deployed the docs site behind the same markdown files",
  "reviewed the installer script; checksum verified before install",
  "wrote the migration note, kept it to one page",
  "the incident was a clock skew, not the parser",
  "quiet day. two reviews and a long walk",
  "retro: fewer meetings, more notes",
  "shipped the smaller calendar patch",
  "the release checklist is now a file in the repo",
  "fixed the flaky test by removing the clock from it",
  "read the diff twice. merged once",
  "the build is a single binary again",
  "cleaned up the vendor directory",
  "handed the on-call phone over. nothing on fire",
  "the docs PR sat for a week and then took ten minutes",
  "split the CLI flags from the library options",
  "the changelog entry for this release is one line",
  "copied the release notes into the journal",
  "a quiet Friday. closed three small issues",
  "reviewed the theme file parser",
  "the migration went out at noon. no pages",
];

const READING = [
  "finished The Peregrine. the diary form is the point, not the bird",
  "re-read the calendar chapter of Tufte. small multiples, no chart junk",
  "started a notebook for the terminal rendering paper",
  "a chapter on indexes; skipped the exercises",
  "found a used copy of The Elements of Typographic Style",
  "the library book is due. read the last chapter standing up",
  "notes on marginalia: the margin is not decoration, it is a second column",
  "a short essay on why plain text survives",
  "started a book about paper calendars; stopped at the diagrams",
  "read the colophon first, as always",
];

// The final week is written by hand so the captured screens have a shape.
const FINAL_WEEK = {
  "2026-11-19": [["09:12:40", "wrote the todo overlay. read-only by choice: tick boxes in the editor"], ["14:40:18", "paired with Ana on the fuzzy scorer"], ["21:03:55", "read three weeks of October instead of writing"]],
  "2026-11-20": [["08:05:33", "- [ ] decide whether themes belong in config or in the theme file"], ["11:30:02", "rain. the calendar is a month of small squares and that is enough"]],
  "2026-11-21": [["10:22:18", "rebuilt the calendar with month totals"], ["20:15:44", "- [x] back up ~/.jot before the disk swap"]],
  "2026-11-22": [["09:04:51", "short note"], ["18:40:12", "walked the long way home"]],
  "2026-11-23": [["08:41:07", "the whole day is one file and one diff"], ["13:12:26", "- [ ] send the agenda"], ["22:02:31", "tried writing in the TUI instead of the editor. quick add is enough"]],
  "2026-11-24": [["09:30:12", "rebuilt the day pane around the renderer"], ["16:05:58", "- [x] book the room for Thursday"]],
  "2026-11-25": [
    ["09:41:07", "the whole day is one file and one diff"],
    ["14:30:12", "- [ ] decide whether themes belong in config or in the theme file\n- [x] back up ~/.jot before the disk swap"],
    ["21:07:40", "quick add is the only write the TUI makes"],
  ],
};

function planDays() {
  const days = new Map();
  const push = (date, journal, time, body) => {
    const key = `${date}\u0000${journal}`;
    if (!days.has(key)) days.set(key, []);
    days.get(key).push({ time, body });
  };
  // n distinct timestamps, in order, from the fixed pool.
  const timesFor = (n) => {
    const pool = [...TIMES];
    const out = [];
    for (let i = 0; i < n && pool.length; i++) out.push(pool.splice(Math.floor(rnd() * pool.length), 1)[0]);
    return out.sort();
  };
  for (let d = new Date(2026, 0, 1); d <= TODAY; d.setDate(d.getDate() + 1)) {
    const date = iso(d);
    if (d.getMonth() === 10) continue; // November is planned below, denser
    const dow = d.getDay();
    const weekday = dow !== 0 && dow !== 6;
    if (rnd() < (weekday ? 0.62 : 0.34)) {
      push(date, "default", pick(TIMES), rnd() < 0.18 ? pick(TODOS) : pick(NOTES));
      if (rnd() < 0.16) push(date, "default", pick(TIMES), pick(NOTES));
    }
    if (rnd() < (weekday ? 0.34 : 0.08)) push(date, "work", pick(TIMES), pick(WORK));
    if (rnd() < 0.075) push(date, "reading", pick(TIMES), pick(READING));
  }
  // November: the month the captures show, so it carries visible heat.
  const heavy = new Set([3, 9, 12, 18, 23]);
  for (let d = new Date(2026, 10, 1); d <= TODAY; d.setDate(d.getDate() + 1)) {
    const date = iso(d);
    if (FINAL_WEEK[date]) {
      for (const [time, body] of FINAL_WEEK[date]) push(date, "default", time, body);
      if (d.getDay() % 2 === 1) push(date, "work", pick(TIMES), pick(WORK));
      continue;
    }
    const weekend = d.getDay() === 0 || d.getDay() === 6;
    const n = heavy.has(d.getDate()) ? 5 : 2 + Math.floor(rnd() * (weekend ? 2 : 3));
    timesFor(n).forEach((time, i) => {
      const body = rnd() < 0.2 ? pick(TODOS) : pick(NOTES);
      push(date, "default", time, i === 0 && rnd() < 0.15 ? `${body}\n\nthe rest of the day was quiet` : body);
    });
    if (rnd() < 0.5) push(date, "work", pick(TIMES), pick(WORK));
    if (rnd() < 0.1) push(date, "reading", pick(TIMES), pick(READING));
  }
  return days;
}

function writeJournal(days) {
  rmSync(root, { recursive: true, force: true });
  mkdirSync(root, { recursive: true });
  writeFileSync(cfgPath, JSON.stringify({ root, default_journal: "default", tui: { theme: "default" } }, null, 2) + "\n");
  const sorted = [...days.entries()].sort(([a], [b]) => a.localeCompare(b));
  for (const [key, entries] of sorted) {
    const [date, journal] = key.split("\u0000");
    entries.sort((a, b) => a.time.localeCompare(b.time));
    const dir = join(root, journal);
    mkdirSync(dir, { recursive: true });
    const body = entries.map((e) => `## ${e.time}\n\n${e.body}\n`).join("\n");
    writeFileSync(join(dir, `${date}.md`), body);
  }
}

function loadJournal() {
  const journals = [];
  for (const name of readdirSync(root, { withFileTypes: true })) {
    if (!name.isDirectory()) continue;
    const dir = join(root, name.name);
    const days = [];
    for (const file of readdirSync(dir).sort()) {
      if (!file.endsWith(".md")) continue;
      const date = file.slice(0, -3);
      const raw = readFileSync(join(dir, file), "utf8");
      const entries = [];
      const re = /^## (\d{2}:\d{2}:\d{2})$/gm;
      const marks = [...raw.matchAll(re)];
      marks.forEach((m, i) => {
        const start = m.index + m[0].length;
        const end = i + 1 < marks.length ? marks[i + 1].index : raw.length;
        const body = raw.slice(start, end).trim();
        if (body) entries.push({ time: m[1], body });
      });
      if (!entries.length) continue;
      const [y, mo, da] = date.split("-").map(Number);
      days.push({ date, weekday: WEEKDAY[new Date(y, mo - 1, da).getDay()], bytes: Buffer.byteLength(raw), entries });
    }
    journals.push({ name: name.name, days });
  }
  journals.sort((a, b) => (a.name === "default" ? -1 : b.name === "default" ? 1 : a.name.localeCompare(b.name)));
  return journals;
}

function build() {
  mkdirSync(cache, { recursive: true });
  execFileSync("go", ["build", "-trimpath", "-o", bin, "./cmd/jot"], { cwd: repo, stdio: "inherit" });
}

const jotEnv = () => ({ ...process.env, NO_COLOR: "", JOT_ROOT: root, JOT_CONFIG: cfgPath });

function run(args) {
  return execFileSync(bin, args, { env: jotEnv(), encoding: "utf8" }).replace(/\s+$/, "");
}

function captureFrames() {
  const out = join(cache, "frames");
  rmSync(out, { recursive: true, force: true });
  mkdirSync(out, { recursive: true });
  const env = Object.fromEntries(Object.entries(process.env).filter(([k]) => k !== "NO_COLOR"));
  Object.assign(env, { TERM: "xterm-256color", COLORTERM: "truecolor" });
  const tmux = (args, opts = {}) => execFileSync("tmux", args, { env, encoding: "utf8", ...opts });
  const start = () => {
    try { tmux(["kill-session", "-t", "jot-demo"]); } catch {}
    const cmd = `TERM=xterm-256color COLORTERM=truecolor JOT_CONFIG=${cfgPath} JOT_ROOT=${root} ${bin} tui; sleep 90`;
    tmux(["new-session", "-d", "-s", "jot-demo", "-x", "104", "-y", "30", cmd]);
    execSync("sleep 2.3");
  };
  const shot = (name) => {
    const ansi = tmux(["capture-pane", "-t", "jot-demo", "-p", "-e"]);
    writeFileSync(join(out, `${name}.ansi`), ansi);
    try { tmux(["kill-session", "-t", "jot-demo"]); } catch {}
  };
  const keys = (...k) => { tmux(["send-keys", "-t", "jot-demo", ...k]); execSync("sleep 0.6"); };
  const type = (text) => { tmux(["send-keys", "-t", "jot-demo", "-l", text]); execSync("sleep 0.8"); };

  start(); shot("calendar");
  start(); keys("/"); type("parser"); shot("search");
  start(); keys("T"); shot("todos");
  start(); keys("?"); shot("help");
  return out;
}

function loadFrames(dir) {
  const meta = [
    { id: "calendar", label: "The month", note: "A calendar with entry heat, the journal list and the selected day rendered as Markdown." },
    { id: "search", label: "Search", note: "Fuzzy search across every journal. Tab narrows the scope to the current one." },
    { id: "todos", label: "Todos", note: "Every checkbox in one overlay. jot never toggles a box; Enter opens the day in your editor." },
    { id: "help", label: "Keys", note: "The full key reference, one keystroke away." },
  ];
  return meta.map((m) => {
    const raw = readFileSync(join(dir, `${m.id}.ansi`), "utf8");
    const lines = raw.replace(/\n+$/, "").split("\n");
    return { ...m, cols: Math.max(...lines.map((l) => strip(l).length)), rows: lines.length, lines };
  });
}

const strip = (s) => s.replace(/\x1b\[[0-9;?]*[a-zA-Z]/g, "");

function transcript() {
  const cmds = [
    ["jot ls", ["ls"]],
    ["jot show -d 2026-11-25", ["show", "-d", "2026-11-25"]],
    ["jot search --fuzzy parser", ["search", "--fuzzy", "parser"]],
    ["jot todo", ["todo"]],
  ];
  return cmds.map(([cmd, args]) => ({ cmd, out: run(args) }));
}

const mode = process.argv[2];
const days = planDays();
writeJournal(days);
build();
const journals = loadJournal();
if (!existsSync(bin)) throw new Error("binary missing");
const totals = journals.reduce((acc, j) => {
  acc.days += j.days.length;
  acc.entries += j.days.reduce((n, d) => n + d.entries.length, 0);
  acc.bytes += j.days.reduce((n, d) => n + d.bytes, 0);
  return acc;
}, { days: 0, entries: 0, bytes: 0, journals: journals.length });
const frames = captureFrames();
const data = {
  today: iso(TODAY),
  totals,
  journals,
  frames: loadFrames(frames),
  transcript: transcript(),
};
writeFileSync(outPath, JSON.stringify(data, null, 1) + "\n");
console.log(`wrote ${outPath}: ${totals.days} days, ${totals.entries} entries, ${data.frames.length} frames`);
if (mode === "--check") console.log("checked");
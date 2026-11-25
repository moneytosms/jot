// The sample journal baked into the site by scripts/demo.mjs. Every date,
// entry, byte count and terminal frame below comes from that run; nothing is
// typed by hand into a component.
import raw from "../data/demo.json";

export interface Entry { time: string; body: string }
export interface Day { date: string; weekday: string; bytes: number; entries: Entry[] }
export interface Journal { name: string; days: Day[] }
export interface Frame { id: string; label: string; note: string; cols: number; rows: number; lines: string[] }
export interface Demo {
  today: string;
  totals: { days: number; entries: number; bytes: number; journals: number };
  journals: Journal[];
  frames: Frame[];
  transcript: { cmd: string; out: string }[];
}

export const demo = raw as unknown as Demo;
export const journal = demo.journals.find((j) => j.name === "default") ?? demo.journals[0];
export const byDate = new Map(journal.days.map((d) => [d.date, d]));
export const countOn = (date: string) => byDate.get(date)?.entries.length ?? 0;

const MONTHS = ["January", "February", "March", "April", "May", "June", "July",
  "August", "September", "October", "November", "December"];
const SHORT = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];
const DAY_MS = 86_400_000;

export const parseDate = (date: string) => {
  const [y, m, d] = date.split("-").map(Number);
  return new Date(Date.UTC(y, m - 1, d));
};
export const isoDate = (d: Date) => d.toISOString().slice(0, 10);

export interface RecentDay {
  date: string;
  weekday: string;
  weekdayShort: string;
  day: number;
  month: string;
  year: number;
  count: number;
  bytes: number;
  entries: Entry[];
  isToday: boolean;
  raw: string;
}

const toRecent = (date: string, isToday: boolean): RecentDay => {
  const day = byDate.get(date);
  const d = parseDate(date);
  return {
    date,
    weekday: day?.weekday ?? new Date(Date.UTC(d.getUTCFullYear(), d.getUTCMonth(), d.getUTCDate())).toLocaleDateString("en-GB", { weekday: "long", timeZone: "UTC" }),
    weekdayShort: d.toLocaleDateString("en-GB", { weekday: "short", timeZone: "UTC" }),
    day: d.getUTCDate(),
    month: MONTHS[d.getUTCMonth()],
    year: d.getUTCFullYear(),
    count: day?.entries.length ?? 0,
    bytes: day?.bytes ?? 0,
    entries: day?.entries ?? [],
    isToday,
    raw: rawOf(day?.entries ?? []),
  };
};

/** The calendar days ending today, newest last. Gaps are kept on purpose. */
export function recentDays(n: number): RecentDay[] {
  const today = parseDate(demo.today);
  return Array.from({ length: n }, (_, i) => {
    const d = new Date(today.getTime() - (n - 1 - i) * DAY_MS);
    return toRecent(isoDate(d), i === n - 1);
  });
}

/** The exact file body jot writes: "## HH:MM:SS", blank line, text, blank line. */
export function rawOf(entries: Entry[]): string {
  return entries.map((e) => `## ${e.time}\n\n${e.body}\n`).join("\n");
}

export const longDate = (date: string, weekday?: string) => {
  const d = parseDate(date);
  return `${weekday ? weekday + ", " : ""}${d.getUTCDate()} ${MONTHS[d.getUTCMonth()]} ${d.getUTCFullYear()}`;
};
export const shortDate = (date: string) => {
  const d = parseDate(date);
  return `${d.getUTCDate()} ${SHORT[d.getUTCMonth()]} ${d.getUTCFullYear()}`;
};

/** Heat: 0 empty, 1 a couple of lines, 4 a day that ran long. */
export const heat = (count: number) => (count === 0 ? 0 : count <= 2 ? 1 : count === 3 ? 2 : count === 4 ? 3 : 4);

export interface YearCell { date: string; day: number; count: number; level: number; today: boolean }
export interface YearChart { weeks: (YearCell | null)[][]; months: { label: string; col: number }[] }

/** Monday-first weeks covering the whole year. */
export function yearWeeks(year: number): YearChart {
  const jan1 = Date.UTC(year, 0, 1);
  const dec31 = Date.UTC(year, 11, 31);
  const mondayOffset = (new Date(jan1).getUTCDay() + 6) % 7;
  const first = jan1 - mondayOffset * DAY_MS;
  const weeks: (YearCell | null)[][] = [];
  const monthCols = new Map<number, number>();
  for (let start = first; start <= dec31; start += 7 * DAY_MS) {
    const week: (YearCell | null)[] = [];
    for (let i = 0; i < 7; i++) {
      const d = new Date(start + i * DAY_MS);
      if (d.getUTCFullYear() !== year) { week.push(null); continue; }
      const date = isoDate(d);
      if (d.getUTCDate() === 1 && !monthCols.has(d.getUTCMonth())) {
        monthCols.set(d.getUTCMonth(), weeks.length);
      }
      const count = countOn(date);
      week.push({ date, day: d.getUTCDate(), count, level: heat(count), today: date === demo.today });
    }
    weeks.push(week);
  }
  const months = [...monthCols.entries()]
    .sort((a, b) => a[0] - b[0])
    .map(([m, col]) => ({ label: SHORT[m], col }));
  return { weeks, months };
}

// Markdown, the subset the journal actually uses: paragraphs, checkboxes and
// inline code. Everything is escaped first; the HTML is ours, not the file's.
const escapeHtml = (s: string) =>
  s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");

export function inlineMd(text: string): string {
  return escapeHtml(text).replace(/`([^`]+)`/g, "<code>$1</code>");
}

export function entryHtml(body: string): string {
  const blocks = body.split(/\n{2,}/).filter((b) => b.trim());
  return blocks.map((block) => {
    const lines = block.split("\n");
    const todos = lines.every((l) => /^- \[[ xX]\] /.test(l));
    if (todos) {
      const items = lines.map((l) => {
        const done = /^- \[[xX]\]/.test(l);
        const text = l.replace(/^- \[[ xX]\] /, "");
        return `<li>${done ? '<span class="tick" aria-hidden="true">[x]</span><span class="sr-only">done: </span>' : '<span class="tick" aria-hidden="true">[ ]</span>'}${inlineMd(text)}</li>`;
      });
      return `<ul class="ticks">${items.join("")}</ul>`;
    }
    return `<p>${lines.map(inlineMd).join("<br />")}</p>`;
  }).join("");
}

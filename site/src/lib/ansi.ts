// ANSI (SGR) to HTML. The frames in src/data/demo.json are real
// `tmux capture-pane -e` output; this renders them as selectable text so the
// terminal snaps to any pixel density and stays readable to a screen reader.

const BASE: Record<number, string> = {
  30: "#3b4048", 31: "#f7768e", 32: "#9ece6a", 33: "#e0af68",
  34: "#7aa2f7", 35: "#bb9af7", 36: "#7dcfff", 37: "#d5dbe3",
  90: "#7a8494", 91: "#ff9db0", 92: "#b9e88a", 93: "#f0c27d",
  94: "#96b7fb", 95: "#d0b3ff", 96: "#a3e4ff", 97: "#ffffff",
};

const CUBE = [0, 95, 135, 175, 215, 255];

const TABLE16 = ["#000000", "#800000", "#008000", "#808000", "#000080", "#800080", "#008080", "#c0c0c0",
  "#808080", "#ff0000", "#00ff00", "#ffff00", "#0000ff", "#ff00ff", "#00ffff", "#ffffff"];

export function xterm256(n: number): string {
  if (n < 16) return TABLE16[n] ?? "#000000";
  if (n < 232) {
    const i = n - 16;
    return `rgb(${CUBE[Math.floor(i / 36)]},${CUBE[Math.floor((i % 36) / 6)]},${CUBE[i % 6]})`;
  }
  const v = 8 + (n - 232) * 10;
  return `rgb(${v},${v},${v})`;
}

const escapeHtml = (s: string) =>
  s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");

interface Style { fg?: string; bg?: string; bold?: boolean; dim?: boolean; italic?: boolean; underline?: boolean; }

function css(st: Style): string {
  const out: string[] = [];
  if (st.fg) out.push(`color:${st.fg}`);
  if (st.bg) out.push(`background:${st.bg}`);
  if (st.bold) out.push("font-weight:600");
  if (st.dim) out.push("opacity:.72");
  if (st.italic) out.push("font-style:italic");
  if (st.underline) out.push("text-decoration:underline");
  return out.join(";");
}

export function ansiLine(line: string): string {
  const st: Style = {};
  let out = "";
  let last = 0;
  const re = /\x1b\[([0-9;]*)m/g;
  let m: RegExpExecArray | null;
  while ((m = re.exec(line))) {
    const text = line.slice(last, m.index);
    if (text) {
      const style = css(st);
      out += style ? `<span style="${style}">${escapeHtml(text)}</span>` : escapeHtml(text);
    }
    last = re.lastIndex;
    const params = m[1].split(";").filter((p) => p !== "").map(Number);
    if (!params.length) params.push(0);
    for (let i = 0; i < params.length; i++) {
      const p = params[i];
      if (p === 0) { st.fg = undefined; st.bg = undefined; st.bold = undefined; st.dim = undefined; st.italic = undefined; st.underline = undefined; }
      else if (p === 1) st.bold = true;
      else if (p === 2) st.dim = true;
      else if (p === 3) st.italic = true;
      else if (p === 4) st.underline = true;
      else if (p === 22) { st.bold = undefined; st.dim = undefined; }
      else if (p === 23) st.italic = undefined;
      else if (p === 24) st.underline = undefined;
      else if (p >= 30 && p <= 37) st.fg = BASE[p];
      else if (p >= 90 && p <= 97) st.fg = BASE[p];
      else if (p === 39) st.fg = undefined;
      else if (p >= 40 && p <= 47) st.bg = BASE[p - 10];
      else if (p >= 100 && p <= 107) st.bg = BASE[p - 10];
      else if (p === 49) st.bg = undefined;
      else if (p === 38 || p === 48) {
        const mode = params[++i];
        if (mode === 2) {
          const [r, g, b] = [params[++i], params[++i], params[++i]];
          if (p === 38) st.fg = `rgb(${r},${g},${b})`; else st.bg = `rgb(${r},${g},${b})`;
        } else if (mode === 5) {
          const n = params[++i];
          if (p === 38) st.fg = xterm256(n); else st.bg = xterm256(n);
        }
      }
    }
  }
  const text = line.slice(last);
  if (text) {
    const style = css(st);
    out += style ? `<span style="${style}">${escapeHtml(text)}</span>` : escapeHtml(text);
  }
  return out;
}

export function ansiToHtml(lines: string[]): string {
  return lines.map(ansiLine).join("\n");
}

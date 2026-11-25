// The one file you edit per project. Everything else reads from here.
const owner = "moneytosms";
const repoName = "jot";

export default {
  name: "jot",
  tagline: "A plaintext journal that is only files.",
  description:
    "jot is a fast, scriptable plaintext journal for the terminal: one Markdown file per day, append-only, with a calendar TUI, fuzzy search, todos and ten themes. No database, no daemon, no account.",
  owner,
  repoName,
  branch: "main",
  repoUrl: `https://github.com/${owner}/${repoName}`,
  site: `https://${owner}.github.io`,
  base: `/${repoName}`,
  footnote:
    "jot 0.7.0, MIT licensed. The journal, counts and terminal frames on this site come from a synthetic sample generated with the real binary; no personal notes were used.",
  guide: ["docs/**/*.md", "README.md", "CONTRIBUTING.md"],
  guideRoot: "README.md",
  // Sidebar groups and reading order (guide ids; pages not listed land in "More").
  nav: [
    ["Start here", ["index", "getting-started", "readme"]],
    ["Using jot", ["writing", "finding", "tui", "your-data", "configuration"]],
    ["Reference", ["commands", "troubleshooting"]],
    ["Contributing", ["development", "contributing"]],
  ],
};

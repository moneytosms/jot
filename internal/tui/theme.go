package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/moneytosms/jot"
)

// Theme is the palette the TUI renders with. Every color is a hex string
// like "#aabbcc" (empty means "let the terminal decide"); view code converts
// them with lipgloss.Color as needed.
type Theme struct {
	// Name is the theme name as requested ("auto", "gruvbox", ...).
	Name string
	// Dark reports whether the palette is tuned for a dark terminal.
	Dark bool
	// Bg is the page background and Fg the main text color.
	Bg, Fg string
	// Muted is secondary text, Dim even quieter text.
	Muted, Dim string
	// Accent highlights interactive elements; AccentFg is readable text on
	// an Accent-colored background.
	Accent, AccentFg string
	// Border colors borders, SelBg/SelFg color the selected day cell.
	Border, SelBg, SelFg string
	// Semantic colors used for calendar heat levels and markdown rendering.
	Green, Yellow, Red, Blue, Magenta, Cyan string
}

// ThemeNames lists every theme the settings panel can offer, in display
// order.
func ThemeNames() []string {
	return []string{
		"auto", "default", "light", "dark", "gruvbox", "nord", "dracula",
		"catppuccin", "tokyo-night", "omarchy",
	}
}

// Resolve turns a theme name into a concrete palette. Supported names are
// "auto" (the default), "default", "light", "gruvbox", "nord", "dracula",
// "catppuccin", "tokyo-night" and "omarchy". Unknown names fall back to the
// terminal-aware built-in palette.
func Resolve(themeName string) Theme {
	name := strings.ToLower(strings.TrimSpace(themeName))
	switch name {
	case "", "auto":
		return resolveAuto()
	case "light":
		return lightTheme()
	case "dark":
		// Fixed dark palette, regardless of the terminal background.
		t := darkTheme()
		t.Name = "dark"
		return t
	case "default":
		return builtinForTerminal()
	case "gruvbox":
		return gruvboxTheme()
	case "nord":
		return nordTheme()
	case "dracula":
		return draculaTheme()
	case "catppuccin":
		return catppuccinTheme()
	case "tokyo-night":
		return tokyoNightTheme()
	case "omarchy":
		if t, ok := omarchyTheme(""); ok {
			return t
		}
		t := darkTheme()
		t.Name = "omarchy"
		return t
	default:
		return builtinForTerminal()
	}
}

// resolveAuto picks a palette for "auto": the Omarchy theme file when it
// exists and parses, otherwise the built-in palette matching the terminal.
// NO_COLOR collapses everything to an uncolored theme.
func resolveAuto() Theme {
	if os.Getenv("NO_COLOR") != "" {
		return Theme{Name: "plain", Dark: hasDarkBackground()}
	}
	if t, ok := omarchyTheme(""); ok {
		return t
	}
	return builtinForTerminal()
}

// builtinForTerminal returns the built-in dark or light palette based on the
// terminal background.
func builtinForTerminal() Theme {
	if hasDarkBackground() {
		return darkTheme()
	}
	return lightTheme()
}

func hasDarkBackground() bool { return lipgloss.HasDarkBackground() }

// darkTheme is the built-in dark palette: warm charcoal and moss, no blue.
func darkTheme() Theme {
	return Theme{
		Name:     "default",
		Dark:     true,
		Bg:       "#141712",
		Fg:       "#d7dccf",
		Muted:    "#88907f",
		Dim:      "#5c6455",
		Accent:   "#a9c48e",
		AccentFg: "#12150f",
		Border:   "#3a4234",
		SelBg:    "#2b3326",
		SelFg:    "#e6ebdd",
		Green:    "#9ec97a",
		Yellow:   "#dcb26b",
		Red:      "#e07a7a",
		Blue:     "#8ab3a5",
		Magenta:  "#c3a3cf",
		Cyan:     "#8cc7bd",
	}
}

// lightTheme is the built-in light palette: paper, ink and forest.
func lightTheme() Theme {
	return Theme{
		Name:     "light",
		Dark:     false,
		Bg:       "#f4efe3",
		Fg:       "#1e1c16",
		Muted:    "#6a6455",
		Dim:      "#98907c",
		Accent:   "#35603b",
		AccentFg: "#f7f3e8",
		Border:   "#cec5b1",
		SelBg:    "#dde6d3",
		SelFg:    "#1e241a",
		Green:    "#3f7a45",
		Yellow:   "#8a6418",
		Red:      "#a63c3c",
		Blue:     "#3a6a60",
		Magenta:  "#7b4a6b",
		Cyan:     "#2f6a63",
	}
}

func gruvboxTheme() Theme {
	return Theme{
		Name:     "gruvbox",
		Dark:     true,
		Bg:       "#282828",
		Fg:       "#ebdbb2",
		Muted:    "#928374",
		Dim:      "#7c6f64",
		Accent:   "#fe8019",
		AccentFg: "#282828",
		Border:   "#504945",
		SelBg:    "#3c3836",
		SelFg:    "#ebdbb2",
		Green:    "#b8bb26",
		Yellow:   "#fabd2f",
		Red:      "#fb4934",
		Blue:     "#83a598",
		Magenta:  "#d3869b",
		Cyan:     "#8ec07c",
	}
}

func nordTheme() Theme {
	return Theme{
		Name:     "nord",
		Dark:     true,
		Bg:       "#2e3440",
		Fg:       "#d8dee9",
		Muted:    "#7b88a1",
		Dim:      "#5f6b7f",
		Accent:   "#88c0d0",
		AccentFg: "#2e3440",
		Border:   "#434c5e",
		SelBg:    "#434c5e",
		SelFg:    "#eceff4",
		Green:    "#a3be8c",
		Yellow:   "#ebcb8b",
		Red:      "#bf616a",
		Blue:     "#81a1c1",
		Magenta:  "#b48ead",
		Cyan:     "#8fbcbb",
	}
}

func draculaTheme() Theme {
	return Theme{
		Name:     "dracula",
		Dark:     true,
		Bg:       "#282a36",
		Fg:       "#f8f8f2",
		Muted:    "#6272a4",
		Dim:      "#565f89",
		Accent:   "#bd93f9",
		AccentFg: "#282a36",
		Border:   "#44475a",
		SelBg:    "#44475a",
		SelFg:    "#f8f8f2",
		Green:    "#50fa7b",
		Yellow:   "#f1fa8c",
		Red:      "#ff5555",
		Blue:     "#8be9fd",
		Magenta:  "#ff79c6",
		Cyan:     "#8be9fd",
	}
}

func catppuccinTheme() Theme {
	return Theme{
		Name:     "catppuccin",
		Dark:     true,
		Bg:       "#1e1e2e",
		Fg:       "#cdd6f4",
		Muted:    "#7f8498",
		Dim:      "#6c7086",
		Accent:   "#cba6f7",
		AccentFg: "#1e1e2e",
		Border:   "#45475a",
		SelBg:    "#45475a",
		SelFg:    "#cdd6f4",
		Green:    "#a6e3a1",
		Yellow:   "#f9e2af",
		Red:      "#f38ba8",
		Blue:     "#89b4fa",
		Magenta:  "#cba6f7",
		Cyan:     "#94e2d5",
	}
}

func tokyoNightTheme() Theme {
	return Theme{
		Name:     "tokyo-night",
		Dark:     true,
		Bg:       "#1a1b26",
		Fg:       "#c0caf5",
		Muted:    "#565f89",
		Dim:      "#414868",
		Accent:   "#7aa2f7",
		AccentFg: "#1a1b26",
		Border:   "#292e42",
		SelBg:    "#33467c",
		SelFg:    "#c0caf5",
		Green:    "#9ece6a",
		Yellow:   "#e0af68",
		Red:      "#f7768e",
		Blue:     "#7aa2f7",
		Magenta:  "#bb9af7",
		Cyan:     "#7dcfff",
	}
}

// omarchyTheme builds a theme from the Omarchy colors.toml. configDir
// overrides the XDG config directory; empty means the real one. A missing,
// unreadable or invalid file yields (Theme{}, false) so callers can fall back
// to a built-in palette silently.
func omarchyTheme(configDir string) (Theme, bool) {
	if strings.TrimSpace(configDir) == "" {
		dir, err := os.UserConfigDir()
		if err != nil || dir == "" {
			return Theme{}, false
		}
		configDir = dir
	}
	data, err := os.ReadFile(filepath.Join(configDir, "omarchy", "current", "theme", "colors.toml"))
	if err != nil {
		return Theme{}, false
	}
	return parseOmarchy(data)
}

// parseOmarchy turns colors.toml bytes into a theme. It expects simple
// "key = value" lines with hex values and reports false when the required
// background and foreground colors are missing or malformed.
func parseOmarchy(data []byte) (Theme, bool) {
	vals := map[string]string{}
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(strings.TrimSuffix(raw, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue // comment or blank
		}
		eq := strings.Index(line, "=")
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		if key == "" {
			continue
		}
		if v, ok := omarchyValue(strings.TrimSpace(line[eq+1:])); ok {
			vals[key] = v
		}
	}
	bg := firstHex(vals, "background")
	fg := firstHex(vals, "foreground", "bright_foreground")
	if bg == "" || fg == "" {
		return Theme{}, false
	}

	hex := func(keys ...string) string { return firstHex(vals, keys...) }
	pick := func(keys ...string) string {
		if v := hex(keys...); v != "" {
			return v
		}
		return ""
	}
	fallback := darkTheme()
	dark := vals["mode"] != "light"

	t := Theme{
		Name:    "omarchy",
		Dark:    dark,
		Bg:      bg,
		Fg:      fg,
		Muted:   pick("muted"),
		Accent:  pick("accent", "blue"),
		Border:  pick("muted", "dark_foreground"),
		SelBg:   pick("selection", "muted"),
		Dim:     pick("dark_foreground", "muted"),
		Green:   pick("green", "bright_green"),
		Yellow:  pick("yellow", "bright_yellow"),
		Red:     pick("red", "bright_red"),
		Blue:    pick("blue", "bright_blue"),
		Magenta: pick("magenta", "bright_magenta"),
		Cyan:    pick("cyan", "bright_cyan"),
	}
	// Fill anything the file does not define from the built-in dark palette
	// so the TUI never renders with missing colors.
	if t.Muted == "" {
		t.Muted = fallback.Muted
	}
	if t.Border == "" {
		t.Border = fallback.Border
	}
	if t.SelBg == "" {
		t.SelBg = fallback.SelBg
	}
	if t.Dim == "" {
		t.Dim = fallback.Dim
	}
	for _, kv := range []struct {
		dst, alt *string
		fb       string
	}{
		{&t.Accent, nil, fallback.Accent},
		{&t.Green, nil, fallback.Green},
		{&t.Yellow, nil, fallback.Yellow},
		{&t.Red, nil, fallback.Red},
		{&t.Blue, nil, fallback.Blue},
		{&t.Magenta, nil, fallback.Magenta},
		{&t.Cyan, nil, fallback.Cyan},
	} {
		if *kv.dst == "" {
			*kv.dst = kv.fb
		}
	}
	// Pick readable foreground colors for accent and selection backgrounds.
	t.AccentFg = contrastPick(t.Accent, fg, bg)
	t.SelFg = contrastPick(t.SelBg, fg, bg)
	return t, true
}

// omarchyValue extracts the value part of a "key = value" line: a quoted
// string (with an optional trailing comment) or a bare token up to the first
// '#' or whitespace.
func omarchyValue(s string) (string, bool) {
	if s == "" {
		return "", false
	}
	if s[0] == '"' || s[0] == '\'' {
		if end := strings.IndexByte(s[1:], s[0]); end >= 0 {
			return s[1 : 1+end], true
		}
		return "", false
	}
	if i := strings.IndexAny(s, "# \t"); i >= 0 {
		s = s[:i]
	}
	return s, s != ""
}

// firstHex returns the first value among keys that is a well-formed hex
// color.
func firstHex(vals map[string]string, keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(vals[k]); hexOK(v) {
			return strings.ToLower(v)
		}
	}
	return ""
}

// hexOK reports whether s looks like #rgb or #rrggbb.
func hexOK(s string) bool {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "#") {
		return false
	}
	switch len(s) {
	case 4, 7:
		for _, r := range s[1:] {
			if !strings.ContainsRune("0123456789abcdefABCDEF", r) {
				return false
			}
		}
		return true
	}
	return false
}

// parseHex decodes #rgb or #rrggbb into 8-bit channels.
func parseHex(s string) (r, g, b int, ok bool) {
	if !hexOK(s) {
		return 0, 0, 0, false
	}
	if len(s) == 4 {
		return int(dig(s[1]) * 17), int(dig(s[2]) * 17), int(dig(s[3]) * 17), true
	}
	return dig2(s[1:3]), dig2(s[3:5]), dig2(s[5:7]), true
}

// dig converts one hex digit byte to its value.
func dig(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	default:
		return c - 'A' + 10
	}
}

// dig2 converts a two-byte hex pair to its value.
func dig2(s string) int { return int(dig(s[0]))*16 + int(dig(s[1])) }

// hexLuminance approximates the relative luminance of a hex color, 0 (black)
// to 1 (white). Malformed colors are treated as black.
func hexLuminance(hex string) float64 {
	r, g, b, ok := parseHex(hex)
	if !ok {
		return 0
	}
	return (0.2126*float64(r) + 0.7152*float64(g) + 0.0722*float64(b)) / 255
}

// contrastPick returns whichever of a and b contrasts more with against.
func contrastPick(against, a, b string) string {
	l := hexLuminance(against)
	if abs(hexLuminance(a)-l) >= abs(hexLuminance(b)-l) {
		return a
	}
	return b
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

// mdStyles converts the theme into the ANSI styles RenderStyled understands.
// Colors become 24-bit escape prefixes; an empty color disables that style.
func (t Theme) mdStyles() jot.MDStyles {
	tc := func(hex string) string {
		r, g, b, ok := parseHex(hex)
		if !ok {
			return ""
		}
		return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", r, g, b)
	}
	return jot.MDStyles{
		Heading1:   "\x1b[1m" + tc(t.Fg),
		Heading2:   "\x1b[1m" + tc(t.Cyan),
		Heading3:   tc(t.Cyan),
		Bold:       "\x1b[1m",
		Italic:     "\x1b[3m",
		Code:       tc(t.Yellow),
		Strike:     "\x1b[9m",
		Quote:      tc(t.Dim),
		Dim:        "\x1b[2m",
		ListBullet: tc(t.Cyan),
		Accent:     "\x1b[1m" + tc(t.Magenta),
	}
}

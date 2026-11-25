package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeOmarchyFixture writes a colors.toml under dir (as the omarchy layout
// expects) and returns the theme the parser builds from it.
func writeOmarchyFixture(t *testing.T, dir, contents string) (Theme, bool) {
	t.Helper()
	themeDir := filepath.Join(dir, "omarchy", "current", "theme")
	if err := os.MkdirAll(themeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(themeDir, "colors.toml"), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	return omarchyTheme(dir)
}

const omarchyDark = `
# Omarchy theme export
mode = "dark"

accent = "#b59790"
selection = "#584e51"
muted = "#584e51"

background = "#0c0b0c"
dark_background = "#090809"

foreground = "#FAFCFB"
light_foreground = "#cfd3cd"
bright_foreground = "#e2dddc"

red = "#c38b7b"
yellow = "#6b5e73"
green = "#87a9b0"
cyan = "#a5a0b6"
blue = "#b59790"
magenta = "#c4d8e2"
`

func TestOmarchyThemeParsing(t *testing.T) {
	theme, ok := writeOmarchyFixture(t, t.TempDir(), omarchyDark)
	if !ok {
		t.Fatal("expected the fixture to parse")
	}
	if !theme.Dark {
		t.Error("theme.Dark = false, want true")
	}
	if theme.Name != "omarchy" {
		t.Errorf("theme.Name = %q", theme.Name)
	}
	want := map[string]string{
		"Bg": theme.Bg, "Fg": theme.Fg, "Muted": theme.Muted,
		"Accent": theme.Accent, "SelBg": theme.SelBg,
		"Red": theme.Red, "Green": theme.Green, "Blue": theme.Blue,
	}
	for name, got := range want {
		if got == "" {
			t.Errorf("theme.%s is empty", name)
		}
		if got != "" && !strings.HasPrefix(got, "#") {
			t.Errorf("theme.%s = %q, want a hex color", name, got)
		}
	}
	if theme.Bg != "#0c0b0c" {
		t.Errorf("theme.Bg = %q", theme.Bg)
	}
	if theme.Fg != "#fafcfb" {
		t.Errorf("theme.Fg = %q (want lowercased)", theme.Fg)
	}
	// Foregrounds on accent/selection must be readable: high contrast.
	if hexLuminance(theme.AccentFg)-hexLuminance(theme.Accent) < 0.15 &&
		hexLuminance(theme.Accent)-hexLuminance(theme.AccentFg) < 0.15 {
		t.Errorf("AccentFg %q has too little contrast with Accent %q", theme.AccentFg, theme.Accent)
	}
	if theme.SelFg == "" {
		t.Error("theme.SelFg is empty")
	}
	// The fixture's "yellow" is a dim grey next to muted, so busy calendar
	// days would look empty; it falls back to red. Green is distinct and kept.
	if theme.Yellow != "#c38b7b" {
		t.Errorf("theme.Yellow = %q, want the red fallback #c38b7b", theme.Yellow)
	}
	if theme.Green != "#87a9b0" {
		t.Errorf("theme.Green = %q, want the theme's own #87a9b0", theme.Green)
	}
}

func TestOmarchyThemeLightMode(t *testing.T) {
	theme, ok := writeOmarchyFixture(t, t.TempDir(), strings.Replace(omarchyDark, `mode = "dark"`, `mode = "light"`, 1))
	if !ok {
		t.Fatal("expected the fixture to parse")
	}
	if theme.Dark {
		t.Error("theme.Dark = true, want false for mode = light")
	}
}

func TestOmarchyThemeFillsMissingColors(t *testing.T) {
	minimal := "background = \"#101010\"\nforeground = \"#eeeeee\"\n"
	theme, ok := writeOmarchyFixture(t, t.TempDir(), minimal)
	if !ok {
		t.Fatal("expected bg/fg-only fixture to parse")
	}
	for name, v := range map[string]string{
		"Accent": theme.Accent, "Muted": theme.Muted, "Border": theme.Border,
		"Green": theme.Green, "Yellow": theme.Yellow, "Red": theme.Red,
		"Blue": theme.Blue, "Magenta": theme.Magenta, "Cyan": theme.Cyan,
	} {
		if v == "" {
			t.Errorf("theme.%s left empty without a fallback", name)
		}
	}
}

func TestOmarchyThemeInvalidAndMissing(t *testing.T) {
	if _, ok := writeOmarchyFixture(t, t.TempDir(), "not a toml file at all\n=== ===\n"); ok {
		t.Error("garbage fixture should not parse")
	}
	if _, ok := writeOmarchyFixture(t, t.TempDir(), "background = \"zzz\"\nforeground = \"#ffffff\"\n"); ok {
		t.Error("invalid hex background should not parse")
	}
	// Missing file falls back silently.
	if _, ok := omarchyTheme(t.TempDir()); ok {
		t.Error("missing colors.toml should not parse")
	}
}

func TestResolveNamedThemes(t *testing.T) {
	if theme := Resolve("gruvbox"); !theme.Dark || theme.Bg != "#282828" || theme.Fg == "" {
		t.Errorf("Resolve(gruvbox) = %+v", theme)
	}
	if theme := Resolve("tokyo-night"); !theme.Dark || theme.Bg != "#1a1b26" {
		t.Errorf("Resolve(tokyo-night) = %+v", theme)
	}
	if theme := Resolve("catppuccin"); !theme.Dark || theme.Accent == "" {
		t.Errorf("Resolve(catppuccin) = %+v", theme)
	}
	if theme := Resolve("light"); theme.Dark || theme.Fg == "" {
		t.Errorf("Resolve(light) = %+v", theme)
	}
	// Unknown names fall back to a usable palette.
	theme := Resolve("no-such-theme")
	if theme.Fg == "" || theme.Bg == "" {
		t.Errorf("unknown theme fallback missing colors: %+v", theme)
	}
	if _, _, _, ok := parseHex(theme.Fg); !ok {
		t.Errorf("unknown theme fallback Fg = %q is not hex", theme.Fg)
	}
}

func TestResolveAutoNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	theme := Resolve("auto")
	if theme.Name != "plain" {
		t.Errorf("Resolve(auto) with NO_COLOR name = %q, want plain", theme.Name)
	}
	if theme.Fg != "" || theme.Accent != "" {
		t.Errorf("Resolve(auto) with NO_COLOR should leave colors empty, got %+v", theme)
	}
}

func TestThemeNames(t *testing.T) {
	names := ThemeNames()
	want := []string{"auto", "default", "light", "dark", "gruvbox", "nord", "dracula", "catppuccin", "tokyo-night", "omarchy"}
	if len(names) != len(want) {
		t.Fatalf("ThemeNames() = %v", names)
	}
	for i, n := range want {
		if names[i] != n {
			t.Errorf("ThemeNames()[%d] = %q, want %q", i, names[i], n)
		}
	}
}

func TestMdStylesFromTheme(t *testing.T) {
	s := darkTheme().mdStyles()
	if s.Heading2 == "" || s.Code == "" {
		t.Fatalf("mdStyles missing entries: %+v", s)
	}
	if !strings.Contains(s.Code, "\x1b[38;2;") {
		t.Errorf("Code style %q is not truecolor", s.Code)
	}
	// A theme without colors emits no color sequences: bold/italic markers
	// remain, but every truecolor prefix disappears.
	plain := (Theme{}).mdStyles()
	for name, v := range map[string]string{
		"Heading2": plain.Heading2, "Heading3": plain.Heading3,
		"Code": plain.Code, "Quote": plain.Quote,
		"ListBullet": plain.ListBullet, "Accent": plain.Accent,
	} {
		if strings.Contains(v, "38;2;") {
			t.Errorf("plain %s = %q still colors text", name, v)
		}
	}
}
func TestResolveDarkIsFixed(t *testing.T) {
	theme := Resolve("dark")
	if theme.Name != "dark" {
		t.Fatalf("Name = %q, want dark", theme.Name)
	}
	if !theme.Dark {
		t.Fatal("dark theme reports a light background")
	}
	if theme.Bg != darkTheme().Bg || theme.Accent != darkTheme().Accent {
		t.Fatalf("dark palette differs from the built-in dark colors: %+v", theme)
	}
}

package tui

import "testing"

func TestANSIWidthSkipsEscapes(t *testing.T) {
	cases := []struct {
		name, in string
		want     int
	}{
		{"plain", "abc", 3},
		{"sgr", "\x1b[31mred\x1b[0m", 3},
		{"truecolor", "\x1b[1m\x1b[38;2;125;207;255m12:00:00\x1b[0m", 8},
		{"erase-line", "\x1b[2Kabc", 3},
		{"non-csi", "\x1bMabc", 3},
	}
	for _, c := range cases {
		if got := ansiWidth(c.in); got != c.want {
			t.Errorf("ansiWidth(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

// padRight must pad coloured text to the requested columns; an over-counting
// width makes it skip padding and shifts the day pane left.
func TestPadRightWithColor(t *testing.T) {
	line := padRight("\x1b[38;2;154;206;106m 2 \x1b[0m", calW)
	if got := ansiWidth(line); got != calW {
		t.Fatalf("padded width = %d, want %d (%q)", got, calW, line)
	}
}

func TestANSITruncateKeepsEscapes(t *testing.T) {
	got := ansiTruncate("\x1b[31mabcdef\x1b[0m", 3)
	if want := "\x1b[31mabc"; got != want {
		t.Errorf("ansiTruncate() = %q, want %q", got, want)
	}
	if w := ansiWidth(got); w != 3 {
		t.Errorf("truncated width = %d, want 3", w)
	}
}

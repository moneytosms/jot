package main

import (
	"bufio"
	"bytes"
	"strings"
	"testing"
)

// TestTUIWiringBareNonTerminal checks that bare jot keeps printing the help
// page when its streams are not terminals.
func TestTUIWiringBareNonTerminal(t *testing.T) {
	testEnv(t)
	code, out, errw := runCLI(t, "")
	if code != 0 {
		t.Fatalf("bare jot exited %d: %q", code, errw)
	}
	for _, want := range []string{"Usage:", "tui"} {
		if !strings.Contains(out, want) {
			t.Errorf("help missing %q:\n%s", want, out)
		}
	}
}

// TestTUIWiringRefusesNonTerminal makes sure `jot tui` fails fast instead of
// starting a Bubble Tea program on buffered streams.
func TestTUIWiringRefusesNonTerminal(t *testing.T) {
	testEnv(t)
	var out, errw bytes.Buffer
	bw := bufio.NewWriter(&out)
	code := run([]string{"tui"}, bufio.NewReader(strings.NewReader("")), bw, &errw)
	if err := bw.Flush(); err != nil {
		t.Fatal(err)
	}
	if code != 1 {
		t.Fatalf("expected exit 1, got %d (%q)", code, errw.String())
	}
	if !strings.Contains(errw.String(), "interactive terminal") {
		t.Fatalf("error does not mention a terminal: %q", errw.String())
	}
}

func TestTUIWiringHelpPage(t *testing.T) {
	testEnv(t)
	code, out, errw := runCLI(t, "", "help", "tui")
	if code != 0 {
		t.Fatalf("help tui exited %d: %q", code, errw)
	}
	for _, want := range []string{"jot tui", "?", "calendar"} {
		if !strings.Contains(out, want) {
			t.Errorf("tui help missing %q:\n%s", want, out)
		}
	}
}

func TestTUIWiringTopLevelHelp(t *testing.T) {
	testEnv(t)
	code, out, errw := runCLI(t, "", "-h")
	if code != 0 {
		t.Fatalf("-h exited %d: %q", code, errw)
	}
	if !strings.Contains(out, "tui        Open the interactive interface") {
		t.Errorf("overview help does not list tui:\n%s", out)
	}
}

func TestTUIWiringUnknownFlag(t *testing.T) {
	testEnv(t)
	code, out, errw := runCLI(t, "", "tui", "--bogus")
	if code != 2 {
		t.Fatalf("expected exit 2, got %d (%q)", code, errw)
	}
	for _, want := range []string{"jot tui", "--theme"} {
		if !strings.Contains(out, want) {
			t.Errorf("flag error did not print the tui help page, missing %q:\n%s", want, out)
		}
	}
}

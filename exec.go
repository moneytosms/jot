package jot

import (
	"bytes"
	"os/exec"
	"strings"
)

// lookPath reports whether name is an executable available on PATH.
func lookPath(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// runCapture runs cmd, capturing stdout and stderr. The returned strings are
// not trimmed; callers decide what to show.
func runCapture(cmd *exec.Cmd) (stdout, stderr string, err error) {
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	err = cmd.Run()
	return out.String(), errBuf.String(), err
}

// shQuote single-quotes s for a POSIX sh command line. Only ever used on
// paths jot built itself, never on user text that reaches a shell elsewhere.
func shQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

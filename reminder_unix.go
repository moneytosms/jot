//go:build !windows

package jot

import (
	"os/exec"
	"runtime"
	"strconv"
	"syscall"
)

// backgroundNotifier names what a background reminder needs, for errors.
var backgroundNotifier = map[bool]string{true: "sh and osascript", false: "sh and notify-send"}[runtime.GOOS == "darwin"]

// The scripts get the delay and stash path as positional arguments, never
// interpolated. A missing stash file means the reminder was cleared.
const (
	bgWait   = `sleep "$1"; [ -f "$2" ] || exit 0; m=$(cat "$2"); rm -f "$2"; `
	bgLinux  = bgWait + `notify-send -a jot -- jot "$m"`
	bgDarwin = bgWait + `osascript -e 'on run argv' -e 'display notification (item 1 of argv) with title "jot"' -e 'end run' "$m"`
)

func backgroundSupported() bool {
	if runtime.GOOS == "darwin" {
		return lookPath("sh") && lookPath("osascript")
	}
	return lookPath("sh") && lookPath("notify-send")
}

func backgroundCommand(secs int, msgFile string) *exec.Cmd {
	script := bgLinux
	if runtime.GOOS == "darwin" {
		script = bgDarwin
	}
	return exec.Command("sh", "-c", script, "sh", strconv.Itoa(secs), msgFile)
}

// detach starts the process in its own session so closing the terminal or
// quitting the TUI does not take it down.
func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}

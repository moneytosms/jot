//go:build windows

package jot

import (
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

const backgroundNotifier = "powershell"

// bgPowerShell reads the delay and stash path from the environment, so no
// value is interpolated into the script. A missing stash file means the
// reminder was cleared.
const bgPowerShell = `Start-Sleep -Seconds $env:JOT_REMIND_SECS
$f = $env:JOT_REMIND_FILE
if (-not (Test-Path -LiteralPath $f)) { exit }
$m = Get-Content -Raw -LiteralPath $f
Remove-Item -LiteralPath $f
Add-Type -AssemblyName System.Windows.Forms, System.Drawing
$n = New-Object System.Windows.Forms.NotifyIcon
$n.Icon = [System.Drawing.SystemIcons]::Information
$n.Visible = $true
$n.ShowBalloonTip(10000, 'jot', $m, 'Info')
Start-Sleep -Seconds 10
$n.Dispose()`

func backgroundSupported() bool { return lookPath("powershell") }

func backgroundCommand(secs int, msgFile string) *exec.Cmd {
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", bgPowerShell)
	cmd.Env = append(os.Environ(), "JOT_REMIND_SECS="+strconv.Itoa(secs), "JOT_REMIND_FILE="+msgFile)
	return cmd
}

// detach runs the process without a console and outside jot's process group.
func detach(cmd *exec.Cmd) {
	const detachedProcess, newProcessGroup = 0x00000008, 0x00000200
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: detachedProcess | newProcessGroup}
}

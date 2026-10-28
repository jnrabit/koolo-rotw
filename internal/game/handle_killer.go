package game

import (
	"log/slog"
	"os/exec"
	"strings"
	"syscall"
)

// KillAllClientHandles closes duplicated "Check For Other Instances" handles so
// the game can be started more than once. Under Wine this is not needed and the
// Sysinternals tool cannot enumerate handles, so a failure is non fatal.
func KillAllClientHandles() error {
	cmd := exec.Command("./tools/handle64.exe", "-accepteula", "-nobanner", "-a", "-v", "-p", "d2r.exe", "Check For Other Instances")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	stdout, err := cmd.Output()
	if err != nil {
		if strings.Contains(string(stdout), "No matching handles found.") {
			return nil
		}
		slog.Warn("handle64.exe unavailable, skipping duplicate instance handle cleanup", "error", err)
		return nil
	}

	stdoutLines := strings.Split(string(stdout), "\r\n")

	// Maybe the handle was previously closed, so we don't need to do anything
	if len(stdoutLines) > 1 {
		for _, line := range stdoutLines {
			if strings.Contains(line, "Check For Other Instances") {
				cols := strings.Split(line, ",") // 0: process, 1: pid, 2: type, 3: handle, 4: name
				if len(cols) < 4 {
					continue
				}

				cmd = exec.Command("./tools/handle64.exe", "-accepteula", "-nobanner", "-p", cols[1], "-c", cols[3], "-y")
				cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
				stdout, err = cmd.Output()
				if err != nil {
					slog.Warn("handle64.exe could not close handle, continuing", "error", err)
					return nil
				}
			}
		}
	}

	return nil
}

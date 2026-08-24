//go:build unix

package runner

import (
	"os/exec"
	"syscall"
)

// ownProcessGroup + killGroup (war story #1 in HANDOFF): agents spawn
// children that inherit the pipes; killing only the parent leaves the
// pipe open and the scanner blocked. Put the agent in its own process
// group so we can SIGKILL the whole tree.
func setProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func killGroup(pid int) error {
	return syscall.Kill(-pid, syscall.SIGKILL)
}

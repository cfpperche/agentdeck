//go:build windows

package runner

import (
	"os"
	"os/exec"
)

// Windows has no Setpgid; JobObjects would be the faithful equivalent —
// out of scope. Kill the direct process as the best available effort.
func setProcAttr(cmd *exec.Cmd) {}

func killGroup(pid int) error {
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return p.Kill()
}

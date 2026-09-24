//go:build unix

package runner

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// setProcessGroup starts cmd in a process group of its own and makes a
// cancelled context stop the whole group with SIGTERM, so the CLI can still
// flush its transcript; finishCLI kills whatever outlives it.
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return signalGroup(cmd, syscall.SIGTERM) }
}

// killProcessGroup kills the process group of a started cmd.
func killProcessGroup(cmd *exec.Cmd) error {
	return signalGroup(cmd, syscall.SIGKILL)
}

// signalGroup signals the process group of a started cmd. A group already gone
// is os.ErrProcessDone, which exec.Cmd.Cancel takes for "exited on its own".
func signalGroup(cmd *exec.Cmd, sig syscall.Signal) error {
	if cmd.Process == nil {
		return os.ErrProcessDone
	}
	err := syscall.Kill(-cmd.Process.Pid, sig)
	if errors.Is(err, syscall.ESRCH) {
		return os.ErrProcessDone
	}
	return err
}

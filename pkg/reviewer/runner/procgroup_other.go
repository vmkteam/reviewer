//go:build !unix

package runner

import (
	"os"
	"os/exec"
)

// setProcessGroup is a no-op without Unix process groups: a cancelled context
// kills the runner CLI alone.
func setProcessGroup(*exec.Cmd) {}

func killProcessGroup(*exec.Cmd) error { return os.ErrProcessDone }

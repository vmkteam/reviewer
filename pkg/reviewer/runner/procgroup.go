package runner

import (
	"context"
	"errors"
	"log/slog"
	"os/exec"
	"path/filepath"
	"time"
)

// cliWaitDelay bounds how long a runner CLI is awaited once its context is done
// (then it is killed) and how long its output pipes are once it exited.
var cliWaitDelay = 5 * time.Second

// prepareCLI makes a runner CLI command stoppable as a whole: a cancelled
// context stops its process group — the CLI together with what it spawned (a
// shell tool, a test run) — and a leftover child that still holds the output
// pipes is not awaited past cliWaitDelay.
func prepareCLI(cmd *exec.Cmd) {
	setProcessGroup(cmd)
	cmd.WaitDelay = cliWaitDelay
}

// finishCLI kills whatever the CLI left running in its process group (a
// background shell, a dev server) and returns the run's error — nil for
// exec.ErrWaitDelay: the CLI itself succeeded, only a leftover child held its
// pipes open. A process that left the group (setsid) is out of reach.
func finishCLI(ctx context.Context, log *slog.Logger, cmd *exec.Cmd, err error) error {
	name := filepath.Base(cmd.Path)
	if killProcessGroup(cmd) == nil {
		log.WarnContext(ctx, "killed processes "+name+" left running")
	}
	if errors.Is(err, exec.ErrWaitDelay) {
		log.WarnContext(ctx, name+" exited, but a process it left running held its output open")
		return nil
	}
	return err
}

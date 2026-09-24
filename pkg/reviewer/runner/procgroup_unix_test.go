//go:build unix

package runner

import (
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunExecKillsWhatTheCLILeftRunning(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "pid")
	// The background sleep lets go of the pipes: only the group kill ends it.
	script := "sleep 60 >/dev/null 2>&1 & echo $! >" + pidFile + "; echo done"
	out := runExec(t.Context(), slog.New(slog.DiscardHandler), "sh", t.TempDir(), []string{"-c", script}, "", nil, nil)
	require.NoError(t, out.err)

	data, err := os.ReadFile(pidFile)
	require.NoError(t, err)
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	require.NoError(t, err)
	t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })
	assert.Eventually(t, func() bool { return syscall.Kill(pid, 0) != nil }, 5*time.Second, 20*time.Millisecond,
		"the leftover process is killed with the run")
}

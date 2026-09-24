package runner

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"reviewsrv/pkg/reviewer"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeCLI puts an executable sh script called name first on PATH.
func fakeCLI(t *testing.T, name, script string) {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+script+"\n"), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestExecCodexRunnerTagsAPIFailure(t *testing.T) {
	for _, tt := range []struct{ name, script, want string }{
		{"billing from the error event",
			`echo '{"type":"turn.failed","error":{"message":"You exceeded your current quota: insufficient_quota"}}'; exit 1`,
			reviewer.RunReasonBilling},
		{"auth from stderr when nothing streamed",
			`echo 'unexpected status 401 Unauthorized' >&2; exit 1`,
			reviewer.RunReasonAuth},
		{"a cancelled job stays cancelled",
			`echo '{"type":"turn.failed","error":{"message":"insufficient_quota"}}'; exit 143`,
			reviewer.RunReasonCancelled},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fakeCLI(t, "codex", tt.script)
			r := &ExecCodexRunner{Dir: t.TempDir(), Log: slog.New(slog.DiscardHandler)}
			_, err := r.Run(t.Context(), "review")
			require.Error(t, err)
			_, reason := reviewer.RunOutcome(err)
			assert.Equal(t, tt.want, reason)
		})
	}
}

func TestExecOpenCodeRunnerTagsAPIFailure(t *testing.T) {
	for _, tt := range []struct {
		name, script, want string
		partial            bool
	}{
		{"auth from stderr", `echo 'Error: 401 Unauthorized' >&2; exit 1`, reviewer.RunReasonAuth, false},
		{"rate limit after some steps",
			`echo '{"type":"step_finish","sessionID":"s","part":{"reason":"tool-calls","tokens":{"input":1,"output":1}}}'
echo 'AI_APICallError: statusCode: 429' >&2; exit 1`,
			reviewer.RunReasonRateLimit, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fakeCLI(t, "opencode", tt.script)
			dir := t.TempDir()
			require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
			r := &ExecOpenCodeRunner{Dir: dir, Log: slog.New(slog.DiscardHandler)}
			cr, err := r.Run(t.Context(), "review")
			require.Error(t, err)
			assert.Equal(t, tt.partial, cr != nil, "a parsed partial result is kept")
			_, reason := reviewer.RunOutcome(err)
			assert.Equal(t, tt.want, reason)
		})
	}
}

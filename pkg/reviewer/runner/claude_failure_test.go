package runner

import (
	"context"
	"errors"
	"log/slog"
	"os/exec"
	"strings"
	"testing"
	"time"

	"reviewsrv/pkg/reviewer"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Trimmed from real CI run outputs: the
// synthetic assistant event Claude Code emits for a failed API call, then the
// is_error result.
const (
	claudeBillingStream = `{"type":"system","subtype":"init","session_id":"s1","model":"claude-opus-4-8"}
{"type":"assistant","message":{"model":"<synthetic>","role":"assistant","content":[{"type":"text","text":"Credit balance is too low"}]},"session_id":"s1","error":"billing_error"}
{"type":"result","subtype":"success","is_error":true,"api_error_status":400,"duration_ms":943,"num_turns":1,"result":"Credit balance is too low","session_id":"s1","total_cost_usd":0,"usage":{}}
`
	claudeServerErrorStream = `{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Read","input":{"file_path":"a.go"}}]},"session_id":"s2"}
{"type":"assistant","message":{"model":"<synthetic>","role":"assistant","content":[{"type":"text","text":"API Error: Internal server error"}]},"session_id":"s2","error":"unknown"}
{"type":"result","subtype":"success","is_error":true,"api_error_status":null,"num_turns":29,"result":"API Error: Internal server error","session_id":"s2","total_cost_usd":1.81118025,"usage":{}}
`
)

// exitErr runs a shell that exits with code, yielding a real *exec.ExitError.
func exitErr(t *testing.T, script string) error {
	t.Helper()
	err := exec.CommandContext(t.Context(), "sh", "-c", script).Run()
	require.Error(t, err)
	return err
}

func TestHandleClaudeErrorBilling(t *testing.T) {
	r := &ExecClaudeRunner{}
	cr, err := r.handleClaudeError(exitErr(t, "exit 1"), []byte(claudeBillingStream), "")
	require.NotNil(t, cr)
	require.Error(t, err)
	// The cause leads the message: the debug list shows only its first 80 chars.
	assert.True(t, strings.HasPrefix(err.Error(), "billing_error: Credit balance is too low (api 400, claude exit status 1)"), err.Error())
	status, reason := reviewer.RunOutcome(err)
	assert.Equal(t, reviewer.RunStatusFailed, status)
	assert.Equal(t, reviewer.RunReasonBilling, reason)
}

func TestHandleClaudeErrorServerError(t *testing.T) {
	r := &ExecClaudeRunner{}
	cr, err := r.handleClaudeError(exitErr(t, "exit 1"), []byte(claudeServerErrorStream), "")
	require.NotNil(t, cr)
	assert.InDelta(t, 1.81118025, cr.TotalCostUSD, 1e-9, "the failed run's cost must survive for the debug bundle")
	assert.Equal(t, "API Error: Internal server error (claude exit status 1)", err.Error())
	_, reason := reviewer.RunOutcome(err)
	assert.Equal(t, reviewer.RunReasonAPIError, reason)
}

func TestHandleClaudeErrorKeepsErrorSubtypeResult(t *testing.T) {
	// error_* subtypes parse with an error; the result (and its cost) is kept.
	stream := `{"type":"result","subtype":"error_max_turns","is_error":true,"result":"","total_cost_usd":2.5,"usage":{}}`
	r := &ExecClaudeRunner{}
	cr, err := r.handleClaudeError(exitErr(t, "exit 1"), []byte(stream), "boom")
	require.NotNil(t, cr)
	assert.InDelta(t, 2.5, cr.TotalCostUSD, 1e-9)
	assert.Contains(t, err.Error(), "claude exited with error: exit status 1 (stderr: boom)")
}

func TestHandleClaudeErrorNoOutput(t *testing.T) {
	r := &ExecClaudeRunner{}
	cr, err := r.handleClaudeError(exitErr(t, "exit 1"), nil, "segfault")
	assert.Nil(t, cr)
	assert.Equal(t, "claude exited with error: exit status 1 (stderr: segfault)", err.Error())
	_, reason := reviewer.RunOutcome(err)
	assert.Equal(t, reviewer.RunReasonOther, reason)
}

func TestHandleClaudeErrorInterrupted(t *testing.T) {
	r := &ExecClaudeRunner{}
	// A cancelled CI job: Claude Code handles SIGTERM and exits 143, mid-run;
	// runExec tags such an exit as cancelled.
	exit := reviewer.WithRunReason(reviewer.RunReasonCancelled, exitErr(t, "exit 143"))
	_, err := r.handleClaudeError(exit, []byte(`{"type":"system","subtype":"thinking_tokens"}`), "")
	status, _ := reviewer.RunOutcome(err)
	assert.Equal(t, reviewer.RunStatusCancelled, status)
	assert.Contains(t, err.Error(), "interrupted")
}

func TestIsInterrupted(t *testing.T) {
	assert.True(t, isInterrupted(exitErr(t, "exit 143")))
	assert.True(t, isInterrupted(exitErr(t, "exit 130")))
	assert.True(t, isInterrupted(exitErr(t, "kill -TERM $$")))
	assert.False(t, isInterrupted(exitErr(t, "kill -KILL $$")), "SIGKILL is our own timeout kill, not a job cancel")
	assert.False(t, isInterrupted(exitErr(t, "exit 1")))
	assert.False(t, isInterrupted(errors.New("boom")))
	assert.False(t, isInterrupted(nil))
}

func TestRunExecTagsContextCause(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	out := runExec(ctx, slog.New(slog.DiscardHandler), "sleep", t.TempDir(), []string{"5"}, "", nil, nil)
	require.Error(t, out.err)
	status, _ := reviewer.RunOutcome(out.err)
	assert.Equal(t, reviewer.RunStatusCancelled, status)
}

// setWaitDelay overrides cliWaitDelay for one test.
func setWaitDelay(t *testing.T, d time.Duration) {
	t.Helper()
	old := cliWaitDelay
	cliWaitDelay = d
	t.Cleanup(func() { cliWaitDelay = old })
}

func TestRunExecCancelKillsLeftoverChildren(t *testing.T) {
	setWaitDelay(t, time.Minute) // only the process-group kill ends the run in time
	ctx, cancel := context.WithTimeout(t.Context(), 300*time.Millisecond)
	defer cancel()

	start := time.Now()
	// The background sleep inherits the CLI's stdout and would hold it open.
	out := runExec(ctx, slog.New(slog.DiscardHandler), "sh", t.TempDir(), []string{"-c", "sleep 60 & sleep 60"}, "", nil, nil)
	assert.Less(t, time.Since(start), 10*time.Second)
	status, _ := reviewer.RunOutcome(out.err)
	assert.Equal(t, reviewer.RunStatusTimeout, status)
}

func TestRunExecDoesNotWaitForLeftoverChild(t *testing.T) {
	setWaitDelay(t, 100*time.Millisecond)

	start := time.Now()
	out := runExec(t.Context(), slog.New(slog.DiscardHandler), "sh", t.TempDir(), []string{"-c", "sleep 60 & echo done"}, "", nil, nil)
	require.NoError(t, out.err, "the CLI itself succeeded")
	assert.Equal(t, "done\n", out.stdout.String())
	assert.Less(t, time.Since(start), 10*time.Second)
}

func TestClaudeReason(t *testing.T) {
	tests := []struct {
		kind   string
		status int
		text   string
		want   string
	}{
		{claudeErrBilling, 400, "Credit balance is too low", reviewer.RunReasonBilling},
		{claudeErrAuth, 0, "", reviewer.RunReasonAuth},
		{claudeErrUnknown, 401, "", reviewer.RunReasonAuth},
		{claudeErrRateLimit, 0, "", reviewer.RunReasonRateLimit},
		{claudeErrServer, 0, "", reviewer.RunReasonAPIError},
		{claudeErrUnknown, 529, "", reviewer.RunReasonAPIError},
		{claudeErrUnknown, 0, "API Error: Internal server error", reviewer.RunReasonAPIError},
		{claudeErrUnknown, 0, "API Error: Connection error.", reviewer.RunReasonAPIError},
		{claudeErrUnknown, 429, "Your credit balance is too low", reviewer.RunReasonBilling},
		{"invalid_request", 400, "prompt is too long", reviewer.RunReasonOther},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, claudeReason(tt.kind, tt.status, tt.text), "%s/%d/%s", tt.kind, tt.status, tt.text)
	}
}

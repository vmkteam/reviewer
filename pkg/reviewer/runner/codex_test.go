package runner

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// parseCodexResult folds a codex `--json` stream of a fresh (non-resumed) run.
func parseCodexResult(data []byte, model string) *ClaudeResult {
	return parseCodexStream(data).toClaudeResult(model, codexUsage{})
}

func TestParseCodexResult(t *testing.T) {
	stream := `{"type":"thread.started","thread_id":"th_abc"}
{"type":"item.started","item":{"type":"command_execution","command":"git diff"}}
{"type":"item.completed","item":{"type":"agent_message","text":"done"}}
{"type":"turn.completed","usage":{"input_tokens":1000,"cached_input_tokens":200,"output_tokens":50}}
`
	cr := parseCodexResult([]byte(stream), "gpt-6-sol")
	require.Equal(t, "th_abc", cr.SessionID)
	require.Equal(t, "done", cr.Result)
	require.Equal(t, 1, cr.NumTurns)
	require.Equal(t, 800, cr.Usage.InputTokens) // 1000 total - 200 cached
	require.Equal(t, 200, cr.Usage.CacheReadInputTokens)
	require.Equal(t, 50, cr.Usage.OutputTokens)
	require.False(t, cr.IsError)
	// (800*2 + 200*0.2 + 50*10) / 1e6
	require.InDelta(t, 0.00214, cr.TotalCostUSD, 1e-9)
	require.Contains(t, cr.ModelUsage, "gpt-6-sol")
}

func TestParseCodexResultError(t *testing.T) {
	stream := `{"type":"thread.started","thread_id":"th_x"}
{"type":"turn.failed","error":{"message":"sandbox denied"}}
`
	cr := parseCodexResult([]byte(stream), "")
	require.True(t, cr.IsError)
	require.Equal(t, "error", cr.Subtype)
	require.Equal(t, "sandbox denied", cr.Result)
}

func TestParseCodexResultTransientError(t *testing.T) {
	t.Run("reconnect followed by a completed turn is not an error", func(t *testing.T) {
		stream := `{"type":"thread.started","thread_id":"th_r"}
{"type":"error","message":"Reconnecting... 2/5 (stream disconnected before completion)"}
{"type":"item.completed","item":{"type":"agent_message","text":"done"}}
{"type":"turn.completed","usage":{"input_tokens":100,"cached_input_tokens":0,"output_tokens":10}}
`
		cr := parseCodexResult([]byte(stream), "gpt-6-sol")
		require.False(t, cr.IsError)
		require.Equal(t, "done", cr.Result)
	})

	t.Run("error after the last completed turn stays an error", func(t *testing.T) {
		stream := `{"type":"turn.completed","usage":{"input_tokens":100,"cached_input_tokens":0,"output_tokens":10}}
{"type":"error","message":"stream disconnected: retries exhausted"}
`
		cr := parseCodexResult([]byte(stream), "gpt-6-sol")
		require.True(t, cr.IsError)
		require.Equal(t, "stream disconnected: retries exhausted", cr.Result)
	})
}

func TestExecCodexRunnerBuildArgs(t *testing.T) {
	t.Run("fresh run", func(t *testing.T) {
		r := &ExecCodexRunner{Model: "gpt-6-sol"}
		args := r.buildArgs()
		require.Equal(t, "exec", args[0])
		require.Contains(t, args, "--json")
		require.Contains(t, args, "--sandbox")
		require.Contains(t, args, "workspace-write")
		require.Contains(t, args, "-m")
		require.Contains(t, args, "gpt-6-sol")
		require.Equal(t, "-", args[len(args)-1])
		require.NotContains(t, args, "resume")
		require.NotContains(t, strings.Join(args, " "), "model_reasoning_effort") // model default
	})

	t.Run("sandbox override for containers", func(t *testing.T) {
		args := (&ExecCodexRunner{Sandbox: "danger-full-access"}).buildArgs()
		require.Contains(t, args, "danger-full-access")
		require.NotContains(t, args, "workspace-write")

		args = (&ExecCodexRunner{SessionID: "th_1", Sandbox: "danger-full-access"}).buildArgs()
		require.Contains(t, args, `sandbox_mode="danger-full-access"`)
	})

	t.Run("effort is passed as a config override", func(t *testing.T) {
		args := (&ExecCodexRunner{Effort: "high"}).buildArgs()
		require.Contains(t, args, `model_reasoning_effort="high"`)
		require.Equal(t, "-", args[len(args)-1])

		args = (&ExecCodexRunner{SessionID: "th_1", Effort: "xhigh"}).buildArgs()
		require.Contains(t, args, `model_reasoning_effort="xhigh"`)
	})

	t.Run("resume session", func(t *testing.T) {
		r := &ExecCodexRunner{SessionID: "th_1"}
		args := r.buildArgs()
		require.Equal(t, []string{"exec", "resume", "th_1"}, args[:3])
		require.NotContains(t, args, "--sandbox") // resume sets sandbox via -c
		require.Contains(t, args, "-c")
		require.Equal(t, "-", args[len(args)-1])
	})
}

func TestParseCodexResultUnknownModelCostsZero(t *testing.T) {
	stream := `{"type":"turn.completed","usage":{"input_tokens":1000,"cached_input_tokens":0,"output_tokens":100}}
`
	require.Zero(t, parseCodexResult([]byte(stream), "gpt-5-codex").TotalCostUSD) // shut down 2026-07-23
	require.Zero(t, parseCodexResult([]byte(stream), "unknown-model").TotalCostUSD)
}

func TestParseCodexResultCacheWrite(t *testing.T) {
	stream := `{"type":"turn.completed","usage":{"input_tokens":1000,"cached_input_tokens":200,"cache_write_input_tokens":300,"output_tokens":50,"reasoning_output_tokens":0}}
`
	cr := parseCodexResult([]byte(stream), "gpt-6-sol")
	require.Equal(t, 500, cr.Usage.InputTokens) // 1000 - 200 cached - 300 written
	require.Equal(t, 200, cr.Usage.CacheReadInputTokens)
	require.Equal(t, 300, cr.Usage.CacheCreationInputTokens)
	// (500*2 + 200*0.2 + 300*2.5 + 50*10) / 1e6
	require.InDelta(t, 0.00229, cr.TotalCostUSD, 1e-9)
}

// Real codex 0.156 numbers: turn.completed reports the thread's cumulative
// usage, so the resumed run (Step 2 retry) must bill only its own delta.
func TestExecCodexRunnerResumeBillsDelta(t *testing.T) {
	first := `{"type":"thread.started","thread_id":"th_1"}
{"type":"turn.completed","usage":{"input_tokens":42992,"cached_input_tokens":37504,"cache_write_input_tokens":0,"output_tokens":208,"reasoning_output_tokens":12}}
`
	resumed := `{"type":"thread.started","thread_id":"th_1"}
{"type":"turn.completed","usage":{"input_tokens":57532,"cached_input_tokens":44544,"cache_write_input_tokens":0,"output_tokens":214,"reasoning_output_tokens":12}}
`
	r := &ExecCodexRunner{Model: "gpt-6-sol"}
	cr := r.result(parseCodexStream([]byte(first)))
	require.Equal(t, 42992-37504, cr.Usage.InputTokens)
	require.Equal(t, 208, cr.Usage.OutputTokens)

	r.SetSession("th_1")
	cr = r.result(parseCodexStream([]byte(resumed)))
	require.Equal(t, 14540-7040, cr.Usage.InputTokens)
	require.Equal(t, 7040, cr.Usage.CacheReadInputTokens)
	require.Equal(t, 6, cr.Usage.OutputTokens)

	// A different thread (or one resumed from another process) has no baseline.
	r = &ExecCodexRunner{Model: "gpt-6-sol", SessionID: "th_other"}
	cr = r.result(parseCodexStream([]byte(resumed)))
	require.Equal(t, 214, cr.Usage.OutputTokens)
}

func TestExecCodexRunnerName(t *testing.T) {
	require.Equal(t, RunnerCodex, (&ExecCodexRunner{}).Name())
}

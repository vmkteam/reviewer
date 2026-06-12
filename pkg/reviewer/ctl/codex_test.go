package ctl

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseCodexResult(t *testing.T) {
	stream := `{"type":"thread.started","thread_id":"th_abc"}
{"type":"item.started","item":{"type":"command_execution","command":"git diff"}}
{"type":"item.completed","item":{"type":"agent_message","text":"done"}}
{"type":"turn.completed","usage":{"input_tokens":1000,"cached_input_tokens":200,"output_tokens":50}}
`
	cr := ParseCodexResult([]byte(stream), "gpt-5-codex")
	require.Equal(t, "th_abc", cr.SessionID)
	require.Equal(t, "done", cr.Result)
	require.Equal(t, 1, cr.NumTurns)
	require.Equal(t, 800, cr.Usage.InputTokens) // 1000 total - 200 cached
	require.Equal(t, 200, cr.Usage.CacheReadInputTokens)
	require.Equal(t, 50, cr.Usage.OutputTokens)
	require.False(t, cr.IsError)
	// (800*1.25 + 200*0.125 + 50*10) / 1e6
	require.InDelta(t, 0.001525, cr.TotalCostUSD, 1e-9)
	require.Contains(t, cr.ModelUsage, "gpt-5-codex")
}

func TestParseCodexResultError(t *testing.T) {
	stream := `{"type":"thread.started","thread_id":"th_x"}
{"type":"turn.failed","error":{"message":"sandbox denied"}}
`
	cr := ParseCodexResult([]byte(stream), "")
	require.True(t, cr.IsError)
	require.Equal(t, "error", cr.Subtype)
	require.Equal(t, "sandbox denied", cr.Result)
}

func TestExecCodexRunnerBuildArgs(t *testing.T) {
	t.Run("fresh run", func(t *testing.T) {
		r := &ExecCodexRunner{Model: "gpt-5-codex"}
		args := r.buildArgs()
		require.Equal(t, "exec", args[0])
		require.Contains(t, args, "--json")
		require.Contains(t, args, "--sandbox")
		require.Contains(t, args, "workspace-write")
		require.Contains(t, args, "-m")
		require.Contains(t, args, "gpt-5-codex")
		require.Equal(t, "-", args[len(args)-1])
		require.NotContains(t, args, "resume")
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

func TestCodexEstimateCostUSD(t *testing.T) {
	require.InDelta(t, 0.001525, codexEstimateCostUSD("gpt-5-codex", 800, 50, 200), 1e-9)
	require.Zero(t, codexEstimateCostUSD("unknown-model", 1000, 100, 0))
}

func TestExecCodexRunnerName(t *testing.T) {
	require.Equal(t, RunnerCodex, (&ExecCodexRunner{}).Name())
}

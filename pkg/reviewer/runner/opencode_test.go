package runner

import (
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseOpenCodeResult(t *testing.T) {
	// Real NDJSON stream captured from `opencode run --format json` (trimmed).
	stream := `{"type":"step_start","timestamp":1776937712618,"sessionID":"ses_246425057ffeZVW0DxiwJpiyjY","part":{"id":"prt_a","type":"step-start"}}
{"type":"text","timestamp":1776937712666,"sessionID":"ses_246425057ffeZVW0DxiwJpiyjY","part":{"id":"prt_b","type":"text","text":"\n\nhello"}}
{"type":"text","timestamp":1776937712700,"sessionID":"ses_246425057ffeZVW0DxiwJpiyjY","part":{"id":"prt_c","type":"text","text":" world"}}
{"type":"step_finish","timestamp":1776937712736,"sessionID":"ses_246425057ffeZVW0DxiwJpiyjY","part":{"id":"prt_d","reason":"stop","modelID":"claude-sonnet-4-5","providerID":"anthropic","type":"step-finish","tokens":{"total":12575,"input":10711,"output":24,"reasoning":0,"cache":{"write":500,"read":1840}},"cost":0.0123}}
`

	cr, err := ParseOpenCodeResult([]byte(stream), "fallback")
	require.NoError(t, err)

	assert.Equal(t, "result", cr.Type)
	assert.Equal(t, "success", cr.Subtype)
	assert.Equal(t, "\n\nhello world", cr.Result)
	assert.Equal(t, "ses_246425057ffeZVW0DxiwJpiyjY", cr.SessionID)
	assert.Equal(t, 1, cr.NumTurns)
	assert.Equal(t, "stop", cr.StopReason)
	assert.False(t, cr.IsError)
	assert.InDelta(t, 0.0123, cr.TotalCostUSD, 1e-9)

	assert.Equal(t, 10711, cr.Usage.InputTokens)
	assert.Equal(t, 24, cr.Usage.OutputTokens)
	assert.Equal(t, 1840, cr.Usage.CacheReadInputTokens)
	assert.Equal(t, 500, cr.Usage.CacheCreationInputTokens)
	assert.Equal(t, 500, cr.Usage.CacheCreation.Ephemeral5mInputTokens)
	assert.Equal(t, 118, cr.DurationMs) // 1776937712736 - 1776937712618

	require.Contains(t, cr.ModelUsage, "anthropic/claude-sonnet-4-5")
	m := cr.ModelUsage["anthropic/claude-sonnet-4-5"]
	assert.Equal(t, 10711, m.InputTokens)
	assert.InDelta(t, 0.0123, m.CostUSD, 1e-9)
}

func TestParseOpenCodeResult_FallbackModel(t *testing.T) {
	// step_finish without modelID → use fallback.
	stream := `{"type":"text","timestamp":1,"sessionID":"ses_x","part":{"type":"text","text":"hi"}}
{"type":"step_finish","timestamp":2,"sessionID":"ses_x","part":{"reason":"stop","tokens":{"input":5,"output":2,"cache":{"read":0,"write":0}},"cost":0}}
`
	cr, err := ParseOpenCodeResult([]byte(stream), "opencode/gpt-5-nano")
	require.NoError(t, err)
	require.Contains(t, cr.ModelUsage, "opencode/gpt-5-nano")
}

func TestParseOpenCodeResult_ErrorReason(t *testing.T) {
	stream := `{"type":"text","timestamp":1,"sessionID":"ses_x","part":{"type":"text","text":"partial"}}
{"type":"step_finish","timestamp":2,"sessionID":"ses_x","part":{"reason":"max_tokens","tokens":{"input":5,"output":2,"cache":{"read":0,"write":0}},"cost":0}}
`
	cr, err := ParseOpenCodeResult([]byte(stream), "m")
	require.NoError(t, err)
	assert.True(t, cr.IsError)
	assert.Equal(t, "error", cr.Subtype)
	assert.Equal(t, "max_tokens", cr.StopReason)
}

func TestParseOpenCodeResult_Empty(t *testing.T) {
	_, err := ParseOpenCodeResult(nil, "")
	require.Error(t, err)

	_, err = ParseOpenCodeResult([]byte("\n\n"), "")
	require.Error(t, err)
}

func TestParseOpenCodeResult_SkipsMalformedLines(t *testing.T) {
	stream := "garbage line\n" +
		`{"type":"text","timestamp":1,"sessionID":"ses_x","part":{"type":"text","text":"ok"}}` + "\n" +
		"{broken json\n" +
		`{"type":"step_finish","timestamp":2,"sessionID":"ses_x","part":{"reason":"stop","tokens":{"input":1,"output":1,"cache":{"read":0,"write":0}},"cost":0}}` + "\n"
	cr, err := ParseOpenCodeResult([]byte(stream), "m")
	require.NoError(t, err)
	assert.Equal(t, "ok", cr.Result)
	assert.Equal(t, 1, cr.NumTurns)
}

func TestOpenCodeBuildArgsPinsDir(t *testing.T) {
	r := &ExecOpenCodeRunner{Dir: "/tmp/wt-1"}
	args := r.buildArgs()
	require.Contains(t, args, "--dir")
	assert.Equal(t, "/tmp/wt-1", args[slices.Index(args, "--dir")+1],
		"--dir must pin the worktree so opencode does not escape to the main repo via the shared git dir")

	r = &ExecOpenCodeRunner{}
	assert.NotContains(t, r.buildArgs(), "--dir", "no dir configured → no flag")
}

func TestFindOpenCodeProjectConfig(t *testing.T) {
	// outer/.opencode sits above the git root: opencode stops before it.
	outer := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(outer, ".opencode"), 0o755))
	repo := filepath.Join(outer, "repo")
	sub := filepath.Join(repo, "pkg")
	require.NoError(t, os.MkdirAll(filepath.Join(repo, ".git"), 0o755))
	require.NoError(t, os.MkdirAll(sub, 0o755))

	t.Run("clean checkout", func(t *testing.T) {
		p, err := findOpenCodeProjectConfig(sub)
		require.NoError(t, err)
		assert.Empty(t, p)
	})

	for _, name := range opencodeProjectConfig {
		t.Run(name, func(t *testing.T) {
			p := filepath.Join(repo, name)
			if name == ".opencode" {
				require.NoError(t, os.Mkdir(p, 0o755))
			} else {
				require.NoError(t, os.WriteFile(p, []byte("{}"), 0o644))
			}
			t.Cleanup(func() { _ = os.RemoveAll(p) })

			got, err := findOpenCodeProjectConfig(sub)
			require.NoError(t, err)
			assert.Equal(t, p, got, "found from a subdir up to the git root")
		})
	}

	t.Run("linked worktree root", func(t *testing.T) {
		wt := filepath.Join(outer, "wt")
		require.NoError(t, os.Mkdir(wt, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(wt, ".git"), []byte("gitdir: /x/.git/worktrees/wt\n"), 0o644))

		p, err := findOpenCodeProjectConfig(wt)
		require.NoError(t, err)
		assert.Empty(t, p, "a .git file ends the walk like a .git dir")
	})
}

func TestExecOpenCodeRunnerRefusesProjectConfig(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".opencode", "plugin"), 0o755))

	r := &ExecOpenCodeRunner{Dir: dir, Log: slog.New(slog.DiscardHandler)}
	cr, err := r.Run(t.Context(), "review")
	require.ErrorContains(t, err, "refusing to run opencode")
	assert.Nil(t, cr)
}

func TestOpenCodeIsolationEnv(t *testing.T) {
	t.Setenv("OPENCODE_DISABLE_PROJECT_CONFIG", "")
	t.Setenv("OPENCODE_CONFIG_CONTENT", "")
	assert.Equal(t, []string{
		"OPENCODE_DISABLE_PROJECT_CONFIG=1",
		`OPENCODE_CONFIG_CONTENT={"formatter":false,"lsp":false}`,
	}, opencodeIsolationEnv())

	t.Setenv("OPENCODE_DISABLE_PROJECT_CONFIG", "0")
	t.Setenv("OPENCODE_CONFIG_CONTENT", `{"share":"disabled","lsp":true}`)
	assert.Equal(t, []string{
		`OPENCODE_CONFIG_CONTENT={"formatter":false,"lsp":false,"share":"disabled"}`,
	}, opencodeIsolationEnv(), "the operator's config is kept, formatter and lsp forced off")

	t.Setenv("OPENCODE_CONFIG_CONTENT", "{ // jsonc\n}")
	assert.Empty(t, opencodeIsolationEnv(), "an unparsable operator config is left alone")
}

package ctl

import (
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

package ctl

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseClaudeResult(t *testing.T) {
	data, err := os.ReadFile("testdata/claude_result.json")
	require.NoError(t, err)

	cr, err := ParseClaudeResult(data)
	require.NoError(t, err)

	assert.Equal(t, "result", cr.Type)
	assert.Equal(t, "success", cr.Subtype)
	assert.InDelta(t, 2.0875265, cr.TotalCostUSD, 0.0001)
	assert.Equal(t, 145780, cr.DurationMs)
	assert.Equal(t, 142410, cr.DurationAPIMs)
	assert.Equal(t, 15, cr.NumTurns)
	assert.Equal(t, "34ee826b-a176-42dc-abd0-2a4ea59f47be", cr.SessionID)
	assert.Equal(t, 2680, cr.Usage.InputTokens)
	assert.Equal(t, 154964, cr.Usage.CacheCreationInputTokens)
	assert.Equal(t, 1879403, cr.Usage.CacheReadInputTokens)
	assert.Equal(t, 6636, cr.Usage.OutputTokens)

	assert.False(t, cr.IsError)
	assert.Equal(t, "end_turn", cr.StopReason)
	assert.Equal(t, "completed", cr.TerminalReason)
	assert.Equal(t, 100000, cr.Usage.CacheCreation.Ephemeral1hInputTokens)
	assert.Equal(t, 54964, cr.Usage.CacheCreation.Ephemeral5mInputTokens)
	assert.Equal(t, 2, cr.Usage.ServerToolUse.WebFetchRequests)
	assert.Equal(t, 0, cr.Usage.ServerToolUse.WebSearchRequests)

	require.Contains(t, cr.ModelUsage, "claude-opus-4-6")
	opus := cr.ModelUsage["claude-opus-4-6"]
	assert.Equal(t, 2680, opus.InputTokens)
	assert.Equal(t, 6636, opus.OutputTokens)
	assert.InDelta(t, 2.0875265, opus.CostUSD, 0.0001)
}

func TestParseClaudeResult_Error(t *testing.T) {
	data, err := os.ReadFile("testdata/claude_result_error.json")
	require.NoError(t, err)

	cr, err := ParseClaudeResult(data)
	require.Error(t, err)
	require.NotNil(t, cr, "expected ClaudeResult even on error")
	assert.Equal(t, "error_max_turns", cr.Subtype)
}

func TestParseClaudeResult_InvalidJSON(t *testing.T) {
	_, err := ParseClaudeResult([]byte("not json"))
	require.Error(t, err)
}

func TestParseClaudeResult_WrongType(t *testing.T) {
	_, err := ParseClaudeResult([]byte(`{"type": "assistant", "subtype": "success"}`))
	require.Error(t, err)
}

func TestParseClaudeResult_EmptyUsage(t *testing.T) {
	data := []byte(`{"type": "result", "subtype": "success", "result": "", "usage": {}}`)
	cr, err := ParseClaudeResult(data)
	require.NoError(t, err)
	assert.Equal(t, 0, cr.Usage.InputTokens)
}

func TestParseClaudeResult_NDJSONArray(t *testing.T) {
	data := []byte(`[
		{"type":"system","subtype":"init","session_id":"abc-123"},
		{"type":"assistant","message":"working..."},
		{"type":"result","subtype":"success","result":"Done.","total_cost_usd":0.50,"num_turns":3,"usage":{"input_tokens":100,"output_tokens":200}}
	]`)

	cr, err := ParseClaudeResult(data)
	require.NoError(t, err)
	assert.Equal(t, "result", cr.Type)
	assert.InDelta(t, 0.50, cr.TotalCostUSD, 0.0001)
	assert.Equal(t, 3, cr.NumTurns)
}

func TestParseClaudeResult_NDJSONNoResult(t *testing.T) {
	data := []byte(`[{"type":"system","subtype":"init"},{"type":"assistant","message":"hi"}]`)
	_, err := ParseClaudeResult(data)
	require.Error(t, err)
}

func TestParseClaudeResult_TruncatedArray(t *testing.T) {
	// Array with result followed by truncated entry (simulates 448KB cutoff).
	data := []byte(`[
		{"type":"system","subtype":"init"},
		{"type":"result","subtype":"success","result":"Done.","total_cost_usd":1.5,"num_turns":5,"usage":{"input_tokens":100,"output_tokens":200}},
		{"type":"assistant","message":"trunc`)

	cr, err := ParseClaudeResult(data)
	require.NoError(t, err)
	assert.Equal(t, "result", cr.Type)
	assert.InDelta(t, 1.5, cr.TotalCostUSD, 0.0001)
}

func TestParseClaudeResult_TruncatedArrayNoResult(t *testing.T) {
	// Truncated array without result entry.
	data := []byte(`[{"type":"system","subtype":"init"},{"type":"assistant","mess`)
	_, err := ParseClaudeResult(data)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no result message")
}

func TestParseClaudeResult_NDJSON_File(t *testing.T) {
	data, err := os.ReadFile("testdata/claude_result_ndjson.json")
	require.NoError(t, err)

	cr, err := ParseClaudeResult(data)
	require.NoError(t, err)
	assert.InDelta(t, 2.0875265, cr.TotalCostUSD, 0.0001)
	assert.Equal(t, 1879403, cr.Usage.CacheReadInputTokens)
}

func TestClaudeResultToModelInfo(t *testing.T) {
	data, err := os.ReadFile("testdata/claude_result.json")
	require.NoError(t, err)

	cr, err := ParseClaudeResult(data)
	require.NoError(t, err)

	mi := cr.ToModelInfo("opus")

	// Full model id from modelUsage overrides the CLI alias.
	assert.Equal(t, "claude-opus-4-6", mi.Model)
	assert.Equal(t, 2680, mi.InputTokens)
	assert.Equal(t, 6636, mi.OutputTokens)
	assert.InDelta(t, 2.0875265, mi.CostUsd, 0.0001)
	assert.Equal(t, 154964, mi.CacheCreationInputTokens)
	assert.Equal(t, 1879403, mi.CacheReadInputTokens)
	assert.Equal(t, 15, mi.NumTurns)
	assert.Equal(t, "34ee826b-a176-42dc-abd0-2a4ea59f47be", mi.SessionID)
	assert.Equal(t, 142410, mi.DurationAPIMs)

	assert.Equal(t, 145780, mi.DurationTotalMs)
	assert.Equal(t, 100000, mi.CacheCreate1hInputTokens)
	assert.Equal(t, 54964, mi.CacheCreate5mInputTokens)
	assert.Equal(t, 2, mi.WebFetchRequests)
	assert.Equal(t, "end_turn", mi.StopReason)
	assert.Equal(t, "completed", mi.TerminalReason)
	assert.False(t, mi.IsError)

	require.Contains(t, mi.Models, "claude-opus-4-6")
	opus := mi.Models["claude-opus-4-6"]
	assert.Equal(t, 2680, opus.InputTokens)
	assert.InDelta(t, 2.0875265, opus.CostUsd, 0.0001)
}

func TestClaudeResultToModelInfo_PrimaryModelPicksHighestCost(t *testing.T) {
	cr := &ClaudeResult{
		Type:    claudeResultType,
		Subtype: "success",
		ModelUsage: map[string]ClaudeModelUse{
			"claude-opus-4-7":         {CostUSD: 2.45},
			"claude-haiku-4-5-202510": {CostUSD: 0.14},
		},
	}

	assert.Equal(t, "claude-opus-4-7", cr.ToModelInfo("opus").Model)
}

func TestClaudeResultToModelInfo_FallbackWhenNoModelUsage(t *testing.T) {
	cr := &ClaudeResult{Type: claudeResultType, Subtype: "success"}

	assert.Equal(t, "opus", cr.ToModelInfo("opus").Model)
}

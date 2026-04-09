package ctl

import (
	"os"
	"testing"
)

func TestParseClaudeResult(t *testing.T) {
	data, err := os.ReadFile("testdata/claude_result.json")
	if err != nil {
		t.Fatal(err)
	}

	cr, err := ParseClaudeResult(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cr.Type != "result" {
		t.Errorf("type = %q, want %q", cr.Type, "result")
	}
	if cr.Subtype != "success" {
		t.Errorf("subtype = %q, want %q", cr.Subtype, "success")
	}
	if cr.TotalCostUSD != 2.0875265 {
		t.Errorf("totalCostUSD = %f, want %f", cr.TotalCostUSD, 2.0875265)
	}
	if cr.DurationMs != 145780 {
		t.Errorf("durationMs = %d, want %d", cr.DurationMs, 145780)
	}
	if cr.DurationAPIMs != 142410 {
		t.Errorf("durationApiMs = %d, want %d", cr.DurationAPIMs, 142410)
	}
	if cr.NumTurns != 15 {
		t.Errorf("numTurns = %d, want %d", cr.NumTurns, 15)
	}
	if cr.SessionID != "34ee826b-a176-42dc-abd0-2a4ea59f47be" {
		t.Errorf("sessionID = %q, want %q", cr.SessionID, "34ee826b-a176-42dc-abd0-2a4ea59f47be")
	}
	if cr.Usage.InputTokens != 2680 {
		t.Errorf("inputTokens = %d, want %d", cr.Usage.InputTokens, 2680)
	}
	if cr.Usage.CacheCreationInputTokens != 154964 {
		t.Errorf("cacheCreationInputTokens = %d, want %d", cr.Usage.CacheCreationInputTokens, 154964)
	}
	if cr.Usage.CacheReadInputTokens != 1879403 {
		t.Errorf("cacheReadInputTokens = %d, want %d", cr.Usage.CacheReadInputTokens, 1879403)
	}
	if cr.Usage.OutputTokens != 6636 {
		t.Errorf("outputTokens = %d, want %d", cr.Usage.OutputTokens, 6636)
	}
}

func TestParseClaudeResult_Error(t *testing.T) {
	data, err := os.ReadFile("testdata/claude_result_error.json")
	if err != nil {
		t.Fatal(err)
	}

	cr, err := ParseClaudeResult(data)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if cr == nil {
		t.Fatal("expected ClaudeResult even on error")
	}
	if cr.Subtype != "error_max_turns" {
		t.Errorf("subtype = %q, want %q", cr.Subtype, "error_max_turns")
	}
}

func TestParseClaudeResult_InvalidJSON(t *testing.T) {
	_, err := ParseClaudeResult([]byte("not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseClaudeResult_WrongType(t *testing.T) {
	_, err := ParseClaudeResult([]byte(`{"type": "assistant", "subtype": "success"}`))
	if err == nil {
		t.Fatal("expected error for wrong type")
	}
}

func TestParseClaudeResult_EmptyUsage(t *testing.T) {
	data := []byte(`{"type": "result", "subtype": "success", "result": "", "usage": {}}`)
	cr, err := ParseClaudeResult(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cr.Usage.InputTokens != 0 {
		t.Errorf("inputTokens = %d, want 0", cr.Usage.InputTokens)
	}
}

func TestParseClaudeResult_NDJSONArray(t *testing.T) {
	data := []byte(`[
		{"type":"system","subtype":"init","session_id":"abc-123"},
		{"type":"assistant","message":"working..."},
		{"type":"result","subtype":"success","result":"Done.","total_cost_usd":0.50,"num_turns":3,"usage":{"input_tokens":100,"output_tokens":200}}
	]`)

	cr, err := ParseClaudeResult(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cr.Type != "result" {
		t.Errorf("type = %q, want %q", cr.Type, "result")
	}
	if cr.TotalCostUSD != 0.50 {
		t.Errorf("cost = %f, want 0.50", cr.TotalCostUSD)
	}
	if cr.NumTurns != 3 {
		t.Errorf("numTurns = %d, want 3", cr.NumTurns)
	}
}

func TestParseClaudeResult_NDJSONNoResult(t *testing.T) {
	data := []byte(`[{"type":"system","subtype":"init"},{"type":"assistant","message":"hi"}]`)
	_, err := ParseClaudeResult(data)
	if err == nil {
		t.Fatal("expected error for NDJSON without result")
	}
}

func TestParseClaudeResult_NDJSON_File(t *testing.T) {
	data, err := os.ReadFile("testdata/claude_result_ndjson.json")
	if err != nil {
		t.Fatal(err)
	}

	cr, err := ParseClaudeResult(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cr.TotalCostUSD != 2.0875265 {
		t.Errorf("cost = %f, want 2.0875265", cr.TotalCostUSD)
	}
	if cr.Usage.CacheReadInputTokens != 1879403 {
		t.Errorf("cacheRead = %d, want 1879403", cr.Usage.CacheReadInputTokens)
	}
}

func TestClaudeResultToModelInfo(t *testing.T) {
	data, err := os.ReadFile("testdata/claude_result.json")
	if err != nil {
		t.Fatal(err)
	}

	cr, err := ParseClaudeResult(data)
	if err != nil {
		t.Fatal(err)
	}

	mi := cr.ToModelInfo("opus")

	if mi.Model != "opus" {
		t.Errorf("model = %q, want %q", mi.Model, "opus")
	}
	if mi.InputTokens != 2680 {
		t.Errorf("inputTokens = %d, want %d", mi.InputTokens, 2680)
	}
	if mi.OutputTokens != 6636 {
		t.Errorf("outputTokens = %d, want %d", mi.OutputTokens, 6636)
	}
	if mi.CostUsd != 2.0875265 {
		t.Errorf("costUsd = %f, want %f", mi.CostUsd, 2.0875265)
	}
	if mi.CacheCreationInputTokens != 154964 {
		t.Errorf("cacheCreationInputTokens = %d, want %d", mi.CacheCreationInputTokens, 154964)
	}
	if mi.CacheReadInputTokens != 1879403 {
		t.Errorf("cacheReadInputTokens = %d, want %d", mi.CacheReadInputTokens, 1879403)
	}
	if mi.NumTurns != 15 {
		t.Errorf("numTurns = %d, want %d", mi.NumTurns, 15)
	}
	if mi.SessionID != "34ee826b-a176-42dc-abd0-2a4ea59f47be" {
		t.Errorf("sessionID = %q, want %q", mi.SessionID, "34ee826b-a176-42dc-abd0-2a4ea59f47be")
	}
	if mi.DurationAPIMs != 142410 {
		t.Errorf("durationApiMs = %d, want %d", mi.DurationAPIMs, 142410)
	}
}

package direct

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"reviewsrv/pkg/reviewer"

	"github.com/stretchr/testify/require"
)

// fakeAPI serves canned JSON bodies on path in order and records the decoded
// request bodies.
func fakeAPI(t *testing.T, path string, bodies ...string) (*httptest.Server, *[]map[string]any) {
	t.Helper()
	var reqs []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		if r.URL.Path != path || json.NewDecoder(r.Body).Decode(&req) != nil || len(reqs) >= len(bodies) {
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		reqs = append(reqs, req)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, bodies[len(reqs)-1])
	}))
	t.Cleanup(srv.Close)
	return srv, &reqs
}

func TestResponsesProviderRoundTrip(t *testing.T) {
	toolTurn := `{"id":"resp_1","object":"response","status":"completed","model":"gpt-6-sol","output":[
		{"id":"rs_1","type":"reasoning","summary":[],"encrypted_content":"ENC","created_by":"model"},
		{"id":"fc_1","type":"function_call","status":"completed","call_id":"call_1","name":"grep","arguments":"{\"pattern\":\"x\"}"}
	],"usage":{"input_tokens":1000,"input_tokens_details":{"cached_tokens":600,"cache_write_tokens":300},"output_tokens":50,"total_tokens":1050}}`
	textTurn := `{"id":"resp_2","object":"response","status":"completed","model":"gpt-6-sol","output":[
		{"id":"msg_1","type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"done","annotations":[]}]}
	],"usage":{"input_tokens":1200,"output_tokens":5,"total_tokens":1205}}`
	srv, reqs := fakeAPI(t, "/v1/responses", toolTurn, textTurn)

	p, err := NewResponsesProvider(OpenAIConfig{APIKey: "k", BaseURL: srv.URL + "/v1", Model: "gpt-6-sol"})
	require.NoError(t, err)

	tools := []ToolDef{{Name: "grep", Description: "search", Schema: map[string]any{"type": "object"}}}
	msgs := make([]Message, 0, 3)
	msgs = append(msgs, Message{Role: RoleUser, Text: "review"})
	resp, err := p.Complete(context.Background(), Request{System: "sys", Messages: msgs, Tools: tools, Effort: "high"})
	require.NoError(t, err)
	require.Equal(t, "tool_calls", resp.StopReason)
	require.Equal(t, []ToolCall{{ID: "call_1", Name: "grep", Args: json.RawMessage(`{"pattern":"x"}`)}}, resp.ToolCalls)
	require.Equal(t, Usage{InputTokens: 100, OutputTokens: 50, CacheReadTokens: 600, CacheWriteTokens: 300}, resp.Usage)

	first := (*reqs)[0]
	require.Equal(t, "gpt-6-sol", first["model"])
	require.Equal(t, "sys", first["instructions"])
	require.Equal(t, false, first["store"])
	require.Equal(t, map[string]any{"effort": "high"}, first["reasoning"])
	require.Equal(t, []any{"reasoning.encrypted_content"}, first["include"])
	require.InDelta(t, float64(defaultResponsesMaxTokens), first["max_output_tokens"], 0)
	require.Equal(t, []any{map[string]any{"type": "function", "name": "grep", "description": "search", "parameters": map[string]any{"type": "object"}}}, first["tools"])
	require.Equal(t, []any{map[string]any{"role": "user", "content": "review"}}, first["input"])

	// Next round replays the kept output items verbatim (encrypted reasoning and
	// ids included, output-only created_by dropped), then the tool output.
	msgs = append(msgs,
		Message{Role: RoleAssistant, ToolCalls: resp.ToolCalls, Raw: resp.Raw},
		Message{Role: RoleTool, ToolResults: []ToolResult{{CallID: "call_1", Name: "grep", Content: "a.go:1"}}},
	)
	resp, err = p.Complete(context.Background(), Request{System: "sys", Messages: msgs, Tools: tools})
	require.NoError(t, err)
	require.Equal(t, "done", resp.Text)
	require.Equal(t, "stop", resp.StopReason)

	second := (*reqs)[1]
	require.NotContains(t, second, "reasoning", "no effort -> model default")
	input, ok := second["input"].([]any)
	require.True(t, ok)
	require.Len(t, input, 4)
	require.Equal(t, map[string]any{"id": "rs_1", "type": "reasoning", "summary": []any{}, "encrypted_content": "ENC"}, input[1])
	require.Equal(t, "fc_1", input[2].(map[string]any)["id"])
	require.Equal(t, map[string]any{"type": "function_call_output", "call_id": "call_1", "output": "a.go:1"}, input[3])
}

func TestNewProviderOpenAIRouting(t *testing.T) {
	// OpenAI's own endpoint speaks the Responses API; a custom base URL may be a
	// proxy that only implements chat completions.
	for _, base := range []string{"", "https://api.openai.com/v1", "https://API.openai.com/v1/"} {
		p, err := NewProvider(ProviderConfig{Provider: ProviderOpenAI, Model: "gpt-6-sol", APIKey: "k", BaseURL: base})
		require.NoError(t, err)
		require.IsType(t, &responsesProvider{}, p, base)
		require.InEpsilon(t, 2.0, p.Pricing().InputPerMTok, 1e-9)
	}
	for _, base := range []string{"https://my-azure.openai.azure.com/openai/v1", "http://litellm:4000/v1"} {
		p, err := NewProvider(ProviderConfig{Provider: ProviderOpenAI, Model: "gpt-5.5", APIKey: "k", BaseURL: base})
		require.NoError(t, err)
		require.IsType(t, &openaiProvider{}, p, base)
	}
}

func TestResponsesProviderRebuildsWithoutRaw(t *testing.T) {
	// After compaction the loop drops Raw: the turn is rebuilt from neutral fields.
	items := toResponsesInput(Request{Messages: []Message{
		{Role: RoleUser, Text: "go"},
		{Role: RoleAssistant, Text: "looking", ToolCalls: []ToolCall{{ID: "c1", Name: "glob", Args: json.RawMessage(`{"pattern":"*"}`)}}},
		{Role: RoleTool, ToolResults: []ToolResult{{CallID: "c1", Content: "a.go"}}},
	}})
	data, err := json.Marshal(items)
	require.NoError(t, err)
	require.JSONEq(t, `[
		{"role":"user","content":"go"},
		{"role":"assistant","content":"looking"},
		{"type":"function_call","call_id":"c1","name":"glob","arguments":"{\"pattern\":\"*\"}"},
		{"type":"function_call_output","call_id":"c1","output":"a.go"}
	]`, string(data))
}

func TestResponsesProviderTruncatedAndFailed(t *testing.T) {
	prev := openaiRetry.backoff
	openaiRetry.backoff = time.Millisecond
	t.Cleanup(func() { openaiRetry.backoff = prev })

	completed := `{"id":"r3","object":"response","status":"completed","model":"gpt-6-sol","output":[
		{"id":"msg_1","type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"done","annotations":[]}]}
	],"usage":{"input_tokens":10,"output_tokens":1,"total_tokens":11}}`
	srv, _ := fakeAPI(t, "/v1/responses",
		// Cut off mid-reasoning: only a reasoning item, nothing after it.
		`{"id":"r1","object":"response","status":"incomplete","incomplete_details":{"reason":"max_output_tokens"},"model":"gpt-6-sol",
			"output":[{"id":"rs_1","type":"reasoning","summary":[],"encrypted_content":"ENC"}]}`,
		// A failed reply with a server error is retried like an HTTP 5xx.
		`{"id":"r2","object":"response","status":"failed","error":{"code":"server_error","message":"boom"},"model":"gpt-6-sol","output":[]}`,
		completed,
		// A definitive failure is not retried.
		`{"id":"r4","object":"response","status":"failed","error":{"code":"invalid_prompt","message":"flagged"},"model":"gpt-6-sol","output":[]}`,
	)
	p, err := NewResponsesProvider(OpenAIConfig{APIKey: "k", BaseURL: srv.URL + "/v1", Model: "gpt-6-sol"})
	require.NoError(t, err)
	var retries []string
	req := Request{
		Messages: []Message{{Role: RoleUser, Text: "go"}},
		OnRetry:  func(err error) { retries = append(retries, err.Error()) },
	}

	resp, err := p.Complete(context.Background(), req)
	require.NoError(t, err)
	require.True(t, IsTruncated(resp.StopReason))
	require.Nil(t, resp.Raw, "a lone reasoning item must not be replayed")

	resp, err = p.Complete(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, "done", resp.Text)
	require.Len(t, retries, 1)
	require.Contains(t, retries[0], "response r2 failed: server_error: boom")

	retries = nil
	_, err = p.Complete(context.Background(), req)
	require.ErrorContains(t, err, "response r4 failed: invalid_prompt: flagged")
	require.Empty(t, retries)
	require.Equal(t, reviewer.RunReasonOther, providerReason(err))
}

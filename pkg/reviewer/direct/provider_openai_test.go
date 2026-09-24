package direct

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	openai "github.com/sashabaranov/go-openai"
	"github.com/stretchr/testify/require"
)

func TestToOpenAIMessagesMultipleToolResults(t *testing.T) {
	req := Request{
		System: "sys",
		Messages: []Message{
			{Role: RoleUser, Text: "go"},
			{Role: RoleAssistant, ToolCalls: []ToolCall{
				{ID: "a", Name: "glob", Args: json.RawMessage(`{"pattern":"*"}`)},
				{ID: "b", Name: "grep", Args: json.RawMessage(`{"pattern":"x"}`)},
			}},
			{Role: RoleTool, ToolResults: []ToolResult{
				{CallID: "a", Name: "glob", Content: "file.go"},
				{CallID: "b", Name: "grep", Content: "match"},
			}},
		},
	}

	msgs := toOpenAIMessages(req)
	// system + user + assistant + one tool message PER result = 5
	require.Len(t, msgs, 5)
	require.Equal(t, openai.ChatMessageRoleSystem, msgs[0].Role)
	require.Equal(t, openai.ChatMessageRoleUser, msgs[1].Role)
	require.Equal(t, openai.ChatMessageRoleAssistant, msgs[2].Role)
	require.Len(t, msgs[2].ToolCalls, 2)
	require.Equal(t, openai.ChatMessageRoleTool, msgs[3].Role)
	require.Equal(t, "a", msgs[3].ToolCallID)
	require.Equal(t, openai.ChatMessageRoleTool, msgs[4].Role)
	require.Equal(t, "b", msgs[4].ToolCallID)
}

func TestIsOpenAITransient(t *testing.T) {
	require.True(t, isOpenAITransient(&openai.APIError{HTTPStatusCode: 429}))
	require.True(t, isOpenAITransient(&openai.APIError{HTTPStatusCode: 503}))
	require.False(t, isOpenAITransient(&openai.APIError{HTTPStatusCode: 400}))
	require.False(t, isOpenAITransient(&openai.APIError{HTTPStatusCode: 401}))
	require.False(t, isOpenAITransient(&openai.APIError{HTTPStatusCode: 429, Code: "insufficient_quota"}), "an exhausted quota is billing, not a rate limit")
	require.True(t, isOpenAITransient(&openai.RequestError{HTTPStatusCode: 500}))
	require.False(t, isOpenAITransient(errors.New("plain error")))
}

func TestToOpenAIMessagesSkipsBareAssistant(t *testing.T) {
	// A bare assistant turn (no content, no tool calls) must be dropped — the
	// DeepSeek/OpenAI API rejects it ("content or tool_calls must be set").
	msgs := toOpenAIMessages(Request{Messages: []Message{
		{Role: RoleUser, Text: "go"},
		{Role: RoleAssistant},
		{Role: RoleUser, Text: "next"},
	}})
	for _, m := range msgs {
		if m.Role == "assistant" {
			t.Fatalf("bare assistant must be skipped, got %+v", m)
		}
	}
	require.Len(t, msgs, 2)
}

func TestOpenAIProviderPassesEffort(t *testing.T) {
	const reply = `{"choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":2}}`
	srv, reqs := fakeAPI(t, "/chat/completions", reply, reply, reply)

	// DeepSeek: the level goes through verbatim (the API maps medium/xhigh to
	// high itself) and the output cap leaves room for thinking.
	p, err := NewProvider(ProviderConfig{Provider: ProviderDeepSeek, Model: "deepseek-v4-pro", APIKey: "k", BaseURL: srv.URL})
	require.NoError(t, err)
	_, err = p.Complete(context.Background(), Request{Messages: []Message{{Role: RoleUser, Text: "go"}}, Effort: "xhigh"})
	require.NoError(t, err)
	require.Equal(t, "xhigh", (*reqs)[0]["reasoning_effort"])
	require.InDelta(t, float64(deepseekMaxTokens), (*reqs)[0]["max_tokens"], 0)

	// No effort -> field omitted, backend default applies.
	_, err = p.Complete(context.Background(), Request{Messages: []Message{{Role: RoleUser, Text: "go"}}})
	require.NoError(t, err)
	require.NotContains(t, (*reqs)[1], "reasoning_effort")

	// openai-compat: an arbitrary backend may reject reasoning_effort, so the
	// profile's effort is not sent.
	p, err = NewProvider(ProviderConfig{Provider: ProviderOpenAICompat, Model: "local-model", APIKey: "k", BaseURL: srv.URL})
	require.NoError(t, err)
	_, err = p.Complete(context.Background(), Request{Messages: []Message{{Role: RoleUser, Text: "go"}}, Effort: "high"})
	require.NoError(t, err)
	require.NotContains(t, (*reqs)[2], "reasoning_effort")
}

func TestSplitInput(t *testing.T) {
	require.Equal(t, Usage{InputTokens: 100, OutputTokens: 5, CacheReadTokens: 600, CacheWriteTokens: 300}, SplitInput(1000, 600, 300, 5))
	// Counters that overshoot the input total are capped, never negative.
	require.Equal(t, Usage{CacheReadTokens: 10}, SplitInput(10, 50, 50, 0))
}

func TestWithRetry(t *testing.T) {
	prev := openaiRetry.backoff
	openaiRetry.backoff = time.Millisecond
	t.Cleanup(func() { openaiRetry.backoff = prev })

	// fails returns an fn that fails with err n times, then succeeds.
	fails := func(n int, err error) (func() (string, error), *int) {
		calls := 0
		return func() (string, error) {
			calls++
			if calls <= n {
				return "", err
			}
			return "ok", nil
		}, &calls
	}
	ctx := context.Background()

	fn, calls := fails(1, &openai.APIError{HTTPStatusCode: 503})
	var retried []string
	got, err := withRetry(ctx, openaiRetry, func(err error) { retried = append(retried, err.Error()) }, fn)
	require.NoError(t, err, "a transient error is retried")
	require.Equal(t, "ok", got)
	require.Equal(t, 2, *calls)
	require.Len(t, retried, 1, "the retried failure is reported")

	fn, calls = fails(maxRetries+1, &openai.APIError{HTTPStatusCode: 429})
	_, err = withRetry(ctx, openaiRetry, nil, fn)
	require.ErrorContains(t, err, "openai:", "retries are bounded")
	require.Equal(t, maxRetries+1, *calls)

	fn, calls = fails(1, &openai.APIError{HTTPStatusCode: 400})
	_, err = withRetry(ctx, openaiRetry, nil, fn)
	require.Error(t, err, "a client error is not retried")
	require.Equal(t, 1, *calls)

	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	fn, calls = fails(1, &openai.APIError{HTTPStatusCode: 503})
	_, err = withRetry(cancelled, openaiRetry, nil, fn)
	require.ErrorContains(t, err, "openai:")
	require.Equal(t, 1, *calls, "no retry after cancellation")
}

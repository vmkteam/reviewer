package direct

import (
	"encoding/json"
	"errors"
	"testing"

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

func TestIsTransientErr(t *testing.T) {
	require.True(t, isTransientErr(&openai.APIError{HTTPStatusCode: 429}))
	require.True(t, isTransientErr(&openai.APIError{HTTPStatusCode: 503}))
	require.False(t, isTransientErr(&openai.APIError{HTTPStatusCode: 400}))
	require.False(t, isTransientErr(&openai.APIError{HTTPStatusCode: 401}))
	require.True(t, isTransientErr(&openai.RequestError{HTTPStatusCode: 500}))
	require.False(t, isTransientErr(errors.New("plain error")))
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

package direct

import (
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/stretchr/testify/require"
)

func TestToAnthropicMessagesReplaysRawAssistant(t *testing.T) {
	raw := anthropic.NewAssistantMessage(anthropic.NewTextBlock("verbatim with thinking"))

	out := toAnthropicMessages(Request{Messages: []Message{
		{Role: RoleUser, Text: "go"},
		// Text here must be ignored in favour of the verbatim Raw turn.
		{Role: RoleAssistant, Text: "rebuilt-should-not-be-used", Raw: raw},
		{Role: RoleTool, ToolResults: []ToolResult{{CallID: "1", Content: "ok"}}},
	}})

	require.Len(t, out, 3)
	require.Equal(t, raw, out[1], "assistant turn must be replayed verbatim from Raw")
}

func TestToAnthropicMessagesRebuildsWithoutRaw(t *testing.T) {
	out := toAnthropicMessages(Request{Messages: []Message{
		{Role: RoleAssistant, Text: "hi", ToolCalls: []ToolCall{{ID: "1", Name: "t", Args: []byte(`{}`)}}},
	}})
	require.Len(t, out, 1)
	require.Equal(t, anthropic.MessageParamRoleAssistant, out[0].Role)
}

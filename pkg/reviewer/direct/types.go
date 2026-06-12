// Package direct implements a review runner that drives an LLM through a direct
// API (Anthropic native or OpenAI-compatible such as DeepSeek) with a narrow,
// review-specific tool surface — read_file, glob, grep, git_diff, submit_review —
// instead of shelling out to the claude/opencode CLIs.
//
// The package is provider-neutral: the agent loop (loop.go) speaks the message,
// tool and usage types defined here, and each LLMProvider translates them to/from
// its own SDK. See docs/llm/DirectAPIRunner.Spec.md for the design.
package direct

import "encoding/json"

// Role identifies who produced a Message.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Message is one turn of the conversation in neutral form. A provider translates
// it to its SDK shape on every request. An assistant message may carry ToolCalls;
// a tool message carries the ToolResults produced for the previous assistant turn.
type Message struct {
	Role        Role
	Text        string
	ToolCalls   []ToolCall
	ToolResults []ToolResult

	// Raw is an opaque, provider-native snapshot of an assistant turn used to
	// replay it verbatim on later requests. The Anthropic provider stores the
	// SDK MessageParam here so signed thinking blocks survive the round-trip
	// (rebuilding from Text+ToolCalls alone would drop them). Nil for messages
	// the loop creates itself; providers that don't set it just reconstruct.
	Raw any
}

// ToolCall is a tool invocation requested by the model.
type ToolCall struct {
	ID   string
	Name string
	Args json.RawMessage
}

// ToolResult is the outcome of executing a ToolCall, fed back to the model.
type ToolResult struct {
	CallID  string
	Name    string
	Content string
	IsError bool
}

// Usage holds the token counters for a single round, with neutral names that
// map onto both Anthropic (cache_creation/cache_read) and OpenAI/DeepSeek
// (prompt_tokens_details.cached_tokens) usage fields.
type Usage struct {
	InputTokens      int `json:"inputTokens"`
	OutputTokens     int `json:"outputTokens"`
	CacheReadTokens  int `json:"cacheReadTokens"`
	CacheWriteTokens int `json:"cacheWriteTokens"`
}

// Request is one provider call: stable system prompt + accumulated history + the
// tool set. Effort is honoured by providers that support it (Anthropic), ignored
// otherwise.
type Request struct {
	System   string
	Messages []Message
	Tools    []ToolDef
	Effort   string
}

// Response is the model's reply for one round.
type Response struct {
	Text       string
	ToolCalls  []ToolCall
	Usage      Usage
	StopReason string

	// Raw is the provider-native assistant turn (see Message.Raw). The loop
	// copies it into the assistant Message it appends so the next request can
	// replay it verbatim, preserving e.g. signed thinking blocks.
	Raw any
}

// sumUsage accumulates token counters across rounds.
func sumUsage(a, b Usage) Usage {
	a.InputTokens += b.InputTokens
	a.OutputTokens += b.OutputTokens
	a.CacheReadTokens += b.CacheReadTokens
	a.CacheWriteTokens += b.CacheWriteTokens
	return a
}

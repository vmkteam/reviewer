package direct

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

// defaultResponsesMaxTokens caps each response. Reasoning tokens count against
// max_output_tokens, so it must fit a long reasoning phase plus a large tool
// payload — the same budget as the Anthropic provider.
const defaultResponsesMaxTokens = defaultAnthropicMaxTokens

// responsesProvider drives the OpenAI Responses API. GPT-6 needs it for tool
// use with reasoning (chat completions allows GPT-6 tool calls only on Sol/Luna
// at effort "none"), and it lets reasoning carry across rounds statelessly:
// store=false plus encrypted reasoning items replayed verbatim on the next
// request, the way the Anthropic provider replays signed thinking blocks.
type responsesProvider struct {
	client    *openai.Client
	model     string
	pricing   Pricing
	maxTokens int
}

// NewResponsesProvider builds a provider for the OpenAI Responses API.
// cfg.Temperature is ignored: reasoning models reject sampling parameters.
func NewResponsesProvider(cfg OpenAIConfig) (LLMProvider, error) {
	client, err := newOpenAIClient(cfg)
	if err != nil {
		return nil, err
	}
	return &responsesProvider{
		client:    client,
		model:     cfg.Model,
		pricing:   cfg.Pricing,
		maxTokens: cmp.Or(cfg.MaxTokens, defaultResponsesMaxTokens),
	}, nil
}

func (p *responsesProvider) Model() string    { return p.model }
func (p *responsesProvider) Pricing() Pricing { return p.pricing }

func (p *responsesProvider) Complete(ctx context.Context, req Request) (Response, error) {
	store := false
	rreq := openai.CreateResponseRequest{
		Model:           p.model,
		Instructions:    req.System,
		Input:           toResponsesInput(req),
		Tools:           toResponsesTools(req.Tools),
		MaxOutputTokens: p.maxTokens,
		Store:           &store,
		Include:         []openai.ResponseInclude{openai.ResponseIncludeReasoningEncryptedContent},
	}
	if req.Effort != "" {
		rreq.Reasoning = &openai.ResponseReasoning{Effort: req.Effort}
	}

	resp, err := withRetry(ctx, openaiRetry, req.OnRetry, func() (openai.CreateResponseResponse, error) {
		resp, err := p.client.CreateResponse(ctx, rreq)
		if err == nil && resp.Status == openai.ResponseStatusFailed {
			// HTTP 200 with the failure in the body: classify and retry it like
			// an API error.
			err = newResponseFailedError(resp)
		}
		return resp, err
	})
	if err != nil {
		return Response{}, err
	}

	out, err := parseResponsesOutput(resp.Output)
	if err != nil {
		return Response{}, err
	}
	out.StopReason = responsesStopReason(resp, len(out.ToolCalls) > 0)
	// A cut-off response can end on a reasoning item with nothing after it,
	// which the API rejects on replay: rebuild that turn from Text+ToolCalls.
	if resp.Status != openai.ResponseStatusCompleted {
		out.Raw = nil
	}
	if u := resp.Usage; u != nil {
		var cached, written int
		if d := u.InputTokensDetails; d != nil {
			cached, written = d.CachedTokens, d.CacheWriteTokens
		}
		out.Usage = SplitInput(u.InputTokens, cached, written, u.OutputTokens)
	}
	return out, nil
}

// parseResponsesOutput reads the text and function calls out of the response
// output items and keeps the items themselves (Response.Raw, encoded once) for
// verbatim replay.
func parseResponsesOutput(items []any) (Response, error) {
	var out Response
	replay := make([]any, 0, len(items))
	for _, raw := range items {
		// created_by is output-only provenance the API doesn't accept as input
		// (the official SDKs' toResponseInputItems drops it too).
		if m, ok := raw.(map[string]any); ok {
			delete(m, "created_by")
		}
		data, err := json.Marshal(raw)
		if err != nil {
			return Response{}, fmt.Errorf("openai: encode output item: %w", err)
		}
		var item openai.ResponseOutputItem
		if err := json.Unmarshal(data, &item); err != nil {
			return Response{}, fmt.Errorf("openai: decode output item: %w", err)
		}
		switch item.Type {
		case "message":
			for _, c := range item.Content {
				if c.Type == "output_text" {
					out.Text += c.Text
				}
			}
		case "function_call":
			out.ToolCalls = append(out.ToolCalls, ToolCall{ID: item.CallID, Name: item.Name, Args: json.RawMessage(item.Arguments)})
		}
		replay = append(replay, json.RawMessage(data))
	}
	if len(replay) > 0 {
		out.Raw = replay
	}
	return out, nil
}

// responsesStopReason maps a response to the loop's stop-reason vocabulary; a
// max_output_tokens cut becomes "length" so IsTruncated catches it.
func responsesStopReason(resp openai.CreateResponseResponse, hasToolCalls bool) string {
	if resp.Status == openai.ResponseStatusIncomplete && resp.IncompleteDetails != nil {
		if resp.IncompleteDetails.Reason == "max_output_tokens" {
			return stopLength
		}
		return resp.IncompleteDetails.Reason
	}
	if hasToolCalls {
		return "tool_calls"
	}
	return "stop"
}

// toResponsesInput renders the history as Responses input items. Assistant turns
// replay their kept output items (incl. encrypted reasoning) when available and
// are rebuilt from the neutral fields otherwise (e.g. after compaction).
func toResponsesInput(req Request) []any {
	var items []any
	for _, m := range req.Messages {
		switch m.Role {
		case RoleSystem:
			// Delivered via Instructions; nothing to add to the history.
		case RoleUser:
			items = append(items, openai.ResponseInputMessage{Role: "user", Content: m.Text})
		case RoleAssistant:
			if raw, ok := m.Raw.([]any); ok {
				items = append(items, raw...)
				continue
			}
			if strings.TrimSpace(m.Text) != "" {
				items = append(items, openai.ResponseInputMessage{Role: "assistant", Content: m.Text})
			}
			for _, tc := range m.ToolCalls {
				items = append(items, openai.ResponseOutputItem{Type: "function_call", CallID: tc.ID, Name: tc.Name, Arguments: string(tc.Args)})
			}
		case RoleTool:
			for _, tr := range m.ToolResults {
				items = append(items, openai.ResponseFunctionCallOutput{Type: "function_call_output", CallID: tr.CallID, Output: tr.Content})
			}
		}
	}
	return items
}

func toResponsesTools(defs []ToolDef) []openai.ResponseTool {
	if len(defs) == 0 {
		return nil
	}
	out := make([]openai.ResponseTool, 0, len(defs))
	for _, d := range defs {
		out = append(out, openai.NewResponseFunctionTool(openai.FunctionDefinition{
			Name:        d.Name,
			Description: d.Description,
			Parameters:  d.Schema,
		}))
	}
	return out
}

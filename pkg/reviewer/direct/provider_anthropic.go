package direct

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// defaultAnthropicMaxTokens caps each response. Per-round output (thinking +
// tool calls) is modest; keep under the non-streaming HTTP-timeout ceiling.
const defaultAnthropicMaxTokens = 16000

// AnthropicConfig configures the native Anthropic provider.
type AnthropicConfig struct {
	APIKey    string
	BaseURL   string
	Model     string
	Pricing   Pricing
	Effort    string
	MaxTokens int
}

// anthropicProvider drives the native Anthropic Messages API, with prompt
// caching on the system prompt + tools and adaptive thinking.
type anthropicProvider struct {
	client    anthropic.Client
	model     string
	pricing   Pricing
	effort    string
	maxTokens int64
}

// NewAnthropicProvider builds the native Anthropic provider.
func NewAnthropicProvider(cfg AnthropicConfig) (LLMProvider, error) {
	if cfg.APIKey == "" {
		return nil, errors.New("anthropic provider: API key is required")
	}
	if cfg.Model == "" {
		return nil, errors.New("anthropic provider: model is required")
	}
	opts := []option.RequestOption{option.WithAPIKey(cfg.APIKey)}
	if cfg.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(cfg.BaseURL))
	}
	mt := int64(cfg.MaxTokens)
	if mt <= 0 {
		mt = defaultAnthropicMaxTokens
	}
	return &anthropicProvider{
		client:    anthropic.NewClient(opts...),
		model:     cfg.Model,
		pricing:   cfg.Pricing,
		effort:    cfg.Effort,
		maxTokens: mt,
	}, nil
}

func (p *anthropicProvider) Model() string    { return p.model }
func (p *anthropicProvider) Pricing() Pricing { return p.pricing }

func (p *anthropicProvider) Complete(ctx context.Context, req Request) (Response, error) {
	params := anthropic.MessageNewParams{
		Model:     p.model,
		MaxTokens: p.maxTokens,
		Messages:  toAnthropicMessages(req),
		Thinking:  anthropic.ThinkingConfigParamUnion{OfAdaptive: &anthropic.ThinkingConfigAdaptiveParam{}},
	}
	if req.System != "" {
		// Cache the system prompt (stable prefix) — tools are cached too via the
		// cache_control on the last tool below.
		params.System = []anthropic.TextBlockParam{{
			Text:         req.System,
			CacheControl: anthropic.NewCacheControlEphemeralParam(),
		}}
	}
	if tools := toAnthropicTools(req.Tools); len(tools) > 0 {
		params.Tools = tools
	}
	if eff := cmp.Or(req.Effort, p.effort); eff != "" {
		params.OutputConfig = anthropic.OutputConfigParam{Effort: anthropic.OutputConfigEffort(eff)}
	}

	resp, err := p.client.Messages.New(ctx, params)
	if err != nil {
		return Response{}, fmt.Errorf("anthropic: %w", err)
	}

	out := Response{StopReason: string(resp.StopReason)}
	for _, block := range resp.Content {
		switch v := block.AsAny().(type) {
		case anthropic.TextBlock:
			out.Text += v.Text
		case anthropic.ToolUseBlock:
			out.ToolCalls = append(out.ToolCalls, ToolCall{
				ID:   v.ID,
				Name: v.Name,
				Args: json.RawMessage(v.JSON.Input.Raw()),
			})
		}
	}
	out.Usage = Usage{
		InputTokens:      int(resp.Usage.InputTokens),
		OutputTokens:     int(resp.Usage.OutputTokens),
		CacheReadTokens:  int(resp.Usage.CacheReadInputTokens),
		CacheWriteTokens: int(resp.Usage.CacheCreationInputTokens),
	}
	// Keep the exact assistant turn (incl. signed thinking blocks) so the next
	// request replays it verbatim — rebuilding from Text+ToolCalls would drop the
	// thinking blocks that adaptive thinking + tool use round-trips.
	out.Raw = resp.ToParam()
	return out, nil
}

func toAnthropicMessages(req Request) []anthropic.MessageParam {
	var msgs []anthropic.MessageParam
	for _, m := range req.Messages {
		switch m.Role {
		case RoleSystem:
			// Delivered via params.System; nothing to add to the history.
		case RoleUser:
			msgs = append(msgs, anthropic.NewUserMessage(anthropic.NewTextBlock(m.Text)))
		case RoleAssistant:
			// Replay the verbatim assistant turn when we kept it (preserves signed
			// thinking blocks); otherwise rebuild from the neutral fields.
			if raw, ok := m.Raw.(anthropic.MessageParam); ok {
				msgs = append(msgs, raw)
				continue
			}
			var blocks []anthropic.ContentBlockParamUnion
			if strings.TrimSpace(m.Text) != "" {
				blocks = append(blocks, anthropic.NewTextBlock(m.Text))
			}
			for _, tc := range m.ToolCalls {
				blocks = append(blocks, anthropic.NewToolUseBlock(tc.ID, tc.Args, tc.Name))
			}
			if len(blocks) > 0 {
				msgs = append(msgs, anthropic.NewAssistantMessage(blocks...))
			}
		case RoleTool:
			var blocks []anthropic.ContentBlockParamUnion
			for _, tr := range m.ToolResults {
				blocks = append(blocks, anthropic.NewToolResultBlock(tr.CallID, tr.Content, tr.IsError))
			}
			if len(blocks) > 0 {
				msgs = append(msgs, anthropic.NewUserMessage(blocks...))
			}
		}
	}
	// Rolling cache breakpoint: mark the last block of the conversation so the
	// whole history prefix (the big preload in msg[0] + all accumulated tool
	// results) is cached. Next round reuses it as a prefix — reading it at the
	// cache-read rate (0.1x) and writing only the new delta — instead of
	// re-sending the entire history at the full input price every round.
	markLastBlockCacheable(msgs)
	return msgs
}

// markLastBlockCacheable sets ephemeral cache_control on the final content block
// of the final message, caching the conversation prefix up to and including it.
func markLastBlockCacheable(msgs []anthropic.MessageParam) {
	if len(msgs) == 0 {
		return
	}
	blocks := msgs[len(msgs)-1].Content
	if len(blocks) == 0 {
		return
	}
	cc := anthropic.NewCacheControlEphemeralParam()
	switch b := &blocks[len(blocks)-1]; {
	case b.OfText != nil:
		b.OfText.CacheControl = cc
	case b.OfToolResult != nil:
		b.OfToolResult.CacheControl = cc
	case b.OfToolUse != nil:
		b.OfToolUse.CacheControl = cc
	case b.OfImage != nil:
		b.OfImage.CacheControl = cc
	}
}

func toAnthropicTools(defs []ToolDef) []anthropic.ToolUnionParam {
	if len(defs) == 0 {
		return nil
	}
	out := make([]anthropic.ToolUnionParam, 0, len(defs))
	for i, d := range defs {
		var required []string
		if r, ok := d.Schema[jsRequired].([]string); ok {
			required = r
		}
		tp := &anthropic.ToolParam{
			Name:        d.Name,
			Description: anthropic.String(d.Description),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: d.Schema[jsProps],
				Required:   required,
			},
		}
		// cache_control on the last tool caches the whole tools+system prefix.
		if i == len(defs)-1 {
			tp.CacheControl = anthropic.NewCacheControlEphemeralParam()
		}
		out = append(out, anthropic.ToolUnionParam{OfTool: tp})
	}
	return out
}

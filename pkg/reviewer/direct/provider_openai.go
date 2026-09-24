package direct

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"

	openai "github.com/sashabaranov/go-openai"
)

// defaultMaxTokens caps each response so a long review answer is not silently
// truncated by a low provider default.
const defaultMaxTokens = 8192

// OpenAIConfig configures an OpenAI-protocol provider: chat completions
// (DeepSeek, or any OpenAI-compatible endpoint) or the OpenAI Responses API.
type OpenAIConfig struct {
	APIKey      string
	BaseURL     string // e.g. https://api.deepseek.com/v1
	Model       string
	Pricing     Pricing
	Temperature float32
	MaxTokens   int // per-response output cap; 0 -> defaultMaxTokens
	// PassEffort sends Request.Effort as reasoning_effort. Only for backends
	// known to accept it (DeepSeek); arbitrary OpenAI-compatible ones may 400.
	PassEffort bool
}

// openaiProvider drives an OpenAI-compatible chat-completions API.
type openaiProvider struct {
	client      *openai.Client
	model       string
	pricing     Pricing
	temperature float32
	maxTokens   int
	passEffort  bool
}

// NewOpenAIProvider builds a chat-completions provider for DeepSeek /
// OpenAI-compatible endpoints.
func NewOpenAIProvider(cfg OpenAIConfig) (LLMProvider, error) {
	client, err := newOpenAIClient(cfg)
	if err != nil {
		return nil, err
	}
	return &openaiProvider{
		client:      client,
		model:       cfg.Model,
		pricing:     cfg.Pricing,
		temperature: cfg.Temperature,
		maxTokens:   cmp.Or(cfg.MaxTokens, defaultMaxTokens),
		passEffort:  cfg.PassEffort,
	}, nil
}

// newOpenAIClient validates cfg and builds the go-openai client shared by the
// chat-completions and Responses providers.
func newOpenAIClient(cfg OpenAIConfig) (*openai.Client, error) {
	if cfg.APIKey == "" {
		return nil, errors.New("openai provider: API key is required")
	}
	if cfg.Model == "" {
		return nil, errors.New("openai provider: model is required")
	}
	conf := openai.DefaultConfig(cfg.APIKey)
	if cfg.BaseURL != "" {
		conf.BaseURL = cfg.BaseURL
	}
	return openai.NewClientWithConfig(conf), nil
}

func (p *openaiProvider) Model() string    { return p.model }
func (p *openaiProvider) Pricing() Pricing { return p.pricing }

func (p *openaiProvider) Complete(ctx context.Context, req Request) (Response, error) {
	creq := openai.ChatCompletionRequest{
		Model:       p.model,
		Messages:    toOpenAIMessages(req),
		Tools:       toOpenAITools(req.Tools),
		Temperature: p.temperature,
		// DeepSeek and most OpenAI-compatible backends only document max_tokens,
		// not max_completion_tokens.
		MaxTokens: p.maxTokens, //nolint:staticcheck // see above
	}
	if p.passEffort {
		// Passed through as-is: DeepSeek takes none/low/high/max and maps
		// minimal→low, medium/xhigh→high itself.
		creq.ReasoningEffort = req.Effort
	}

	resp, err := withRetry(ctx, openaiRetry, req.OnRetry, func() (openai.ChatCompletionResponse, error) {
		return p.client.CreateChatCompletion(ctx, creq)
	})
	if err != nil {
		return Response{}, err
	}
	if len(resp.Choices) == 0 {
		return Response{}, errors.New("openai: response had no choices")
	}

	ch := resp.Choices[0]
	out := Response{Text: ch.Message.Content, StopReason: string(ch.FinishReason)}
	for _, tc := range ch.Message.ToolCalls {
		out.ToolCalls = append(out.ToolCalls, ToolCall{
			ID:   tc.ID,
			Name: tc.Function.Name,
			Args: json.RawMessage(tc.Function.Arguments),
		})
	}

	cached := 0
	if d := resp.Usage.PromptTokensDetails; d != nil {
		cached = d.CachedTokens
	}
	out.Usage = SplitInput(resp.Usage.PromptTokens, cached, 0, resp.Usage.CompletionTokens)
	return out, nil
}

func toOpenAIMessages(req Request) []openai.ChatCompletionMessage {
	var msgs []openai.ChatCompletionMessage
	if req.System != "" {
		msgs = append(msgs, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleSystem, Content: req.System})
	}
	for _, m := range req.Messages {
		switch m.Role {
		case RoleSystem:
			// System content is delivered via req.System (rendered first above);
			// a system message in the history is folded there, nothing to add here.
		case RoleUser:
			msgs = append(msgs, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: m.Text})
		case RoleAssistant:
			am := openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: m.Text}
			for _, tc := range m.ToolCalls {
				am.ToolCalls = append(am.ToolCalls, openai.ToolCall{
					ID:       tc.ID,
					Type:     openai.ToolTypeFunction,
					Function: openai.FunctionCall{Name: tc.Name, Arguments: string(tc.Args)},
				})
			}
			// A bare assistant turn (no content and no tool calls) is rejected by
			// the API ("content or tool_calls must be set"); some models (e.g.
			// deepseek-v4-pro) emit one occasionally. Skip it — matching the
			// anthropic provider, which only appends non-empty assistant turns.
			if am.Content == "" && len(am.ToolCalls) == 0 {
				continue
			}
			msgs = append(msgs, am)
		case RoleTool:
			for _, tr := range m.ToolResults {
				msgs = append(msgs, openai.ChatCompletionMessage{
					Role:       openai.ChatMessageRoleTool,
					ToolCallID: tr.CallID,
					Content:    tr.Content,
				})
			}
		}
	}
	return msgs
}

func toOpenAITools(defs []ToolDef) []openai.Tool {
	if len(defs) == 0 {
		return nil
	}
	out := make([]openai.Tool, 0, len(defs))
	for _, d := range defs {
		out = append(out, openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        d.Name,
				Description: d.Description,
				Parameters:  d.Schema,
			},
		})
	}
	return out
}

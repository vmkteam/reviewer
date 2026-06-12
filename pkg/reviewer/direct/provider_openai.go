package direct

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

const (
	// defaultMaxRetries is the transient-error retry budget when not configured.
	defaultMaxRetries = 3
	// defaultMaxTokens caps each response so a long review answer is not silently
	// truncated by a low provider default.
	defaultMaxTokens = 8192
)

// OpenAIConfig configures an OpenAI-compatible provider (DeepSeek, or any
// endpoint speaking the OpenAI chat-completions protocol).
type OpenAIConfig struct {
	APIKey      string
	BaseURL     string // e.g. https://api.deepseek.com/v1
	Model       string
	Pricing     Pricing
	Temperature float32
	MaxRetries  int // transient (429/5xx/network) retry budget; 0 -> defaultMaxRetries
	MaxTokens   int // per-response output cap; 0 -> defaultMaxTokens
}

// openaiProvider drives an OpenAI-compatible chat-completions API.
type openaiProvider struct {
	client      *openai.Client
	model       string
	pricing     Pricing
	temperature float32
	maxRetries  int
	maxTokens   int
}

// NewOpenAIProvider builds a provider for DeepSeek / OpenAI-compatible endpoints.
func NewOpenAIProvider(cfg OpenAIConfig) (LLMProvider, error) {
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
	maxRetries := cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = defaultMaxRetries
	}
	maxTokens := cfg.MaxTokens
	if maxTokens <= 0 {
		maxTokens = defaultMaxTokens
	}
	return &openaiProvider{
		client:      openai.NewClientWithConfig(conf),
		model:       cfg.Model,
		pricing:     cfg.Pricing,
		temperature: cfg.Temperature,
		maxRetries:  maxRetries,
		maxTokens:   maxTokens,
	}, nil
}

func (p *openaiProvider) Model() string    { return p.model }
func (p *openaiProvider) Pricing() Pricing { return p.pricing }

func (p *openaiProvider) Complete(ctx context.Context, req Request) (Response, error) {
	creq := openai.ChatCompletionRequest{
		Model:       p.model,
		Messages:    toOpenAIMessages(req),
		Tools:       toOpenAITools(req.Tools),
		Temperature: p.temperature,
		MaxTokens:   p.maxTokens,
	}

	// Retry transient errors (429 / 5xx / network) with exponential backoff.
	// A non-transient error (400, auth) fails immediately.
	var resp openai.ChatCompletionResponse
	var err error
	backoff := 500 * time.Millisecond
	for attempt := 0; ; attempt++ {
		resp, err = p.client.CreateChatCompletion(ctx, creq)
		if err == nil {
			break
		}
		if attempt >= p.maxRetries || ctx.Err() != nil || !isTransientErr(err) {
			return Response{}, fmt.Errorf("openai: %w", err)
		}
		select {
		case <-ctx.Done():
			return Response{}, ctx.Err()
		case <-time.After(backoff):
		}
		backoff *= 2
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

	// Split cached prompt tokens out of the input count so cost matches the
	// Anthropic semantics (InputTokens = uncached remainder).
	cached := 0
	if resp.Usage.PromptTokensDetails != nil {
		cached = resp.Usage.PromptTokensDetails.CachedTokens
	}
	input := resp.Usage.PromptTokens - cached
	if input < 0 {
		input, cached = resp.Usage.PromptTokens, 0
	}
	out.Usage = Usage{
		InputTokens:     input,
		OutputTokens:    resp.Usage.CompletionTokens,
		CacheReadTokens: cached,
	}
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

// isTransientErr reports whether err is worth retrying: an HTTP 429 / 5xx
// response or a network-level request error.
func isTransientErr(err error) bool {
	var apiErr *openai.APIError
	if errors.As(err, &apiErr) {
		return apiErr.HTTPStatusCode == 429 || apiErr.HTTPStatusCode >= 500
	}
	var reqErr *openai.RequestError
	return errors.As(err, &reqErr)
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

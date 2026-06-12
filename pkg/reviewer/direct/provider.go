package direct

import "context"

// LLMProvider is one backend (Anthropic native, DeepSeek/OpenAI-compatible, …).
// Complete runs a single round: the model sees the system prompt, history and
// tool set, and returns its reply (text + requested tool calls) plus the round's
// token usage.
type LLMProvider interface {
	Complete(ctx context.Context, req Request) (Response, error)
	Model() string
	Pricing() Pricing
}

// Pricing is the per-million-token cost table for one model. DeepSeek values come
// from config; Claude values are the published rates (see the spec).
type Pricing struct {
	InputPerMTok      float64
	OutputPerMTok     float64
	CacheReadPerMTok  float64
	CacheWritePerMTok float64
}

// computeCost returns the USD cost of u under pricing p.
func computeCost(u Usage, p Pricing) float64 {
	const m = 1_000_000.0
	return float64(u.InputTokens)/m*p.InputPerMTok +
		float64(u.OutputTokens)/m*p.OutputPerMTok +
		float64(u.CacheReadTokens)/m*p.CacheReadPerMTok +
		float64(u.CacheWriteTokens)/m*p.CacheWritePerMTok
}

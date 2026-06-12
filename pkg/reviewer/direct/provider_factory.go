package direct

import (
	"fmt"
	"strings"
)

// providerDeepSeek is the default provider id and the DeepSeek model-family
// prefix (the published pricing table keys off the same token).
const providerDeepSeek = "deepseek"

// ProviderConfig selects and configures an LLM backend.
type ProviderConfig struct {
	Provider    string // "deepseek" (default) | "openai" | "anthropic"
	Model       string
	BaseURL     string
	APIKey      string
	Temperature float32
	Pricing     Pricing // optional override; falls back to pricingFor(Model)
}

// NewProvider builds an LLMProvider from cfg. The native Anthropic provider is
// added in M1; until then "anthropic" returns an error.
func NewProvider(cfg ProviderConfig) (LLMProvider, error) {
	pricing := cfg.Pricing
	if pricing == (Pricing{}) {
		pricing = pricingFor(cfg.Model)
	}
	switch strings.ToLower(cfg.Provider) {
	case "", providerDeepSeek:
		base := cfg.BaseURL
		if base == "" {
			base = "https://api.deepseek.com"
		}
		return NewOpenAIProvider(OpenAIConfig{APIKey: cfg.APIKey, BaseURL: base, Model: cfg.Model, Pricing: pricing, Temperature: cfg.Temperature})
	case "openai", "openai-compat":
		return NewOpenAIProvider(OpenAIConfig{APIKey: cfg.APIKey, BaseURL: cfg.BaseURL, Model: cfg.Model, Pricing: pricing, Temperature: cfg.Temperature})
	case "anthropic":
		// effort flows through Request.Effort (from DirectRunner.Effort).
		return NewAnthropicProvider(AnthropicConfig{APIKey: cfg.APIKey, BaseURL: cfg.BaseURL, Model: cfg.Model, Pricing: pricing})
	default:
		return nil, fmt.Errorf("unknown provider %q", cfg.Provider)
	}
}

// pricingFor returns published Claude per-MTok pricing for known model families,
// or a zero table (cost reported as 0) for anything else, e.g. DeepSeek — set
// ProviderConfig.Pricing to override.
func pricingFor(model string) Pricing {
	switch {
	case strings.HasPrefix(model, "claude-opus"), strings.HasPrefix(model, "claude-fable"):
		return Pricing{InputPerMTok: 5, OutputPerMTok: 25, CacheReadPerMTok: 0.5, CacheWritePerMTok: 6.25}
	case strings.HasPrefix(model, "claude-sonnet"):
		return Pricing{InputPerMTok: 3, OutputPerMTok: 15, CacheReadPerMTok: 0.3, CacheWritePerMTok: 3.75}
	case strings.HasPrefix(model, "claude-haiku"):
		return Pricing{InputPerMTok: 1, OutputPerMTok: 5, CacheReadPerMTok: 0.1, CacheWritePerMTok: 1.25}
	case strings.HasPrefix(model, "deepseek-v4-pro"):
		// V4 Pro current (permanently-discounted) rates, USD/MTok. Cache hits are
		// billed at CacheRead; DeepSeek has no separate cache-write charge (a miss
		// is just the input price).
		return Pricing{InputPerMTok: 0.435, OutputPerMTok: 0.87, CacheReadPerMTok: 0.003625, CacheWritePerMTok: 0.435}
	case strings.HasPrefix(model, "deepseek-v4-flash"):
		// V4 Flash rates (also the deepseek-chat/reasoner compatibility aliases).
		return Pricing{InputPerMTok: 0.14, OutputPerMTok: 0.28, CacheReadPerMTok: 0.0028, CacheWritePerMTok: 0.14}
	case strings.HasPrefix(model, providerDeepSeek):
		// Legacy deepseek-chat/reasoner alias to V4 Flash (deprecating 2026-07-24).
		return Pricing{InputPerMTok: 0.14, OutputPerMTok: 0.28, CacheReadPerMTok: 0.0028, CacheWritePerMTok: 0.14}
	default:
		return Pricing{}
	}
}

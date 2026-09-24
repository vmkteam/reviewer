package direct

import (
	"cmp"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Provider ids accepted by NewProvider for the direct runner. ProviderDeepSeek
// doubles as the DeepSeek model-family prefix (the published pricing table keys
// off the same token); the empty provider id defaults to DeepSeek.
const (
	ProviderDeepSeek     = "deepseek"
	ProviderOpenAI       = "openai"
	ProviderOpenAICompat = "openai-compat"
	ProviderAnthropic    = "anthropic"
)

// validProviders is the set of explicit (non-empty) provider ids NewProvider
// accepts. Defined next to the switch below so the two can't drift, and reused
// by IsValidProvider so the VT admin validates against this single source.
var validProviders = map[string]bool{
	ProviderDeepSeek:     true,
	ProviderOpenAI:       true,
	ProviderOpenAICompat: true,
	ProviderAnthropic:    true,
}

// IsValidProvider reports whether name is an explicit provider id that
// NewProvider accepts (case-insensitive, matching NewProvider). The empty
// default is intentionally excluded — callers validating a user-entered value
// should treat "" as "unset", not as a valid provider.
func IsValidProvider(name string) bool {
	return validProviders[strings.ToLower(name)]
}

// DefaultCompactAt returns the history-compaction threshold suited to a
// provider's context window. Anthropic's 1M-token models defer compaction
// nearly to the end of a review — compacting mid-run rewrites the whole prompt
// cache; other providers keep the conservative Options default (returned 0 means
// "use DefaultOptions"). Lives next to NewProvider so
// provider-capability knowledge stays in one place.
func DefaultCompactAt(provider string) int {
	if strings.EqualFold(provider, ProviderAnthropic) {
		return CompactAtLargeContext
	}
	return 0
}

// ProviderConfig selects and configures an LLM backend.
type ProviderConfig struct {
	Provider    string // "deepseek" (default) | "openai" | "openai-compat" | "anthropic"
	Model       string
	BaseURL     string
	APIKey      string
	Temperature float32
	Pricing     Pricing // optional override; falls back to PricingFor(Model, now)
}

// NewProvider builds an LLMProvider from cfg.
func NewProvider(cfg ProviderConfig) (LLMProvider, error) {
	pricing := cfg.Pricing
	if pricing == (Pricing{}) {
		// Priced at run start: DeepSeek's peak/off-peak rate is picked once, so a
		// run straddling a window boundary is billed at its starting rate.
		pricing = PricingFor(cfg.Model, time.Now())
	}
	oc := OpenAIConfig{APIKey: cfg.APIKey, BaseURL: cfg.BaseURL, Model: cfg.Model, Pricing: pricing, Temperature: cfg.Temperature}
	provider := strings.ToLower(cfg.Provider)
	switch provider {
	case "", ProviderDeepSeek:
		oc.BaseURL = cmp.Or(oc.BaseURL, "https://api.deepseek.com")
		oc.MaxTokens, oc.PassEffort = deepseekMaxTokens, true
		return NewOpenAIProvider(oc)
	case ProviderOpenAI, ProviderOpenAICompat:
		// OpenAI itself goes through the Responses API: GPT-6 tool use with
		// reasoning needs it, and chat completions rejects max_tokens on gpt-5+.
		// A custom base URL (Azure, LiteLLM and other proxies) may only speak
		// chat completions, so it keeps that protocol, like openai-compat.
		if provider == ProviderOpenAI && isOpenAIEndpoint(cfg.BaseURL) {
			return NewResponsesProvider(oc)
		}
		return NewOpenAIProvider(oc)
	case ProviderAnthropic:
		// effort flows through Request.Effort (from DirectRunner.Effort).
		return NewAnthropicProvider(AnthropicConfig{APIKey: cfg.APIKey, BaseURL: cfg.BaseURL, Model: cfg.Model, Pricing: pricing})
	default:
		return nil, fmt.Errorf("unknown provider %q", cfg.Provider)
	}
}

// isOpenAIEndpoint reports whether baseURL is OpenAI's own API (the default when
// empty) rather than a proxy that may not implement the Responses API.
func isOpenAIEndpoint(baseURL string) bool {
	if baseURL == "" {
		return true
	}
	u, err := url.Parse(baseURL)
	return err == nil && strings.EqualFold(u.Hostname(), "api.openai.com")
}

// deepseekMaxTokens is the DeepSeek per-response output cap. Thinking tokens
// count against max_tokens, and the API's own defaults are 64K in thinking mode
// and 128K at reasoning_effort "max" — the shared 8K default would cut reasoning.
const deepseekMaxTokens = 128 * 1024

// openAIPricing lists OpenAI standard-tier rates (USD/MTok, as of 2026-09) by
// exact model id — a prefix match would price gpt-5.5-pro as gpt-5.5. Models
// without a cache-write charge bill written tokens as plain input. The
// long-context surcharge (requests over 272K input tokens) is not modelled.
var openAIPricing = map[string]Pricing{
	"gpt-6-astra":   {InputPerMTok: 10, OutputPerMTok: 50, CacheReadPerMTok: 1, CacheWritePerMTok: 12.5},
	"gpt-6-sol":     {InputPerMTok: 2, OutputPerMTok: 10, CacheReadPerMTok: 0.2, CacheWritePerMTok: 2.5},
	"gpt-6-luna":    {InputPerMTok: 0.1, OutputPerMTok: 0.5, CacheReadPerMTok: 0.01, CacheWritePerMTok: 0.125},
	"gpt-5.6":       {InputPerMTok: 4, OutputPerMTok: 20, CacheReadPerMTok: 0.4, CacheWritePerMTok: 5}, // bare alias routes to Sol
	"gpt-5.6-sol":   {InputPerMTok: 4, OutputPerMTok: 20, CacheReadPerMTok: 0.4, CacheWritePerMTok: 5}, // promotional, through at least 2026-11-21
	"gpt-5.6-terra": {InputPerMTok: 2, OutputPerMTok: 12, CacheReadPerMTok: 0.2, CacheWritePerMTok: 2.5},
	"gpt-5.6-luna":  {InputPerMTok: 0.2, OutputPerMTok: 1.2, CacheReadPerMTok: 0.02, CacheWritePerMTok: 0.25},
	"gpt-5.5":       {InputPerMTok: 5, OutputPerMTok: 30, CacheReadPerMTok: 0.5, CacheWritePerMTok: 5},
	"gpt-5.5-pro":   {InputPerMTok: 30, OutputPerMTok: 180, CacheReadPerMTok: 30, CacheWritePerMTok: 30},
	"gpt-5.4":       {InputPerMTok: 2.5, OutputPerMTok: 15, CacheReadPerMTok: 0.25, CacheWritePerMTok: 2.5},
	"gpt-5.4-mini":  {InputPerMTok: 0.75, OutputPerMTok: 4.5, CacheReadPerMTok: 0.075, CacheWritePerMTok: 0.75},
	"gpt-5.4-nano":  {InputPerMTok: 0.2, OutputPerMTok: 1.25, CacheReadPerMTok: 0.02, CacheWritePerMTok: 0.2},
	"gpt-5.4-pro":   {InputPerMTok: 30, OutputPerMTok: 180, CacheReadPerMTok: 30, CacheWritePerMTok: 30},
	"gpt-5.3-codex": {InputPerMTok: 1.75, OutputPerMTok: 14, CacheReadPerMTok: 0.175, CacheWritePerMTok: 1.75},
}

// PricingFor returns published per-MTok pricing (as of 2026-09) for known
// Claude, OpenAI and DeepSeek models at time at, or a zero table (cost reported
// as 0) for anything else — set ProviderConfig.Pricing to override. It is also
// the price table the codex runner estimates its cost from. More specific
// prefixes must precede their family (claude-opus-5-5 before claude-opus).
func PricingFor(model string, at time.Time) Pricing {
	if p, ok := openAIPricing[model]; ok {
		return p
	}
	switch {
	case strings.HasPrefix(model, "claude-fable-5-1"):
		// Fable 5 rates except cache reads, which drop to 0.025x input.
		return Pricing{InputPerMTok: 10, OutputPerMTok: 50, CacheReadPerMTok: 0.25, CacheWritePerMTok: 12.5}
	case strings.HasPrefix(model, "claude-fable"):
		return Pricing{InputPerMTok: 10, OutputPerMTok: 50, CacheReadPerMTok: 1, CacheWritePerMTok: 12.5}
	case strings.HasPrefix(model, "claude-opus-5-5"):
		// Cheaper than Opus 5, with cache reads at 0.05x input.
		return Pricing{InputPerMTok: 4, OutputPerMTok: 20, CacheReadPerMTok: 0.2, CacheWritePerMTok: 5}
	case strings.HasPrefix(model, "claude-opus"):
		return Pricing{InputPerMTok: 5, OutputPerMTok: 25, CacheReadPerMTok: 0.5, CacheWritePerMTok: 6.25}
	case strings.HasPrefix(model, "claude-sonnet-5"):
		return Pricing{InputPerMTok: 2, OutputPerMTok: 10, CacheReadPerMTok: 0.2, CacheWritePerMTok: 2.5}
	case strings.HasPrefix(model, "claude-sonnet"):
		return Pricing{InputPerMTok: 3, OutputPerMTok: 15, CacheReadPerMTok: 0.3, CacheWritePerMTok: 3.75}
	case strings.HasPrefix(model, "claude-haiku"):
		return Pricing{InputPerMTok: 1, OutputPerMTok: 5, CacheReadPerMTok: 0.1, CacheWritePerMTok: 1.25}
	case strings.HasPrefix(model, "deepseek-v4-pro"):
		// Peak rates, USD/MTok. Cache hits are billed at CacheRead; DeepSeek has
		// no separate cache-write charge (a miss is just the input price).
		return deepseekPricing(Pricing{InputPerMTok: 1.32, OutputPerMTok: 3.96, CacheReadPerMTok: 0.044, CacheWritePerMTok: 1.32}, at)
	case strings.HasPrefix(model, ProviderDeepSeek):
		// deepseek-flash (V4.1 Flash) peak rates; the retired deepseek-v4-flash
		// and deepseek-chat/reasoner names are served and billed as Flash.
		return deepseekPricing(Pricing{InputPerMTok: 0.3, OutputPerMTok: 1.2, CacheReadPerMTok: 0.006, CacheWritePerMTok: 0.3}, at)
	default:
		return Pricing{}
	}
}

// deepseekPricing returns the peak table during DeepSeek's peak window and half
// of it otherwise.
func deepseekPricing(peak Pricing, at time.Time) Pricing {
	if deepseekPeak(at) {
		return peak
	}
	return Pricing{
		InputPerMTok:      peak.InputPerMTok / 2,
		OutputPerMTok:     peak.OutputPerMTok / 2,
		CacheReadPerMTok:  peak.CacheReadPerMTok / 2,
		CacheWritePerMTok: peak.CacheWritePerMTok / 2,
	}
}

// deepseekPeak reports whether t is in DeepSeek's peak billing window: 01:00–04:00
// and 06:00–10:00 UTC, Monday to Friday. Chinese public holidays are off-peak
// too but not modelled, so runs on those days are over-estimated.
func deepseekPeak(t time.Time) bool {
	t = t.UTC()
	if wd := t.Weekday(); wd == time.Saturday || wd == time.Sunday {
		return false
	}
	h := t.Hour()
	return (h >= 1 && h < 4) || (h >= 6 && h < 10)
}

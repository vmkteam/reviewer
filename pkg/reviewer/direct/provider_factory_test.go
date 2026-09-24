package direct

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPricingFor(t *testing.T) {
	now := time.Now()
	require.Equal(t, Pricing{InputPerMTok: 5, OutputPerMTok: 25, CacheReadPerMTok: 0.5, CacheWritePerMTok: 6.25}, PricingFor("claude-opus-4-8", now))
	require.Equal(t, Pricing{InputPerMTok: 5, OutputPerMTok: 25, CacheReadPerMTok: 0.5, CacheWritePerMTok: 6.25}, PricingFor("claude-opus-5", now))
	require.Equal(t, Pricing{InputPerMTok: 4, OutputPerMTok: 20, CacheReadPerMTok: 0.2, CacheWritePerMTok: 5}, PricingFor("claude-opus-5-5", now))
	require.Equal(t, Pricing{InputPerMTok: 10, OutputPerMTok: 50, CacheReadPerMTok: 1, CacheWritePerMTok: 12.5}, PricingFor("claude-fable-5", now))
	require.Equal(t, Pricing{InputPerMTok: 10, OutputPerMTok: 50, CacheReadPerMTok: 0.25, CacheWritePerMTok: 12.5}, PricingFor("claude-fable-5-1", now))
	require.Equal(t, Pricing{InputPerMTok: 2, OutputPerMTok: 10, CacheReadPerMTok: 0.2, CacheWritePerMTok: 2.5}, PricingFor("claude-sonnet-5", now))
	require.InEpsilon(t, 3.0, PricingFor("claude-sonnet-4-6", now).InputPerMTok, 1e-9)
	require.InEpsilon(t, 1.0, PricingFor("claude-haiku-4-5", now).InputPerMTok, 1e-9)
	// OpenAI: exact ids (gpt-5.5-pro must not price as gpt-5.5); no cache-write
	// charge -> writes bill as input.
	require.Equal(t, Pricing{InputPerMTok: 2, OutputPerMTok: 10, CacheReadPerMTok: 0.2, CacheWritePerMTok: 2.5}, PricingFor("gpt-6-sol", now))
	require.InEpsilon(t, 180.0, PricingFor("gpt-5.5-pro", now).OutputPerMTok, 1e-9)
	require.InEpsilon(t, 5.0, PricingFor("gpt-5.5", now).CacheWritePerMTok, 1e-9)
	require.Equal(t, Pricing{}, PricingFor("gpt-5-codex", now)) // shut down 2026-07-23
	// Unknown model -> zero table (cost reported as 0), no panic.
	require.Equal(t, Pricing{}, PricingFor("gpt-4o", now))
	require.Equal(t, Pricing{}, PricingFor("", now))
}

func TestPricingForDeepSeekPeak(t *testing.T) {
	peak := time.Date(2026, 9, 23, 7, 30, 0, 0, time.UTC)    // Wednesday 07:30 UTC
	offPeak := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC) // Wednesday 12:00 UTC
	weekend := time.Date(2026, 9, 26, 7, 30, 0, 0, time.UTC) // Saturday, same hour

	require.Equal(t, Pricing{InputPerMTok: 1.32, OutputPerMTok: 3.96, CacheReadPerMTok: 0.044, CacheWritePerMTok: 1.32}, PricingFor("deepseek-v4-pro", peak))
	require.InEpsilon(t, 0.66, PricingFor("deepseek-v4-pro", offPeak).InputPerMTok, 1e-9)
	require.InEpsilon(t, 0.022, PricingFor("deepseek-v4-pro", weekend).CacheReadPerMTok, 1e-9)

	require.Equal(t, Pricing{InputPerMTok: 0.3, OutputPerMTok: 1.2, CacheReadPerMTok: 0.006, CacheWritePerMTok: 0.3}, PricingFor("deepseek-flash", peak))
	require.InEpsilon(t, 0.6, PricingFor("deepseek-flash", offPeak).OutputPerMTok, 1e-9)
	// Retired names are served and billed as deepseek-flash.
	require.Equal(t, PricingFor("deepseek-flash", peak), PricingFor("deepseek-v4-flash", peak))
	require.Equal(t, PricingFor("deepseek-flash", offPeak), PricingFor("deepseek-chat", offPeak))

	// Window edges: [01:00, 04:00) and [06:00, 10:00) UTC, in any time zone.
	day := func(h, m int) time.Time { return time.Date(2026, 9, 21, h, m, 0, 0, time.UTC) } // Monday
	require.False(t, deepseekPeak(day(0, 59)))
	require.True(t, deepseekPeak(day(1, 0)))
	require.False(t, deepseekPeak(day(4, 0)))
	require.True(t, deepseekPeak(day(9, 59)))
	require.False(t, deepseekPeak(day(10, 0)))
	require.True(t, deepseekPeak(day(7, 0).In(time.FixedZone("MSK", 3*3600))))
}

func TestNewProviderUnknownAndAnthropic(t *testing.T) {
	// anthropic now builds a native provider.
	p, err := NewProvider(ProviderConfig{Provider: "anthropic", Model: "claude-opus-4-8", APIKey: "k"})
	require.NoError(t, err)
	require.Equal(t, "claude-opus-4-8", p.Model())

	_, err = NewProvider(ProviderConfig{Provider: "bogus", Model: "m", APIKey: "k"})
	require.ErrorContains(t, err, "unknown provider")
}

func TestNewProviderValidatesConfig(t *testing.T) {
	// deepseek with empty model -> error from the underlying openai provider.
	_, err := NewProvider(ProviderConfig{Provider: "deepseek", Model: "", APIKey: "k"})
	require.ErrorContains(t, err, "model is required")

	// Missing API key -> error.
	_, err = NewProvider(ProviderConfig{Provider: "deepseek", Model: "deepseek-chat", APIKey: ""})
	require.ErrorContains(t, err, "API key is required")
}

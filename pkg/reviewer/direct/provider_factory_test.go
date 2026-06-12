package direct

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPricingFor(t *testing.T) {
	require.Equal(t, Pricing{InputPerMTok: 5, OutputPerMTok: 25, CacheReadPerMTok: 0.5, CacheWritePerMTok: 6.25}, pricingFor("claude-opus-4-8"))
	require.Equal(t, Pricing{InputPerMTok: 5, OutputPerMTok: 25, CacheReadPerMTok: 0.5, CacheWritePerMTok: 6.25}, pricingFor("claude-fable-5"))
	require.InEpsilon(t, 3.0, pricingFor("claude-sonnet-4-6").InputPerMTok, 1e-9)
	require.InEpsilon(t, 1.0, pricingFor("claude-haiku-4-5").InputPerMTok, 1e-9)
	require.InEpsilon(t, 0.27, pricingFor("deepseek-chat").InputPerMTok, 1e-9)
	require.InEpsilon(t, 0.07, pricingFor("deepseek-reasoner").CacheReadPerMTok, 1e-9)
	// Unknown model -> zero table (cost reported as 0), no panic.
	require.Equal(t, Pricing{}, pricingFor("gpt-4o"))
	require.Equal(t, Pricing{}, pricingFor(""))
}

func TestNewProviderUnknownAndAnthropic(t *testing.T) {
	_, err := NewProvider(ProviderConfig{Provider: "anthropic", Model: "claude-opus-4-8", APIKey: "k"})
	require.ErrorContains(t, err, "not implemented")

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

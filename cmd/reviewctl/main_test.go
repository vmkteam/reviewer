package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDirectKeyEnvs(t *testing.T) {
	require.Equal(t, []string{"REVIEW_API_KEY", "ANTHROPIC_API_KEY"}, directKeyEnvs("anthropic"))
	require.Equal(t, []string{"REVIEW_API_KEY", "ANTHROPIC_API_KEY"}, directKeyEnvs("Anthropic")) // case-insensitive
	require.Equal(t, []string{"REVIEW_API_KEY", "OPENAI_API_KEY"}, directKeyEnvs("openai-compat"))
	require.Equal(t, []string{"REVIEW_API_KEY", "DEEPSEEK_API_KEY"}, directKeyEnvs("deepseek"))
	require.Equal(t, []string{"REVIEW_API_KEY", "DEEPSEEK_API_KEY"}, directKeyEnvs("")) // default
}

func TestDirectAPIKey(t *testing.T) {
	t.Run("REVIEW_API_KEY overrides any provider", func(t *testing.T) {
		t.Setenv("REVIEW_API_KEY", "universal")
		require.Equal(t, "universal", directAPIKey("openai-compat"))
		require.Equal(t, "universal", directAPIKey("anthropic"))
	})

	t.Run("provider-specific fallback", func(t *testing.T) {
		t.Setenv("REVIEW_API_KEY", "")
		t.Setenv("OPENAI_API_KEY", "oai")
		require.Equal(t, "oai", directAPIKey("openai-compat"))
	})

	t.Run("openai-compat no longer borrows DEEPSEEK_API_KEY", func(t *testing.T) {
		t.Setenv("REVIEW_API_KEY", "")
		t.Setenv("OPENAI_API_KEY", "")
		t.Setenv("DEEPSEEK_API_KEY", "ds")
		require.Empty(t, directAPIKey("openai-compat"))
		require.Equal(t, "ds", directAPIKey("deepseek"))
	})
}

package main

import (
	"slices"
	"testing"

	"reviewsrv/pkg/reviewer/ctl"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServerPanelMembers(t *testing.T) {
	rc := &ctl.ReviewConfig{
		Primary: &ctl.ResolvedProfile{Runner: "claude", Model: "opus", RunnerProfileID: 1, Token: "tp"},
		Panel: []*ctl.ResolvedProfile{
			{Runner: "codex", Model: "gpt-5.5", RunnerProfileID: 2, Token: "tm"},
			{Runner: "opencode", Model: "deepseek-v4", RunnerProfileID: 3},
		},
		Judge: &ctl.ResolvedProfile{Runner: "claude", Model: "opus", RunnerProfileID: 9, Token: "tj"},
	}

	members := serverPanelMembers(rc)
	require.Len(t, members, 3, "the full run set is [primary] + panel")
	assert.Equal(t, "claude", members[0].Runner)
	require.NotNil(t, members[0].Profile)
	assert.Equal(t, 1, members[0].Profile.RunnerProfileID, "primary is the first member")
	assert.Equal(t, "codex", members[1].Runner)
	assert.Equal(t, "tm", members[1].Profile.Token, "each member carries its own token")
	assert.Equal(t, "opencode", members[2].Runner)

	j := profileMember(rc.Judge)
	assert.Equal(t, "claude", j.Runner)
	assert.Equal(t, "opus", j.Model)
	require.NotNil(t, j.Profile)
	assert.Equal(t, 9, j.Profile.RunnerProfileID)
}

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

func TestApplyProfile(t *testing.T) {
	p := &ctl.ResolvedProfile{Runner: "direct", Model: "claude-opus-5-5", RunnerProfileID: 5, Title: "direct",
		Params: ctl.RunnerProfileParams{MaxRounds: 120, AllowDangerousPermissions: true}}
	passed := func(flags ...string) func(string) bool {
		return func(name string) bool { return slices.Contains(flags, name) }
	}

	cfg := &ctl.Config{Runner: "claude", MaxRounds: 0}
	applyProfile(passed(), cfg, p)
	assert.Equal(t, "direct", cfg.Runner)
	assert.Equal(t, 120, cfg.MaxRounds, "the profile's budget applies without the flag")
	assert.True(t, cfg.AllowDangerousPermissions)

	cfg = &ctl.Config{Runner: "codex", MaxRounds: 0}
	applyProfile(passed("runner", "max-rounds"), cfg, p)
	assert.Equal(t, "codex", cfg.Runner, "an explicit flag wins")
	assert.Zero(t, cfg.MaxRounds, "an explicit --max-rounds 0 (the default) wins too")
	assert.True(t, cfg.MaxRoundsSet, "and overrides the panel members' profiles")

	cfg = &ctl.Config{MaxRounds: 200}
	applyProfile(passed("max-rounds"), cfg, &ctl.ResolvedProfile{})
	assert.Equal(t, 200, cfg.MaxRounds)
	applyProfile(passed(), cfg, &ctl.ResolvedProfile{})
	assert.Equal(t, 200, cfg.MaxRounds, "a profile without a budget leaves the value alone")

	full := &ctl.ResolvedProfile{Runner: "direct", Model: "m", Effort: "high", APIProvider: "anthropic", APIBaseURL: "https://proxy",
		Params: ctl.RunnerProfileParams{AllowDangerousPermissions: false}}
	cfg = &ctl.Config{Model: "flag-model", AllowDangerousPermissions: true}
	applyProfile(passed("model", "allow-dangerous-permissions"), cfg, full)
	assert.Equal(t, ctl.Config{Runner: "direct", Model: "flag-model", Effort: "high", APIProvider: "anthropic",
		APIBaseURL: "https://proxy", AllowDangerousPermissions: true}, *cfg, "string fields and a passed bool flag")
}

func TestExplicitFlagsCountsMaxRoundsEnv(t *testing.T) {
	none := func(string) bool { return false }
	t.Setenv(envMaxRounds, "")
	assert.False(t, explicitFlags(none)("max-rounds"))

	t.Setenv(envMaxRounds, "200") // how CI sets the budget
	assert.True(t, explicitFlags(none)("max-rounds"))
	assert.False(t, explicitFlags(none)("model"), "other runner settings come from the profile in CI")
}

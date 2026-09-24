package runner

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCredEnv(t *testing.T) {
	const v = "REVIEWSRV_TEST_CRED_VAR"

	t.Run("empty token injects nothing", func(t *testing.T) {
		t.Setenv(v, "") // ambient unset
		assert.Nil(t, credEnv(v, ""))
	})

	t.Run("token injected when ambient var is unset", func(t *testing.T) {
		t.Setenv(v, "")
		assert.Equal(t, []string{v + "=tok"}, credEnv(v, "tok"))
	})

	t.Run("ambient env wins over the profile token", func(t *testing.T) {
		t.Setenv(v, "ambient")
		assert.Nil(t, credEnv(v, "tok"), "an already-set env var is not overridden")
	})
}

func TestClaudeIsolationEnv(t *testing.T) {
	t.Setenv("CLAUDE_CODE_DISABLE_AUTO_MEMORY", "")
	assert.Equal(t, []string{"CLAUDE_CODE_DISABLE_AUTO_MEMORY=1"}, claudeIsolationEnv())

	t.Setenv("CLAUDE_CODE_DISABLE_AUTO_MEMORY", "0")
	assert.Nil(t, claudeIsolationEnv(), "the operator's env wins")
}

func TestRunExecScrubsCISecrets(t *testing.T) {
	t.Setenv("REVIEWER_GITLAB_TOKEN", "glpat-secret")
	t.Setenv("ANTHROPIC_API_KEY", "sk-ambient")
	out := runExec(t.Context(), slog.New(slog.DiscardHandler), "sh", t.TempDir(),
		[]string{"-c", `echo "gitlab=${REVIEWER_GITLAB_TOKEN-unset} key=$ANTHROPIC_API_KEY tracker=$REVIEW_TRACKER_TOKEN"`},
		"", []string{"REVIEW_TRACKER_TOKEN=tracker", "ANTHROPIC_API_KEY=sk-extra"}, nil)
	require.NoError(t, out.err)
	assert.Equal(t, "gitlab=unset key=sk-extra tracker=tracker\n", out.stdout.String(),
		"CI secrets are dropped; an extra var wins over the ambient one")
}

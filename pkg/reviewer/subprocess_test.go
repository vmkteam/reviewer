package reviewer

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChildEnv(t *testing.T) {
	t.Setenv(EnvGitLabToken, "glpat-secret")
	t.Setenv(EnvProjectKey, "key")
	t.Setenv(EnvAPIKey, "sk-review")
	t.Setenv("CI_JOB_TOKEN", "job-secret")
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant")

	vars := map[string]string{}
	for _, kv := range ChildEnv("REVIEW_TRACKER_TOKEN=tracker") {
		name, value, _ := strings.Cut(kv, "=")
		vars[name] = value
	}
	for _, name := range []string{EnvGitLabToken, EnvProjectKey, EnvAPIKey, "CI_JOB_TOKEN"} {
		assert.NotContains(t, vars, name)
	}
	assert.Equal(t, "sk-ant", vars["ANTHROPIC_API_KEY"], "provider keys stay: the CLIs read them")
	assert.Equal(t, "tracker", vars["REVIEW_TRACKER_TOKEN"])
	assert.Contains(t, vars, "PATH")
}

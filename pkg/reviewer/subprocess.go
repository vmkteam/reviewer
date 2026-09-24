package reviewer

import (
	"os"
	"slices"
	"strings"
)

// ciSecretEnv lists variables no subprocess on the checkout needs but a
// prompt-injected agent could exfiltrate: reviewctl's own credentials and
// GitLab's job secrets. LLM provider keys stay — the runner CLIs read them.
// Secrets under names the operator picks (CI/CD variables, id_tokens) are not
// covered, and the job token also sits in the checkout's git remote URL.
var ciSecretEnv = []string{
	EnvGitLabToken, EnvProjectKey, EnvAPIKey,
	"CI_JOB_TOKEN", "CI_REPOSITORY_URL",
	"CI_REGISTRY_PASSWORD", "CI_DEPLOY_PASSWORD", "CI_DEPENDENCY_PROXY_PASSWORD",
}

// ChildEnv is the environment of a subprocess reviewctl starts on the untrusted
// checkout — a runner CLI, git: the ambient one without ciSecretEnv, plus extra.
// extra comes last, and for a key also in the ambient env (a merged
// OPENCODE_CONFIG_CONTENT) os/exec keeps the last value. Defense in depth, not a
// boundary: a process under the same uid can still read reviewctl's own
// /proc/<pid>/environ; only a separate uid or PID namespace closes that.
func ChildEnv(extra ...string) []string {
	env := slices.DeleteFunc(os.Environ(), func(kv string) bool {
		name, _, _ := strings.Cut(kv, "=")
		return slices.Contains(ciSecretEnv, name)
	})
	return append(env, extra...)
}

// GitArgs prefixes the arguments of a git command reviewctl runs on the
// checkout: an agent can write hooks and an fsmonitor command into its .git
// (shared by every panel worktree), which git would otherwise execute.
func GitArgs(args ...string) []string {
	return append([]string{"-c", "core.hooksPath=/dev/null", "-c", "core.fsmonitor=false"}, args...)
}

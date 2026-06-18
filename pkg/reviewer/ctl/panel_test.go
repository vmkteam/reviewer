package ctl

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"reviewsrv/pkg/reviewer/runner"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRunner is a ReviewRunner that leaves the on-disk review.json skeleton as-is
// (so ReadReviewJSON sees a valid empty review) or fails when err is set.
type fakeRunner struct {
	name string
	err  error
}

func (f *fakeRunner) Run(context.Context, string) (*runner.ClaudeResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &runner.ClaudeResult{}, nil
}
func (f *fakeRunner) Name() string      { return f.name }
func (f *fakeRunner) SetSession(string) {}

func TestParseMulti(t *testing.T) {
	t.Run("empty is nil, no panel", func(t *testing.T) {
		got, err := ParseMulti("")
		require.NoError(t, err)
		assert.Nil(t, got)

		got, err = ParseMulti("   ")
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("parses runner:model members, preserving slashes in model", func(t *testing.T) {
		got, err := ParseMulti("codex:gpt-5.5, opencode:openrouter/deepseek/deepseek-v4-pro")
		require.NoError(t, err)
		assert.Equal(t, []MemberSpec{
			{Runner: runner.RunnerCodex, Model: "gpt-5.5"},
			{Runner: runner.RunnerOpenCode, Model: "openrouter/deepseek/deepseek-v4-pro"},
		}, got)
	})

	t.Run("bare runner uses default model", func(t *testing.T) {
		got, err := ParseMulti("claude")
		require.NoError(t, err)
		assert.Equal(t, []MemberSpec{{Runner: runner.RunnerClaude, Model: ""}}, got)
	})

	t.Run("duplicates are allowed (self-fusion)", func(t *testing.T) {
		got, err := ParseMulti("direct:claude-opus-4-8,direct:claude-opus-4-8")
		require.NoError(t, err)
		assert.Len(t, got, 2)
	})

	t.Run("unknown runner errors", func(t *testing.T) {
		_, err := ParseMulti("gemini:flash")
		require.Error(t, err)
	})
}

func TestGitWorktreeAddRemove(t *testing.T) {
	// Hermetic check of the worktree mechanics (no runner/LLM): a member worktree
	// is created detached at the commit with the repo content, then fully removed.
	repo := t.TempDir()
	runGit(t, repo, "init", "-q")
	require.NoError(t, os.WriteFile(filepath.Join(repo, "file.txt"), []byte("hi"), 0o600))
	runGit(t, repo, "add", ".")
	runGit(t, repo, "-c", "user.email=t@example.com", "-c", "user.name=t", "commit", "-q", "-m", "init")

	c := &Controller{cfg: &Config{Dir: repo}, log: slog.Default()}
	wt := filepath.Join(t.TempDir(), "wt-1")

	require.NoError(t, c.gitWorktreeAdd(t.Context(), wt, "HEAD"))
	assert.FileExists(t, filepath.Join(wt, "file.txt"), "worktree must contain the committed tree")

	c.gitWorktreeRemove(t.Context(), wt)
	assert.NoDirExists(t, wt, "worktree dir must be gone after remove")
}

func TestRunMembers_ToleratesFailures(t *testing.T) {
	c := &Controller{
		cfg: &Config{
			Multi: []MemberSpec{
				{Runner: runner.RunnerClaude, Model: "ok-a"},
				{Runner: runner.RunnerClaude, Model: "boom"},
				{Runner: runner.RunnerClaude, Model: "ok-b"},
			},
		},
		log: slog.Default(),
		runnerFactory: func(mc *Config) (runner.ReviewRunner, error) {
			if mc.Model == "boom" {
				return &fakeRunner{name: runner.RunnerClaude, err: errors.New("kaboom")}, nil
			}
			return &fakeRunner{name: runner.RunnerClaude}, nil
		},
	}
	dirs := []string{t.TempDir(), t.TempDir(), t.TempDir()}
	labels := []string{"ok-a", "boom", "ok-b"}

	out := c.runMembers(t.Context(), dirs, labels, "prompt")
	require.Len(t, out, 2, "the failing member is tolerated, the rest succeed")
	assert.Equal(t, "ok-a", out[0].label, "successful members keep panel order")
	assert.Equal(t, "ok-b", out[1].label)
}

func TestWithTimeout(t *testing.T) {
	ctx, cancel := withTimeout(t.Context(), 0)
	defer cancel()
	_, ok := ctx.Deadline()
	assert.False(t, ok, "0 duration → no deadline")

	ctx2, cancel2 := withTimeout(t.Context(), time.Minute)
	defer cancel2()
	_, ok = ctx2.Deadline()
	assert.True(t, ok, "positive duration → deadline set")
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "git %v: %s", args, out)
}

func TestMemberLabel(t *testing.T) {
	assert.Equal(t, "1-codex-gpt-5.5", memberLabel(0, MemberSpec{Runner: "codex", Model: "gpt-5.5"}))
	// duplicates stay distinct by index
	assert.Equal(t, "2-codex-gpt-5.5", memberLabel(1, MemberSpec{Runner: "codex", Model: "gpt-5.5"}))
	// slashes in the model become path-safe
	assert.Equal(t, "1-opencode-openrouter-deepseek-deepseek-v4-pro",
		memberLabel(0, MemberSpec{Runner: "opencode", Model: "openrouter/deepseek/deepseek-v4-pro"}))
	// bare runner, no model
	assert.Equal(t, "1-claude", memberLabel(0, MemberSpec{Runner: "claude"}))
}

func TestSourceLabels(t *testing.T) {
	got := sourceLabels([]MemberSpec{
		{Runner: "codex", Model: "gpt-5.5"},
		{Runner: "opencode", Model: "openrouter/deepseek/deepseek-v4-pro"},
		{Runner: "direct", Model: "gpt-5.5"}, // duplicate model → -2 suffix keeps it distinct
		{Runner: "claude"},                   // no model → runner name
	})
	assert.Equal(t, []string{
		"gpt-5.5",
		"openrouter-deepseek-deepseek-v4-pro",
		"gpt-5.5-2",
		"claude",
	}, got)
}

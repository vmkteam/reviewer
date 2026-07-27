package ctl

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"reviewsrv/pkg/rest"
	"reviewsrv/pkg/reviewer/runner"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRunner is a ReviewRunner that fills the on-disk review.json with one
// issue (when dir is set), leaves the skeleton untouched (dir empty — an
// "empty review" runner), or fails outright when err is set.
type fakeRunner struct {
	name string
	dir  string
	err  error
}

func (f *fakeRunner) Run(context.Context, string) (*runner.ClaudeResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.dir != "" {
		draft, err := ReadReviewJSON(f.dir)
		if err != nil {
			return nil, err
		}
		draft.Issues = append(draft.Issues, rest.ReviewDraftIssue{
			LocalID: "C1", Severity: "low", Title: "t", FileType: "code",
		})
		if err := WriteReviewJSON(f.dir, draft); err != nil {
			return nil, err
		}
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
	// Failed members must ship their artifacts as a debug bundle BEFORE panel
	// cleanup wipes the worktree (its only copy) — count the uploads.
	var bundles atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/v1/upload/debug/") {
			bundles.Add(1)
		}
		_, _ = w.Write([]byte(`{"id":"x","url":"/v1/debug/storage/x/"}`))
	}))
	t.Cleanup(srv.Close)

	c := NewController(&Config{
		URL: srv.URL, Key: "proj-key",
		Multi: []MemberSpec{
			{Runner: runner.RunnerClaude, Model: "ok-a"},
			{Runner: runner.RunnerClaude, Model: "boom"},
			{Runner: runner.RunnerClaude, Model: "empty"},
			{Runner: runner.RunnerClaude, Model: "ok-b"},
		},
	}, nil, slog.Default())
	c.runnerFactory = func(mc *Config) (runner.ReviewRunner, error) {
		switch mc.Model {
		case "boom":
			return &fakeRunner{name: runner.RunnerClaude, err: errors.New("kaboom")}, nil
		case "empty": // exits cleanly but never touches review.json
			return &fakeRunner{name: runner.RunnerClaude}, nil
		default:
			return &fakeRunner{name: runner.RunnerClaude, dir: mc.Dir}, nil
		}
	}
	dirs := []string{t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()}
	labels := []string{"ok-a", "boom", "empty", "ok-b"}

	out := c.runMembers(t.Context(), dirs, labels, "prompt")
	require.Len(t, out, 2, "the erroring and the empty-review members are both excluded")
	assert.Equal(t, "ok-a", out[0].label, "successful members keep panel order")
	assert.Equal(t, "ok-b", out[1].label)
	assert.EqualValues(t, 2, bundles.Load(), "boom and empty members must each upload a debug bundle")
}

// judgeRunner leaves the skeleton untouched on the first run (an "empty
// review" judge flap) and fills review.json on the second.
type judgeRunner struct {
	dir  string
	runs int
}

func (j *judgeRunner) Run(context.Context, string) (*runner.ClaudeResult, error) {
	j.runs++
	if j.runs == 1 {
		return &runner.ClaudeResult{}, nil
	}
	draft, err := ReadReviewJSON(j.dir)
	if err != nil {
		return nil, err
	}
	draft.Issues = append(draft.Issues, rest.ReviewDraftIssue{LocalID: "F1", Severity: "low", Title: "fused", FileType: "code"})
	if err := WriteReviewJSON(j.dir, draft); err != nil {
		return nil, err
	}
	return &runner.ClaudeResult{}, nil
}
func (j *judgeRunner) Name() string      { return runner.RunnerClaude }
func (j *judgeRunner) SetSession(string) {}

func TestRunJudge_EmptyFirstAttemptRetries(t *testing.T) {
	// A member worktree with its produced review, to be staged for the judge.
	mdir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(mdir, "review.json"), []byte(`{"issues":[]}`), 0o600))
	mdPath := filepath.Join(mdir, "R1.m.ai.md")
	require.NoError(t, os.WriteFile(mdPath, []byte("body"), 0o600))
	outputs := []*memberOutput{{label: "opus", dir: mdir, mdFiles: map[string]string{"architecture": mdPath}}}

	judgeDir := t.TempDir()
	jr := &judgeRunner{dir: judgeDir}
	c := NewController(&Config{Judge: &MemberSpec{Runner: runner.RunnerClaude}}, nil, slog.Default())
	c.runnerFactory = func(*Config) (runner.ReviewRunner, error) { return jr, nil }

	fusion, err := c.runJudge(t.Context(), judgeDir, "fuse the members", outputs)
	require.NoError(t, err)
	require.Equal(t, 2, jr.runs, "an empty judge review must be retried, not uploaded")
	require.Len(t, fusion.draft.Issues, 1)

	// Members staged where the fusion prompt expects them, survived the retry's
	// artifact cleanup (CleanReviewArtifacts only touches the dir root).
	assert.FileExists(t, filepath.Join(judgeDir, "members", "opus", "review.json"))
	assert.FileExists(t, filepath.Join(judgeDir, "members", "opus", "R1.m.ai.md"))
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

func TestMemberConfigInheritsTracker(t *testing.T) {
	c := NewController(&Config{
		Runner:       "claude",
		TrackerURL:   "https://yt.example.com",
		TrackerToken: "tracker-tok",
	}, nil, slog.Default())

	mc := c.memberConfig("/tmp/wt", MemberSpec{Runner: "codex", Model: "m", Profile: &ResolvedProfile{Token: "profile-tok"}})

	// Tracker access is project-wide: the profile overlay must not touch it.
	assert.Equal(t, "https://yt.example.com", mc.TrackerURL)
	assert.Equal(t, "tracker-tok", mc.TrackerToken)
	assert.Equal(t, "profile-tok", mc.Token, "profile credentials still overlaid")
	assert.Equal(t, "codex", mc.Runner)
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

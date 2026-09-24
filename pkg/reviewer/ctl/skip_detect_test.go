package ctl

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"reviewsrv/pkg/rest"
	"reviewsrv/pkg/reviewer"
	"reviewsrv/pkg/reviewer/runner"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeReviewJSON(t *testing.T, path string, draft *rest.ReviewDraft) error {
	t.Helper()
	data, err := json.Marshal(draft)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func TestIsReviewJSONUnfilled(t *testing.T) {
	tests := []struct {
		name  string
		draft *rest.ReviewDraft
		want  bool
	}{
		{
			name:  "nil draft",
			draft: nil,
			want:  false,
		},
		{
			name: "empty files and no issues — unfilled",
			draft: &rest.ReviewDraft{
				Files:  []rest.ReviewDraftFile{},
				Issues: []rest.ReviewDraftIssue{},
			},
			want: true,
		},
		{
			name: "all summaries blank and no issues — unfilled",
			draft: &rest.ReviewDraft{
				Files: []rest.ReviewDraftFile{
					{ReviewType: "code", Summary: ""},
					{ReviewType: "tests", Summary: ""},
				},
				Issues: []rest.ReviewDraftIssue{},
			},
			want: true,
		},
		{
			name: "summary whitespace only — counts as blank, unfilled",
			draft: &rest.ReviewDraft{
				Files: []rest.ReviewDraftFile{
					{ReviewType: "code", Summary: "  \n\t  "},
				},
				Issues: []rest.ReviewDraftIssue{},
			},
			want: true,
		},
		{
			name: "one non-blank summary — filled",
			draft: &rest.ReviewDraft{
				Files: []rest.ReviewDraftFile{
					{ReviewType: "code", Summary: "looks fine"},
					{ReviewType: "tests", Summary: ""},
				},
				Issues: []rest.ReviewDraftIssue{},
			},
			want: false,
		},
		{
			name: "issues present even with blank summaries — filled",
			draft: &rest.ReviewDraft{
				Files: []rest.ReviewDraftFile{
					{ReviewType: "code", Summary: ""},
				},
				Issues: []rest.ReviewDraftIssue{{LocalID: "C1"}},
			},
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, isReviewJSONUnfilled(tc.draft))
		})
	}
}

// retryStep2RunnerStub records SetSession and Run calls so retryStep2 can be
// driven without spawning a real CLI.
type retryStep2RunnerStub struct {
	sessionSet string
	runCalled  bool
	runErr     error
	beforeRun  func() error // simulates the runner editing review.json on disk
}

func (r *retryStep2RunnerStub) Name() string         { return "stub" }
func (r *retryStep2RunnerStub) SetSession(id string) { r.sessionSet = id }
func (r *retryStep2RunnerStub) Run(_ context.Context, _ string) (*runner.ClaudeResult, error) {
	r.runCalled = true
	if r.beforeRun != nil {
		if err := r.beforeRun(); err != nil {
			return nil, err
		}
	}
	if r.runErr != nil {
		return nil, r.runErr
	}
	return &runner.ClaudeResult{}, nil
}

func TestRetryStep2(t *testing.T) {
	t.Run("empty sessionId fails as not submitted and skips runner", func(t *testing.T) {
		stub := &retryStep2RunnerStub{}
		c := &Controller{
			cfg:    &Config{Dir: t.TempDir()},
			log:    slog.Default(),
			runner: trackSpend(stub),
		}
		draft, err := c.retryStep2(context.Background(), "")
		require.ErrorContains(t, err, "no session to resume")
		_, reason := reviewer.RunOutcome(err)
		assert.Equal(t, reviewer.RunReasonNotSubmitted, reason)
		assert.Nil(t, draft)
		assert.False(t, stub.runCalled, "runner.Run must not be called when sessionId empty")
		assert.Empty(t, stub.sessionSet, "SetSession must not be called when sessionId empty")
	})

	t.Run("nil runner fails without panic", func(t *testing.T) {
		c := &Controller{
			cfg: &Config{Dir: t.TempDir()},
			log: slog.Default(),
		}
		draft, err := c.retryStep2(context.Background(), "ses_123")
		require.Error(t, err)
		assert.Nil(t, draft)
	})

	t.Run("runner error is returned", func(t *testing.T) {
		stub := &retryStep2RunnerStub{runErr: errors.New("boom")}
		c := &Controller{
			cfg:    &Config{Dir: t.TempDir()},
			log:    slog.Default(),
			runner: trackSpend(stub),
		}
		draft, err := c.retryStep2(context.Background(), "ses_123")
		require.ErrorContains(t, err, "boom")
		assert.Nil(t, draft)
		assert.Equal(t, "ses_123", stub.sessionSet, "SetSession must be called before Run")
	})

	t.Run("happy path: runner fills review.json on disk", func(t *testing.T) {
		dir := t.TempDir()
		// Write a minimal valid review.json for ReadReviewJSON to parse.
		stub := &retryStep2RunnerStub{
			beforeRun: func() error {
				return writeReviewJSON(t, filepath.Join(dir, "review.json"), &rest.ReviewDraft{
					Files: []rest.ReviewDraftFile{
						{ReviewType: "code", Summary: "filled by retry", IsAccepted: true},
					},
					Issues: []rest.ReviewDraftIssue{},
				})
			},
		}
		c := &Controller{
			cfg:    &Config{Dir: dir},
			log:    slog.Default(),
			runner: trackSpend(stub),
		}
		draft, err := c.retryStep2(context.Background(), "ses_abc")
		require.NoError(t, err)
		require.NotNil(t, draft, "expected non-nil draft after successful retry")
		assert.Equal(t, "ses_abc", stub.sessionSet)
		require.Len(t, draft.Files, 1)
		assert.Equal(t, "filled by retry", draft.Files[0].Summary)
	})
}

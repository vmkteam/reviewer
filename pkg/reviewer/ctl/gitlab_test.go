package ctl

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"reviewsrv/pkg/rest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testDraft(t *testing.T) *rest.ReviewDraft {
	t.Helper()
	draft, err := ReadReviewJSON("testdata")
	require.NoError(t, err)
	return draft
}

func TestPostSummaryComment(t *testing.T) {
	var gotAuth, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		assert.True(t, strings.HasSuffix(r.URL.Path, "/notes"), "path = %q, want suffix /notes", r.URL.Path)
		body, _ := io.ReadAll(r.Body)
		var payload map[string]string
		json.Unmarshal(body, &payload)
		gotBody = payload["body"]
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	cfg := &Config{
		GitLabToken: "test-token",
		GitLabURL:   srv.URL,
		ProjectID:   "123",
		MRIID:       "42",
	}

	g := NewGitLabClient(cfg, slog.Default())
	draft := testDraft(t)
	err := g.PostSummaryComment(context.Background(), draft, "https://reviewer.example.com/reviews/1/")
	require.NoError(t, err)
	assert.Equal(t, "Bearer test-token", gotAuth)
	assert.Contains(t, gotBody, "Code Review")
	assert.Contains(t, gotBody, "Full review")
}

func TestPostInlineComment(t *testing.T) {
	var gotPath string
	var gotPayload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &gotPayload)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	cfg := &Config{
		GitLabToken: "test-token",
		GitLabURL:   srv.URL,
		ProjectID:   "123",
		MRIID:       "42",
		DiffBaseSHA: "base-sha-123",
		Commit:      "head-sha-456",
	}

	g := NewGitLabClient(cfg, slog.Default())
	issue := rest.ReviewDraftIssue{
		LocalID:     "C1",
		Severity:    "critical",
		Title:       "Missing error handling",
		Description: "Handler ignores error",
		File:        "pkg/api/handler.go",
		Lines:       "42-45",
		IssueType:   "error-handling",
	}

	err := g.PostInlineCommentWithFallback(context.Background(), issue)
	require.NoError(t, err)
	assert.True(t, strings.HasSuffix(gotPath, "/discussions"), "path = %q, want suffix /discussions", gotPath)

	pos, ok := gotPayload["position"].(map[string]any)
	require.True(t, ok, "position not found in payload")
	assert.Equal(t, "pkg/api/handler.go", pos["new_path"])
	assert.EqualValues(t, 42, pos["new_line"])
	assert.Equal(t, "base-sha-123", pos["base_sha"])
	assert.Equal(t, "head-sha-456", pos["head_sha"])
}

func TestPostInlineComment_Fallback(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if strings.HasSuffix(r.URL.Path, "/discussions") {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"message":"400 Bad request"}`))
			return
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	cfg := &Config{
		GitLabToken: "test-token",
		GitLabURL:   srv.URL,
		ProjectID:   "123",
		MRIID:       "42",
		DiffBaseSHA: "base-sha",
		Commit:      "head-sha",
	}

	g := NewGitLabClient(cfg, slog.Default())
	issue := rest.ReviewDraftIssue{
		LocalID:   "C1",
		Severity:  "critical",
		Title:     "Test issue",
		File:      "main.go",
		Lines:     "10",
		IssueType: "error-handling",
	}

	err := g.PostInlineCommentWithFallback(context.Background(), issue)
	require.NoError(t, err)

	require.Len(t, paths, 2, "expected 2 requests (discussion + fallback note)")
	assert.True(t, strings.HasSuffix(paths[0], "/discussions"), "first request path = %q, want /discussions", paths[0])
	assert.True(t, strings.HasSuffix(paths[1], "/notes"), "second request path = %q, want /notes", paths[1])
}

func TestPostInlineComment_NoFileOrLines(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	cfg := &Config{
		GitLabToken: "test-token",
		GitLabURL:   srv.URL,
		ProjectID:   "123",
		MRIID:       "42",
	}

	g := NewGitLabClient(cfg, slog.Default())
	issue := rest.ReviewDraftIssue{
		LocalID:   "C1",
		Severity:  "critical",
		Title:     "Test issue",
		File:      "",
		Lines:     "",
		IssueType: "tests",
	}

	err := g.PostInlineCommentWithFallback(context.Background(), issue)
	require.NoError(t, err)
	// Should fall back to note, not discussion.
	assert.True(t, strings.HasSuffix(gotPath, "/notes"), "path = %q, want suffix /notes (fallback)", gotPath)
}

func TestPostMRComment_NoToken(t *testing.T) {
	cfg := &Config{} // no token
	assert.False(t, cfg.HasGitLab())
}

func TestRenderSummaryComment(t *testing.T) {
	tests := []struct {
		name     string
		issues   []rest.ReviewDraftIssue
		contains []string
	}{
		{
			name: "red light",
			issues: []rest.ReviewDraftIssue{
				{Severity: "critical", LocalID: "C1", Title: "Bug", File: "main.go", Lines: "1", IssueType: "error-handling", FileType: "code"},
			},
			contains: []string{"🔴", "Red Light", "C1"},
		},
		{
			name: "yellow light",
			issues: []rest.ReviewDraftIssue{
				{Severity: "high", LocalID: "H1", Title: "Issue", FileType: "code", IssueType: "naming"},
			},
			contains: []string{"🟡", "Yellow Light"},
		},
		{
			name:     "green light",
			issues:   nil,
			contains: []string{"🟢", "Green Light"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			draft := &rest.ReviewDraft{}
			draft.Review.ModelInfo.Model = "opus"
			draft.Review.ModelInfo.CostUsd = 1.52
			draft.Review.DurationMs = 93000
			draft.Review.EffortMinutes = 15
			draft.Files = append(draft.Files, struct {
				ReviewType string `json:"reviewType"`
				Summary    string `json:"summary"`
				IsAccepted bool   `json:"isAccepted"`
			}{ReviewType: "code", Summary: "Review", IsAccepted: true})
			draft.Issues = tt.issues

			body, err := renderSummaryComment(draft, "https://example.com/reviews/1/")
			require.NoError(t, err)

			for _, s := range tt.contains {
				assert.Contains(t, body, s)
			}
		})
	}
}

func TestParseLinePosition(t *testing.T) {
	tests := []struct {
		input string
		want  int
		ok    bool
	}{
		{"42-45", 42, true},
		{"42", 42, true},
		{"", 0, false},
		{"abc", 0, false},
		{"0", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, ok := parseLinePosition(tt.input)
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

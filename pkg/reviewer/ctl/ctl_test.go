package ctl

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"reviewsrv/pkg/rest"
	"reviewsrv/pkg/reviewer/runner"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testClaudeRunner returns a fixed ClaudeResult from testdata. The optional
// beforeRun hook lets a test simulate side effects (e.g. Claude overwriting
// the skeleton with a broken review.json) before the result is returned.
type testClaudeRunner struct {
	fixturePath string
	beforeRun   func() error
	sessionID   string // captured from SetSession
}

func (r *testClaudeRunner) Run(_ context.Context, _ string) (*runner.ClaudeResult, error) {
	if r.beforeRun != nil {
		if err := r.beforeRun(); err != nil {
			return nil, err
		}
	}
	data, err := os.ReadFile(r.fixturePath)
	if err != nil {
		return nil, err
	}
	return runner.ParseClaudeResult(data)
}

func (r *testClaudeRunner) Name() string                { return runner.RunnerClaude }
func (r *testClaudeRunner) SetSession(sessionID string) { r.sessionID = sessionID }

// setupTestDir copies testdata files to a temp dir for upload tests.
func setupTestDir(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()

	files := []string{"review.json", "R1.architecture.md", "R2.code.md", "R3.security.md", "R4.tests.md"}
	for _, f := range files {
		data, err := os.ReadFile(filepath.Join("testdata", f))
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(tmpDir, f), data, 0o644)
		require.NoError(t, err)
	}

	return tmpDir
}

func TestController_Upload(t *testing.T) {
	var uploadedReview bool
	var uploadedFiles []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		// POST /v1/reviewctl/upload/{projectKey}/ — review
		if len(parts) == 4 && r.Method == http.MethodPost {
			body, _ := io.ReadAll(r.Body)
			var draft map[string]any
			json.Unmarshal(body, &draft)
			uploadedReview = true
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("42"))
			return
		}
		// POST /v1/reviewctl/upload/{projectKey}/{reviewId}/{type}/ — file
		if len(parts) == 6 && r.Method == http.MethodPost {
			uploadedFiles = append(uploadedFiles, parts[5])
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	tmpDir := setupTestDir(t)

	cfg := &Config{
		Key: "test-key",
		URL: srv.URL,
		Dir: tmpDir,
	}

	c := NewController(cfg, nil, slog.Default())
	err := c.Upload(context.Background())
	require.NoError(t, err)

	assert.True(t, uploadedReview, "review.json was not uploaded")
	assert.Len(t, uploadedFiles, 4)

	// Verify HTML was generated.
	htmlPath := filepath.Join(tmpDir, "review.html")
	_, err = os.Stat(htmlPath)
	assert.False(t, os.IsNotExist(err), "review.html was not generated")
}

func TestController_Review(t *testing.T) {
	promptCalled := false
	var uploadedReview bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		// POST /v1/reviewctl/rpc/ — Prompt
		if path == "/v1/reviewctl/rpc/" && r.Method == http.MethodPost {
			promptCalled = true
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "result": "Review %SOURCE_BRANCH% to %TARGET_BRANCH%", "id": 1})
			return
		}
		// POST /v1/reviewctl/upload/{key}/
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) == 4 && r.Method == http.MethodPost {
			uploadedReview = true
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("42"))
			return
		}
		if len(parts) == 6 && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	tmpDir := setupTestDir(t)

	cfg := &Config{
		Key:          "test-key",
		URL:          srv.URL,
		Model:        "opus",
		Dir:          tmpDir,
		SourceBranch: "feature/test",
		TargetBranch: "master",
	}

	runner := &testClaudeRunner{fixturePath: "testdata/claude_result.json"}
	c := NewController(cfg, runner, slog.Default())

	err := c.Review(context.Background())
	require.NoError(t, err)

	assert.True(t, promptCalled, "prompt was not fetched")
	assert.True(t, uploadedReview, "review was not uploaded")
}

func TestController_Review_UploadsDebugBundleOnValidationFailure(t *testing.T) {
	var debugUploaded bool
	var debugError string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/v1/reviewctl/rpc/" && r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "result": "prompt", "id": 1})
			return
		}
		if strings.HasPrefix(path, "/v1/upload/debug/") && r.Method == http.MethodPost {
			if !assert.NoError(t, r.ParseMultipartForm(32<<20)) {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			debugUploaded = true
			if v := r.MultipartForm.Value["errorMsg"]; len(v) > 0 {
				debugError = v[0]
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"x","url":"/v1/debug/storage/x/"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	// Simulate Claude overwriting the skeleton with an invalid review.json
	// — reproduces the CI failure where the model picks empty reviewType.
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "claude-output.json"), []byte(`{"type":"result"}`), 0o644))

	cfg := &Config{Key: "test-key", URL: srv.URL, Model: "opus", Dir: tmpDir, Runner: runner.RunnerClaude}
	corrupted := []byte(`{"review":{"title":"x"},"files":[{"reviewType":"","summary":"s"}],"issues":[]}`)
	runner := &testClaudeRunner{
		fixturePath: "testdata/claude_result.json",
		beforeRun: func() error {
			return os.WriteFile(filepath.Join(tmpDir, "review.json"), corrupted, 0o644)
		},
	}
	c := NewController(cfg, runner, slog.Default())

	err := c.Review(context.Background())
	require.Error(t, err, "Review must fail on invalid review.json")
	assert.Contains(t, err.Error(), "invalid reviewType")
	assert.True(t, debugUploaded, "debug bundle must be uploaded on failure")
	assert.Contains(t, debugError, "files[0]", "errorMsg must carry verbose validation detail")
}

func TestController_Comment(t *testing.T) {
	var commentPosted bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/notes") {
			commentPosted = true
			w.WriteHeader(http.StatusCreated)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/discussions") {
			w.WriteHeader(http.StatusCreated)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	tmpDir := setupTestDir(t)

	cfg := &Config{
		Key:         "test-key",
		URL:         "https://reviewer.example.com",
		Dir:         tmpDir,
		ReviewID:    42,
		GitLabToken: "test-token",
		GitLabURL:   srv.URL,
		ProjectID:   "123",
		MRIID:       "42",
		DiffBaseSHA: "base-sha",
		Commit:      "head-sha",
	}

	c := NewController(cfg, nil, slog.Default())
	err := c.Comment(context.Background())
	require.NoError(t, err)

	assert.True(t, commentPosted, "MR comment was not posted")
}

func TestFillMetadataClearsPlaceholdersAndFillsFromGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q", "-b", "feature/x"},
		{"config", "user.email", "dev@example.com"},
		{"config", "user.name", "Dev Author"},
		{"commit", "-q", "--allow-empty", "-m", "init"},
	} {
		out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
		require.NoError(t, err, string(out))
	}

	c := NewController(&Config{Dir: dir}, nil, slog.Default())
	draft := &rest.ReviewDraft{}
	draft.Review.ExternalID = PlaceholderExternalID
	draft.Review.Author = PlaceholderAuthor
	draft.Review.SourceBranch = PlaceholderSourceBranch
	draft.Review.TargetBranch = PlaceholderTargetBranch
	draft.Review.CommitHash = PlaceholderCommitHash
	draft.Review.Title = PlaceholderTitle

	c.fillMetadata(context.Background(), draft)

	// Placeholders cleared; author/branch/commit recovered from git; fields with
	// no local source stay empty rather than leaking literal placeholders.
	require.Equal(t, "Dev Author", draft.Review.Author)
	require.Equal(t, "feature/x", draft.Review.SourceBranch)
	require.Equal(t, gitMeta(context.Background(), dir, "rev-parse", "HEAD"), draft.Review.CommitHash)
	require.NotEmpty(t, draft.Review.CommitHash)
	require.Empty(t, draft.Review.TargetBranch)
	require.Empty(t, draft.Review.Title)
	require.Empty(t, draft.Review.ExternalID)

	// The second Title placeholder variant is cleared too.
	d2 := &rest.ReviewDraft{}
	d2.Review.Title = PlaceholderMRTitle
	c.fillMetadata(context.Background(), d2)
	require.Empty(t, d2.Review.Title)
}

func TestWriteReviewJSONRoundTrip(t *testing.T) {
	dir := t.TempDir()
	draft := &rest.ReviewDraft{}
	draft.Review.Description = "done"
	draft.Review.ModelInfo.Model = "claude-opus-5"
	draft.Review.ModelInfo.CostUsd = 1.5
	draft.Review.DurationMs = 4200

	require.NoError(t, WriteReviewJSON(dir, draft))
	got, _ := ReadReviewJSON(dir) // validation outcome is irrelevant here — the metadata must round-trip
	require.NotNil(t, got)
	require.Equal(t, "claude-opus-5", got.Review.ModelInfo.Model)
	require.InEpsilon(t, 1.5, got.Review.ModelInfo.CostUsd, 1e-9)
	require.Equal(t, 4200, got.Review.DurationMs)
}

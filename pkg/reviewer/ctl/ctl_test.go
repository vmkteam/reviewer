package ctl

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testClaudeRunner returns a fixed ClaudeResult from testdata.
type testClaudeRunner struct {
	fixturePath string
}

func (r *testClaudeRunner) Run(_ context.Context, _ string) (*ClaudeResult, error) {
	data, err := os.ReadFile(r.fixturePath)
	if err != nil {
		return nil, err
	}
	return ParseClaudeResult(data)
}

// setupTestDir copies testdata files to a temp dir for upload tests.
func setupTestDir(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()

	files := []string{"review.json", "R1.architecture.md", "R2.code.md", "R3.security.md", "R4.tests.md"}
	for _, f := range files {
		data, err := os.ReadFile(filepath.Join("testdata", f))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(tmpDir, f), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	return tmpDir
}

func TestController_Upload(t *testing.T) {
	var uploadedReview bool
	var uploadedFiles []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		// POST /v1/upload/{projectKey}/ — review
		if len(parts) == 3 && r.Method == http.MethodPost {
			body, _ := io.ReadAll(r.Body)
			var draft map[string]any
			json.Unmarshal(body, &draft)
			uploadedReview = true
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("42"))
			return
		}
		// POST /v1/upload/{projectKey}/{reviewId}/{type}/ — file
		if len(parts) == 5 && r.Method == http.MethodPost {
			uploadedFiles = append(uploadedFiles, parts[4])
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !uploadedReview {
		t.Error("review.json was not uploaded")
	}
	if len(uploadedFiles) != 4 {
		t.Errorf("uploaded %d files, want 4", len(uploadedFiles))
	}

	// Verify HTML was generated.
	htmlPath := filepath.Join(tmpDir, "review.html")
	if _, err := os.Stat(htmlPath); os.IsNotExist(err) {
		t.Error("review.html was not generated")
	}
}

func TestController_Review(t *testing.T) {
	promptCalled := false
	var uploadedReview bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		// GET /v1/prompt/{key}/
		if strings.HasPrefix(path, "/v1/prompt/") && r.Method == http.MethodGet {
			promptCalled = true
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Review %SOURCE_BRANCH% to %TARGET_BRANCH%"))
			return
		}
		// POST /v1/upload/{key}/
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) == 3 && r.Method == http.MethodPost {
			uploadedReview = true
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("42"))
			return
		}
		if len(parts) == 5 && r.Method == http.MethodPost {
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !promptCalled {
		t.Error("prompt was not fetched")
	}
	if !uploadedReview {
		t.Error("review was not uploaded")
	}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !commentPosted {
		t.Error("MR comment was not posted")
	}
}

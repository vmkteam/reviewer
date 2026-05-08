package ctl

import (
	"compress/gzip"
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

	"reviewsrv/pkg/rest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUploadReview(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.True(t, strings.HasPrefix(r.URL.Path, "/v1/upload/test-key/"), "path = %q, want prefix /v1/upload/test-key/", r.URL.Path)

		body, _ := io.ReadAll(r.Body)
		var draft rest.ReviewDraft
		err := json.Unmarshal(body, &draft)
		assert.NoError(t, err)
		assert.NotEmpty(t, draft.Review.Title)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("42"))
	}))
	defer srv.Close()

	draft, err := ReadReviewJSON("testdata")
	require.NoError(t, err)

	c := NewUploadClient(slog.Default())
	reviewID, err := c.UploadReview(context.Background(), srv.URL, "test-key", draft)
	require.NoError(t, err)
	assert.Equal(t, 42, reviewID)
}

func TestUploadFile(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/octet-stream", r.Header.Get("Content-Type"))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewUploadClient(slog.Default())
	err := c.UploadFile(context.Background(), srv.URL, "test-key", 42, "architecture", []byte("# Architecture"))
	require.NoError(t, err)
	assert.Equal(t, "/v1/upload/test-key/42/architecture/", gotPath)
}

func TestUploadReview_ServerError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{"server error", http.StatusInternalServerError},
		{"not found", http.StatusNotFound},
		{"bad request", http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte("error"))
			}))
			defer srv.Close()

			draft, _ := ReadReviewJSON("testdata")
			c := NewUploadClient(slog.Default())
			_, err := c.UploadReview(context.Background(), srv.URL, "test-key", draft)
			require.Error(t, err)
		})
	}
}

func TestReadReviewJSON(t *testing.T) {
	draft, err := ReadReviewJSON("testdata")
	require.NoError(t, err)
	assert.Equal(t, "Add user authentication", draft.Review.Title)
	assert.Len(t, draft.Files, 4)
	assert.Len(t, draft.Issues, 2)
}

func TestReadReviewJSON_Minimal(t *testing.T) {
	tmpDir := t.TempDir()
	data, err := os.ReadFile("testdata/review_minimal.json")
	require.NoError(t, err)
	writeErr := os.WriteFile(filepath.Join(tmpDir, "review.json"), data, 0o644)
	require.NoError(t, writeErr)

	draft, err := ReadReviewJSON(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, "Fix typo", draft.Review.Title)
	assert.Empty(t, draft.Issues)
}

func TestReadReviewJSON_ReturnsDraftOnValidationError(t *testing.T) {
	tmpDir := t.TempDir()
	// Valid JSON but empty reviewType — must reproduce the CI failure shape.
	body := []byte(`{"review":{"title":"x"},"files":[{"reviewType":"","summary":"s"}],"issues":[]}`)
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "review.json"), body, 0o644))

	draft, err := ReadReviewJSON(tmpDir)
	require.Error(t, err)
	require.NotNil(t, draft, "draft must be returned alongside the validation error so callers can log it")
	assert.Equal(t, "x", draft.Review.Title)
	assert.Len(t, draft.Files, 1)
	assert.Contains(t, err.Error(), "files[0]")
}

func TestUploadDebugBundle(t *testing.T) {
	var (
		gotPath   string
		gotFields map[string]string
		gotFiles  map[string][]byte
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if !assert.NoError(t, r.ParseMultipartForm(32<<20)) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		gotFields = make(map[string]string, len(r.MultipartForm.Value))
		for k, v := range r.MultipartForm.Value {
			if len(v) > 0 {
				gotFields[k] = v[0]
			}
		}

		gotFiles = make(map[string][]byte)
		for _, headers := range r.MultipartForm.File {
			for _, fh := range headers {
				f, err := fh.Open()
				if !assert.NoError(t, err) {
					return
				}
				gr, err := gzip.NewReader(f)
				if !assert.NoError(t, err) {
					_ = f.Close()
					return
				}
				data, err := io.ReadAll(gr)
				assert.NoError(t, err)
				_ = f.Close()
				gotFiles[strings.TrimSuffix(fh.Filename, ".gz")] = data
			}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "abc123", "url": "/v1/debug/storage/abc123/"})
	}))
	defer srv.Close()

	c := NewUploadClient(slog.Default())
	url, err := c.UploadDebugBundle(context.Background(), srv.URL, "test-key", DebugMeta{
		MRIid:    "42",
		Runner:   "claude",
		Model:    "opus",
		ErrorMsg: "validate review.json: invalid reviewType at files[2]: \"\"",
	}, map[string][]byte{
		"review.json":        []byte(`{"files":[]}`),
		"claude-output.json": []byte(`{"type":"result"}`),
	})

	require.NoError(t, err)
	assert.Equal(t, "/v1/debug/storage/abc123/", url)
	assert.Equal(t, "/v1/upload/debug/test-key/", gotPath)
	assert.Equal(t, "42", gotFields["mrIid"])
	assert.Equal(t, "claude", gotFields["runner"])
	assert.Contains(t, gotFields["errorMsg"], "files[2]")
	assert.JSONEq(t, `{"files":[]}`, string(gotFiles["review.json"]))
	assert.JSONEq(t, `{"type":"result"}`, string(gotFiles["claude-output.json"]))
}

func TestUploadDebugBundle_NoOpWhenEmpty(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		called = true
	}))
	defer srv.Close()

	c := NewUploadClient(slog.Default())
	url, err := c.UploadDebugBundle(context.Background(), srv.URL, "k", DebugMeta{}, nil)
	require.NoError(t, err)
	assert.Empty(t, url)
	assert.False(t, called, "upload must be skipped when there is nothing to send")
}

func TestUploadDebugBundle_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("nope"))
	}))
	defer srv.Close()

	c := NewUploadClient(slog.Default())
	_, err := c.UploadDebugBundle(context.Background(), srv.URL, "k", DebugMeta{ErrorMsg: "boom"},
		map[string][]byte{"x.json": []byte("{}")})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 400")
}

func TestCollectDebugArtifacts(t *testing.T) {
	tmpDir := t.TempDir()
	for name, content := range map[string]string{
		"claude-output.json":     `{}`,
		"opencode-output.jsonl":  "{}\n",
		"review.json":            `{}`,
		"R1.feature.md":          "arch",
		"R2.feature.ru.md":       "code",
		"README.md":              "ignore me", // not R<digit>.*
		"unrelated.txt":          "ignore",
		"opencode-output.jsonlx": "ignore",
	} {
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, name), []byte(content), 0o644))
	}

	files := CollectDebugArtifacts(tmpDir)

	assert.Contains(t, files, "claude-output.json")
	assert.Contains(t, files, "opencode-output.jsonl")
	assert.Contains(t, files, "review.json")
	assert.Contains(t, files, "R1.feature.md")
	assert.Contains(t, files, "R2.feature.ru.md")
	assert.NotContains(t, files, "README.md")
	assert.NotContains(t, files, "unrelated.txt")
}

func TestCollectDebugArtifacts_MissingDir(t *testing.T) {
	files := CollectDebugArtifacts("/nonexistent/path/should-not-explode")
	assert.Empty(t, files)
}

func TestFindMDFiles(t *testing.T) {
	files, err := FindMDFiles("testdata")
	require.NoError(t, err)

	expected := map[string]bool{
		"architecture": true,
		"code":         true,
		"security":     true,
		"tests":        true,
	}

	for rt := range expected {
		assert.Contains(t, files, rt, "missing review type %q", rt)
	}

	assert.Len(t, files, len(expected))
}

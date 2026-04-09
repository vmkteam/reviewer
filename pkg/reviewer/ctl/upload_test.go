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

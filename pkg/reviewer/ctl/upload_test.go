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
)

func TestUploadReview(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		if !strings.HasPrefix(r.URL.Path, "/v1/upload/test-key/") {
			t.Errorf("path = %q, want prefix /v1/upload/test-key/", r.URL.Path)
		}

		body, _ := io.ReadAll(r.Body)
		var draft rest.ReviewDraft
		if err := json.Unmarshal(body, &draft); err != nil {
			t.Errorf("unmarshal body: %v", err)
		}
		if draft.Review.Title == "" {
			t.Error("review title is empty")
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("42"))
	}))
	defer srv.Close()

	draft, err := ReadReviewJSON("testdata")
	if err != nil {
		t.Fatal(err)
	}

	c := NewUploadClient(slog.Default())
	reviewID, err := c.UploadReview(context.Background(), srv.URL, "test-key", draft)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reviewID != 42 {
		t.Errorf("reviewID = %d, want 42", reviewID)
	}
}

func TestUploadFile(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/octet-stream" {
			t.Errorf("Content-Type = %q, want application/octet-stream", ct)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewUploadClient(slog.Default())
	err := c.UploadFile(context.Background(), srv.URL, "test-key", 42, "architecture", []byte("# Architecture"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/v1/upload/test-key/42/architecture/" {
		t.Errorf("path = %q, want /v1/upload/test-key/42/architecture/", gotPath)
	}
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
			if err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestReadReviewJSON(t *testing.T) {
	draft, err := ReadReviewJSON("testdata")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if draft.Review.Title != "Add user authentication" {
		t.Errorf("title = %q, want %q", draft.Review.Title, "Add user authentication")
	}
	if len(draft.Files) != 4 {
		t.Errorf("files count = %d, want 4", len(draft.Files))
	}
	if len(draft.Issues) != 2 {
		t.Errorf("issues count = %d, want 2", len(draft.Issues))
	}
}

func TestReadReviewJSON_Minimal(t *testing.T) {
	tmpDir := t.TempDir()
	data, err := os.ReadFile("testdata/review_minimal.json")
	if err != nil {
		t.Fatal(err)
	}
	if writeErr := os.WriteFile(filepath.Join(tmpDir, "review.json"), data, 0o644); writeErr != nil {
		t.Fatal(writeErr)
	}

	draft, err := ReadReviewJSON(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if draft.Review.Title != "Fix typo" {
		t.Errorf("title = %q, want %q", draft.Review.Title, "Fix typo")
	}
	if len(draft.Issues) != 0 {
		t.Errorf("issues count = %d, want 0", len(draft.Issues))
	}
}

func TestFindMDFiles(t *testing.T) {
	files, err := FindMDFiles("testdata")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := map[string]bool{
		"architecture": true,
		"code":         true,
		"security":     true,
		"tests":        true,
	}

	for rt := range expected {
		if _, ok := files[rt]; !ok {
			t.Errorf("missing review type %q", rt)
		}
	}

	if len(files) != len(expected) {
		t.Errorf("found %d files, want %d", len(files), len(expected))
	}
}

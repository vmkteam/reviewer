package ctl

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchPrompt(t *testing.T) {
	const wantPrompt = "Review this code for %SOURCE_BRANCH% targeting %TARGET_BRANCH%"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		if !strings.HasPrefix(r.URL.Path, "/v1/prompt/") {
			t.Errorf("path = %q, want prefix /v1/prompt/", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(wantPrompt))
	}))
	defer srv.Close()

	c := NewPromptClient(slog.Default())
	got, err := c.FetchPrompt(context.Background(), srv.URL, "test-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != wantPrompt {
		t.Errorf("prompt = %q, want %q", got, wantPrompt)
	}
}

func TestFetchPrompt_ServerError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{"not found", http.StatusNotFound},
		{"server error", http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte("error"))
			}))
			defer srv.Close()

			c := NewPromptClient(slog.Default())
			_, err := c.FetchPrompt(context.Background(), srv.URL, "test-key")
			if err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestSubstituteVariables(t *testing.T) {
	cfg := &Config{
		SourceBranch: "feature/foo",
		TargetBranch: "master",
		MRTitle:      "Add new feature",
		ExternalID:   "123",
	}

	t.Run("all replacements", func(t *testing.T) {
		input := "Review %SOURCE_BRANCH% → %TARGET_BRANCH%, MR: %MR_TITLE%, ID: %EXTERNAL_ID%"
		want := "Review feature/foo → master, MR: Add new feature, ID: 123"
		got := SubstituteVariables(input, cfg)
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("no placeholders", func(t *testing.T) {
		input := "Plain text without placeholders"
		got := SubstituteVariables(input, cfg)
		if got != input {
			t.Errorf("got %q, want %q", got, input)
		}
	})

	t.Run("unknown placeholders unchanged", func(t *testing.T) {
		input := "%UNKNOWN% stays as is"
		got := SubstituteVariables(input, cfg)
		if got != input {
			t.Errorf("got %q, want %q", got, input)
		}
	})
}

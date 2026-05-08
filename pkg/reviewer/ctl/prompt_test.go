package ctl

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchPrompt(t *testing.T) {
	const wantPrompt = "Review this code for %SOURCE_BRANCH% targeting %TARGET_BRANCH%"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.True(t, strings.HasPrefix(r.URL.Path, "/v1/prompt/"), "path = %q, want prefix /v1/prompt/", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(wantPrompt))
	}))
	defer srv.Close()

	c := NewPromptClient(slog.Default())
	got, err := c.FetchPrompt(context.Background(), srv.URL, "test-key")
	require.NoError(t, err)
	assert.Equal(t, wantPrompt, got)
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
			require.Error(t, err)
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
		assert.Equal(t, want, got)
	})

	t.Run("no placeholders", func(t *testing.T) {
		input := "Plain text without placeholders"
		got := SubstituteVariables(input, cfg)
		assert.Equal(t, input, got)
	})

	t.Run("unknown placeholders unchanged", func(t *testing.T) {
		input := "%UNKNOWN% stays as is"
		got := SubstituteVariables(input, cfg)
		assert.Equal(t, input, got)
	})

	t.Run("empty cfg fields leave placeholders intact for model resolution", func(t *testing.T) {
		empty := &Config{}
		input := "Review %SOURCE_BRANCH% → %TARGET_BRANCH%, MR: %TITLE%, ID: %EXTERNAL_ID%"
		got := SubstituteVariables(input, empty)
		assert.Equal(t, input, got, "empty cfg must not blank out placeholders")
	})
}

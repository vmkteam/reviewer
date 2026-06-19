package ctl

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchPrompt(t *testing.T) {
	const wantPrompt = "Review this code for %SOURCE_BRANCH% targeting %TARGET_BRANCH%"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1/reviewctl/rpc/", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "result": wantPrompt, "id": 1})
	}))
	defer srv.Close()

	c := NewPromptClient(slog.Default())
	got, err := c.FetchPrompt(context.Background(), srv.URL, "test-key")
	require.NoError(t, err)
	assert.Equal(t, wantPrompt, got)
}

func TestFetchConfig(t *testing.T) {
	reply := func(result map[string]any) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "/v1/reviewctl/rpc/", r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "result": result, "id": 1})
		}))
	}

	t.Run("panel with judge maps primary, panel and judge", func(t *testing.T) {
		srv := reply(map[string]any{
			"primary": map[string]any{"runnerProfileId": 7, "title": "Primary", "runner": "claude", "model": "opus", "effort": "xhigh", "token": "tok-p"},
			"panel": []any{
				map[string]any{"runnerProfileId": 8, "title": "Member", "runner": "codex", "model": "gpt-5.5", "token": "tok-m"},
			},
			"judge": map[string]any{"runnerProfileId": 9, "title": "Judge", "runner": "claude", "model": "opus"},
		})
		defer srv.Close()

		rc, err := NewPromptClient(slog.Default()).FetchConfig(context.Background(), srv.URL, "test-key")
		require.NoError(t, err)
		require.NotNil(t, rc.Primary)
		assert.Equal(t, 7, rc.Primary.RunnerProfileID)
		assert.Equal(t, "claude", rc.Primary.Runner)
		assert.Equal(t, "tok-p", rc.Primary.Token)
		require.Len(t, rc.Panel, 1)
		assert.Equal(t, "codex", rc.Panel[0].Runner)
		assert.Equal(t, "gpt-5.5", rc.Panel[0].Model)
		require.NotNil(t, rc.Judge)
		assert.Equal(t, 9, rc.Judge.RunnerProfileID)
	})

	t.Run("single review: no panel, no judge", func(t *testing.T) {
		srv := reply(map[string]any{
			"primary": map[string]any{"runnerProfileId": 7, "title": "Primary", "runner": "claude", "model": "opus"},
		})
		defer srv.Close()

		rc, err := NewPromptClient(slog.Default()).FetchConfig(context.Background(), srv.URL, "test-key")
		require.NoError(t, err)
		require.NotNil(t, rc.Primary)
		assert.Empty(t, rc.Panel)
		assert.Nil(t, rc.Judge)
	})
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

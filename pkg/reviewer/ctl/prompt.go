package ctl

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// PromptClient fetches review prompts from the reviewsrv server.
type PromptClient struct {
	httpClient *http.Client
	log        *slog.Logger
}

// NewPromptClient creates a new PromptClient.
func NewPromptClient(log *slog.Logger) *PromptClient {
	return &PromptClient{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		log:        log,
	}
}

// FetchPrompt fetches the assembled prompt for the given project key.
func (c *PromptClient) FetchPrompt(ctx context.Context, serverURL, projectKey string) (string, error) {
	url := fmt.Sprintf("%s/v1/prompt/%s/", strings.TrimRight(serverURL, "/"), projectKey)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("create prompt request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch prompt: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read prompt response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch prompt: HTTP %d: %s", resp.StatusCode, string(body))
	}

	c.log.InfoContext(ctx, "fetched prompt", "projectKey", projectKey, "length", len(body))

	return string(body), nil
}

// SubstituteVariables replaces CI placeholders in the prompt text.
func SubstituteVariables(prompt string, cfg *Config) string {
	r := strings.NewReplacer(
		"%SOURCE_BRANCH%", cfg.SourceBranch,
		"%TARGET_BRANCH%", cfg.TargetBranch,
		"%MR_TITLE%", cfg.MRTitle,
		"%EXTERNAL_ID%", cfg.ExternalID,
	)
	return r.Replace(prompt)
}

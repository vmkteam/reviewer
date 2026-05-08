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

// CI metadata placeholders left in the prompt body and the review.json
// skeleton when the corresponding cfg field is empty (local-run scenarios).
// The prompt instructs the model to resolve these from git context.
const (
	PlaceholderSourceBranch = "%SOURCE_BRANCH%"
	PlaceholderTargetBranch = "%TARGET_BRANCH%"
	PlaceholderTitle        = "%TITLE%"
	PlaceholderExternalID   = "%EXTERNAL_ID%"
	PlaceholderCommitHash   = "%COMMIT_HASH%"
	PlaceholderAuthor       = "%AUTHOR%"

	// PlaceholderMRTitle is an alias kept for prompts authored before %TITLE%
	// became canonical. New code should use PlaceholderTitle.
	//
	// Deprecated: use PlaceholderTitle.
	PlaceholderMRTitle = "%MR_TITLE%"
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

// SubstituteVariables replaces CI placeholders in the prompt text. Empty
// values are skipped so the placeholder survives — the model is told to
// resolve unresolved placeholders from git context (see promptReviewJSON).
func SubstituteVariables(prompt string, cfg *Config) string {
	all := []string{
		PlaceholderSourceBranch, cfg.SourceBranch,
		PlaceholderTargetBranch, cfg.TargetBranch,
		PlaceholderMRTitle, cfg.MRTitle,
		PlaceholderTitle, cfg.MRTitle,
		PlaceholderExternalID, cfg.ExternalID,
	}
	pairs := make([]string, 0, len(all))
	for i := 0; i < len(all); i += 2 {
		if all[i+1] != "" {
			pairs = append(pairs, all[i], all[i+1])
		}
	}
	if len(pairs) == 0 {
		return prompt
	}
	return strings.NewReplacer(pairs...).Replace(prompt)
}

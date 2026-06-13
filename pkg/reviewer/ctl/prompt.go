package ctl

import (
	"bytes"
	"context"
	"encoding/json"
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

// RunnerProfileParams mirrors the server's params payload.
type RunnerProfileParams struct {
	AllowDangerousPermissions bool `json:"allowDangerousPermissions"`
}

// ReviewConfig is the resolved runner configuration returned by the reviewctl RPC.
type ReviewConfig struct {
	RunnerProfileID int                 `json:"runnerProfileId"`
	Title           string              `json:"title"`
	Runner          string              `json:"runner"`
	Model           string              `json:"model"`
	Effort          string              `json:"effort"`
	APIProvider     string              `json:"apiProvider"`
	APIBaseURL      string              `json:"apiBaseURL"`
	Token           string              `json:"token"`
	Params          RunnerProfileParams `json:"params"`
}

// FetchConfig fetches the resolved runner profile for the project key over the
// internal reviewctl RPC.
func (c *PromptClient) FetchConfig(ctx context.Context, serverURL, projectKey string) (*ReviewConfig, error) {
	var rc ReviewConfig
	if err := c.rpcCall(ctx, serverURL, "ReviewConfig", projectKey, &rc); err != nil {
		return nil, err
	}
	c.log.InfoContext(ctx, "fetched review config", "projectKey", projectKey, "profileId", rc.RunnerProfileID, "runner", rc.Runner, "model", rc.Model)
	return &rc, nil
}

// FetchPrompt fetches the assembled prompt for the given project key over the
// internal reviewctl RPC.
func (c *PromptClient) FetchPrompt(ctx context.Context, serverURL, projectKey string) (string, error) {
	var prompt string
	if err := c.rpcCall(ctx, serverURL, "Prompt", projectKey, &prompt); err != nil {
		return "", err
	}
	c.log.InfoContext(ctx, "fetched prompt", "projectKey", projectKey, "length", len(prompt))
	return prompt, nil
}

// rpcCall performs a JSON-RPC 2.0 call to /v1/reviewctl/rpc/ and decodes the
// result into out.
func (c *PromptClient) rpcCall(ctx context.Context, serverURL, method, projectKey string, out any) error {
	url := strings.TrimRight(serverURL, "/") + "/v1/reviewctl/rpc/"
	reqBody, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  map[string]string{"projectKey": projectKey},
		"id":      1,
	})
	if err != nil {
		return fmt.Errorf("marshal %s request: %w", method, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("create %s request: %w", method, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%s: %w", method, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read %s response: %w", method, err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: HTTP %d: %s", method, resp.StatusCode, string(body))
	}

	var env struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return fmt.Errorf("parse %s response: %w", method, err)
	}
	if env.Error != nil {
		return fmt.Errorf("reviewctl %s: %s", method, env.Error.Message)
	}
	if out != nil && len(env.Result) > 0 {
		if err := json.Unmarshal(env.Result, out); err != nil {
			return fmt.Errorf("decode %s result: %w", method, err)
		}
	}
	return nil
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

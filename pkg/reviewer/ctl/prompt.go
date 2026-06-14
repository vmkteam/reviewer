package ctl

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"reviewsrv/pkg/reviewer/ctl/reviewctlclient"
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

// reviewctlRPCPath is the internal JSON-RPC endpoint reviewctl talks to. The
// generated client posts to the exact endpoint it is given, so the full path is
// appended to the server URL here.
const reviewctlRPCPath = "/v1/reviewctl/rpc/"

// PromptClient fetches the runner profile and review prompt from the reviewsrv
// server over the internal reviewctl JSON-RPC, using the rpcgen-generated client.
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

// client builds a generated reviewctl client pointed at serverURL. The endpoint
// (server URL + the RPC path) is only known per call, so it is built on demand
// over the shared httpClient (which keeps the 10s timeout).
func (c *PromptClient) client(serverURL string) *reviewctlclient.Client {
	return reviewctlclient.NewClient(strings.TrimRight(serverURL, "/")+reviewctlRPCPath, c.httpClient)
}

// FetchConfig fetches the resolved runner profile for the project key over the
// internal reviewctl RPC.
func (c *PromptClient) FetchConfig(ctx context.Context, serverURL, projectKey string) (*ReviewConfig, error) {
	cfg, err := c.client(serverURL).Reviewctl.ReviewConfig(ctx, projectKey)
	if err != nil {
		return nil, err
	}

	rc := &ReviewConfig{
		RunnerProfileID: cfg.RunnerProfileID,
		Title:           cfg.Title,
		Runner:          cfg.Runner,
		Model:           cfg.Model,
		Effort:          cfg.Effort,
		APIProvider:     cfg.ApiProvider,
		APIBaseURL:      cfg.ApiBaseURL,
		Token:           cfg.Token,
		Params:          RunnerProfileParams{AllowDangerousPermissions: cfg.Params.AllowDangerousPermissions},
	}
	c.log.InfoContext(ctx, "fetched review config", "projectKey", projectKey, "profileId", rc.RunnerProfileID, "runner", rc.Runner, "model", rc.Model)
	return rc, nil
}

// FetchPrompt fetches the assembled prompt for the given project key over the
// internal reviewctl RPC.
func (c *PromptClient) FetchPrompt(ctx context.Context, serverURL, projectKey string) (string, error) {
	prompt, err := c.client(serverURL).Reviewctl.Prompt(ctx, projectKey)
	if err != nil {
		return "", err
	}
	c.log.InfoContext(ctx, "fetched prompt", "projectKey", projectKey, "length", len(prompt))
	return prompt, nil
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

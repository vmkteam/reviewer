package ctl

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"reviewsrv/pkg/db"
	"reviewsrv/pkg/reviewer/runner"
)

// Config holds all CLI flags and CI environment variables for reviewctl.
type Config struct {
	Key       string
	URL       string
	PublicURL string // browser-facing base URL for links in MR comments; falls back to URL
	Runner    string // "claude" (default) or "opencode"
	Model     string
	Dir       string
	Verbose   bool

	// GitLab MR comment settings.
	GitLabURL   string
	GitLabToken string
	MRIID       string
	ProjectID   string
	DiffBaseSHA string

	// MR metadata (populated from CI environment).
	SourceBranch string
	TargetBranch string
	Commit       string
	Author       string
	MRTitle      string
	ExternalID   string

	// Claude session for --resume (reuses prompt cache).
	SessionID       string
	ContinueSession bool // use --continue instead of --resume

	// Direct-API runner (--runner direct): provider, endpoint and reasoning effort.
	// The API key is read from the environment (ANTHROPIC_API_KEY / DEEPSEEK_API_KEY),
	// never a flag.
	APIProvider string // "deepseek" (default) | "openai-compat" | "anthropic"
	APIBaseURL  string
	Effort      string

	// Resolved runner profile (fetched from the server over /v1/reviewctl/rpc/).
	// Token is the optional API-key fallback used only when the matching env var
	// is absent; RunnerProfileID/Title are recorded in the review snapshot.
	Token              string
	RunnerProfileID    int
	RunnerProfileTitle string

	// AllowDangerousPermissions toggles `--dangerously-skip-permissions` for
	// runners that support it (currently opencode). Defaults to true to match
	// previous behaviour — unattended CI runs need it to avoid permission
	// prompts. Set false for local interactive review on untrusted code.
	AllowDangerousPermissions bool

	// Multi holds the panel members for a local multi-review run (--multi /
	// $REVIEW_MULTI). Empty = single review. Each member is a runner+model run in
	// its own git worktree with ambient credentials; the server-driven panel
	// (with per-member tokens) arrives in a later phase.
	Multi []MemberSpec

	// Judge is the synthesizer for a multi-review panel (--judge / $REVIEW_JUDGE).
	// When set with >=2 members, reviewctl runs the panel then the judge over the
	// members' outputs and uploads one fused review. Nil = no fusion (members only).
	Judge *MemberSpec

	// Timeout bounds each member and the judge run (--timeout / $REVIEW_TIMEOUT).
	// 0 = no timeout. Members run concurrently, so this caps the slowest one.
	Timeout time.Duration

	// DebugUpload uploads collected artifacts to /v1/upload/debug/ on every run.
	// On failure, the upload happens regardless of this flag.
	DebugUpload bool

	// For comment subcommand.
	ReviewID int
}

// RunnerProfileSnapshot builds the resolved-profile snapshot recorded on the
// review. The token is deliberately excluded — secrets never land in a review.
func (c *Config) RunnerProfileSnapshot() db.ReviewRunnerProfile {
	return db.ReviewRunnerProfile{
		RunnerProfileID: c.RunnerProfileID,
		Title:           c.RunnerProfileTitle,
		Runner:          c.Runner,
		Model:           c.Model,
		Effort:          c.Effort,
		APIProvider:     c.APIProvider,
		APIBaseURL:      c.APIBaseURL,
		Params:          db.RunnerProfileParams{AllowDangerousPermissions: c.AllowDangerousPermissions},
	}
}

// Validate checks that required fields are set for the given subcommand.
func (c *Config) Validate(cmd string) error {
	if c.Key == "" {
		return errors.New("--key / $PROJECT_KEY is required")
	}
	if c.URL == "" {
		return errors.New("--url / $REVIEWSRV_URL is required")
	}

	if cmd == "comment" && c.ReviewID == 0 { //nolint:goconst // CLI subcommand name
		return errors.New("--review-id is required for comment subcommand")
	}

	return nil
}

// HasGitLab returns true if GitLab MR comment settings are configured.
func (c *Config) HasGitLab() bool {
	return c.GitLabToken != "" && c.GitLabURL != "" && c.MRIID != "" && c.ProjectID != ""
}

// PublicBaseURL returns the browser-facing base URL for links shown to users,
// falling back to URL when PublicURL is not set.
func (c *Config) PublicBaseURL() string {
	if c.PublicURL != "" {
		return c.PublicURL
	}
	return c.URL
}

// ResolveDefaults fills runner-specific defaults (model, reasoning effort) when
// the corresponding flag is empty, so log lines, ModelInfo and the debug bundle
// all show what was actually sent instead of "" (the user-facing input). Mutates c.
//
// opencode stays unpinned: its default lives in the user's opencode config.
// Claude CLI's own default drifts between sonnet/opus across releases —
// we pin opus to keep review cost and quality predictable.
func (c *Config) ResolveDefaults() {
	if c.Model == "" && (c.Runner == "" || c.Runner == runner.RunnerClaude) {
		c.Model = "opus" //nolint:goconst // Claude model alias
	}
	// Direct runner against Anthropic: pin a concrete model and reasoning effort
	// so cost/quality stay predictable. Without an explicit effort the Anthropic
	// API silently defaults to "high", whereas Claude Code uses "xhigh" for
	// agentic coding — match it so the direct runner isn't a notch weaker out of
	// the box. DeepSeek/openai-compat ignore effort and require an explicit --model.
	if c.Runner == runner.RunnerDirect && c.APIProvider == "anthropic" { //nolint:goconst // provider id; canonical const lives in pkg/reviewer/direct
		if c.Model == "" {
			c.Model = "claude-opus-4-8" //nolint:goconst // pinned model id
		}
		if c.Effort == "" {
			c.Effort = "xhigh" //nolint:goconst // reasoning effort level
		}
	}
	// Codex reports no dollar cost, so pin a concrete model: the CLI then uses a
	// predictable model and the cost is estimated from tokens against the price
	// table (an empty model leaves cost at 0). Override with --model.
	if c.Runner == runner.RunnerCodex && c.Model == "" {
		c.Model = "gpt-5.1-codex"
	}
}

// MemberSpec is one panel member for a local --multi run: a runner and its model.
// Per-member tokens/providers come from the server in the full flow; --multi is a
// debug override that relies on ambient credentials.
type MemberSpec struct {
	Runner string
	Model  string
}

// ParseMulti parses the --multi value: a comma-separated list of runner:model
// members, e.g. "codex:gpt-5.5,opencode:openrouter/deepseek/deepseek-v4-pro". Only
// the first colon separates runner from model (models may contain slashes); a bare
// "runner" with no colon uses the runner's default model. Returns nil for an empty
// string (single review, no panel).
func ParseMulti(s string) ([]MemberSpec, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}

	var out []MemberSpec
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		r, m, _ := strings.Cut(part, ":")
		r = strings.TrimSpace(r)
		m = strings.TrimSpace(m)
		switch r {
		case runner.RunnerClaude, runner.RunnerOpenCode, runner.RunnerCodex, runner.RunnerDirect:
		default:
			return nil, fmt.Errorf("--multi: unknown runner %q in %q (want claude|opencode|codex|direct)", r, part)
		}
		out = append(out, MemberSpec{Runner: r, Model: m})
	}
	return out, nil
}

// ParseJudge parses the --judge value: a single runner:model spec (same grammar
// as one --multi member). Returns nil for an empty string (no judge / no fusion).
func ParseJudge(s string) (*MemberSpec, error) {
	specs, err := ParseMulti(s)
	if err != nil {
		return nil, err
	}
	switch len(specs) {
	case 0:
		return nil, nil
	case 1:
		return &specs[0], nil
	default:
		return nil, fmt.Errorf("--judge expects a single runner:model, got %d", len(specs))
	}
}

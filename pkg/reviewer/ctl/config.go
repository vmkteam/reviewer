package ctl

import "errors"

// Runner identifiers used by Config.Runner, ReviewRunner.Name and db.ReviewModelInfo.Runner.
const (
	RunnerClaude   = "claude"
	RunnerOpenCode = "opencode"
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

	// DebugUpload uploads collected artifacts to /v1/upload/debug/ on every run.
	// On failure, the upload happens regardless of this flag.
	DebugUpload bool

	// For comment subcommand.
	ReviewID int
}

// Validate checks that required fields are set for the given subcommand.
func (c *Config) Validate(cmd string) error {
	if c.Key == "" {
		return errors.New("--key / $PROJECT_KEY is required")
	}
	if c.URL == "" {
		return errors.New("--url / $REVIEWSRV_URL is required")
	}

	if cmd == "comment" && c.ReviewID == 0 {
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

// ResolveModel fills c.Model with the runner-specific default when empty,
// so log lines, ModelInfo and the debug bundle all show what was actually
// passed to the CLI instead of "" (the user-facing input). Mutates c.
//
// opencode stays unpinned: its default lives in the user's opencode config.
// Claude CLI's own default drifts between sonnet/opus across releases —
// we pin opus to keep review cost and quality predictable.
func (c *Config) ResolveModel() {
	if c.Model == "" && (c.Runner == "" || c.Runner == RunnerClaude) {
		c.Model = "opus"
	}
}

package ctl

import "errors"

// Runner identifiers used by Config.Runner, ClaudeRunner.Name and db.ReviewModelInfo.Runner.
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

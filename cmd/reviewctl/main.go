package main

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"reviewsrv/pkg/reviewer/ctl"
	"reviewsrv/pkg/reviewer/direct"
	"reviewsrv/pkg/reviewer/runner"

	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	cfg := &ctl.Config{}

	rootCmd := &cobra.Command{
		Use:          "reviewctl",
		Short:        "AI code review orchestrator",
		SilenceUsage: true,
		PersistentPreRun: func(cmd *cobra.Command, _ []string) {
			// Skip the banner for `version` so `reviewctl version` stays scriptable.
			if cmd.Name() == "version" {
				return
			}
			slog.Default().InfoContext(cmd.Context(), "reviewctl", "version", version)
		},
	}

	pf := rootCmd.PersistentFlags()
	pf.StringVar(&cfg.Key, "key", os.Getenv("PROJECT_KEY"), "project key (UUID)")
	pf.StringVar(&cfg.URL, "url", os.Getenv("REVIEWSRV_URL"), "reviewsrv server URL (used for API calls from CI)")
	pf.StringVar(&cfg.PublicURL, "public-url", os.Getenv("REVIEWSRV_PUBLIC_URL"), "browser-facing base URL for links in MR comments (defaults to --url)")
	pf.StringVar(&cfg.Runner, "runner", ctl.EnvDefault("REVIEW_RUNNER", runner.RunnerClaude), "runner: claude | opencode | codex | direct (direct = no CLI, calls the API directly)")
	pf.StringVar(&cfg.Model, "model", os.Getenv("REVIEW_MODEL"), "model name (optional; if empty, runner CLI picks its own default)")
	pf.StringVar(&cfg.Dir, "dir", ctl.EnvDefault("REVIEW_DIR", "."), "working directory with review files")
	pf.BoolVar(&cfg.Verbose, "verbose", ctl.EnvBool("REVIEW_VERBOSE", false), "verbose output")
	pf.StringVar(&cfg.GitLabURL, "gitlab-url", os.Getenv("CI_API_V4_URL"), "GitLab API URL")
	pf.StringVar(&cfg.GitLabToken, "gitlab-token", os.Getenv("REVIEWER_GITLAB_TOKEN"), "GitLab API token")
	pf.StringVar(&cfg.MRIID, "mr-iid", os.Getenv("CI_MERGE_REQUEST_IID"), "MR IID")
	pf.StringVar(&cfg.ProjectID, "project-id", os.Getenv("CI_PROJECT_ID"), "GitLab project ID")
	pf.StringVar(&cfg.SourceBranch, "source-branch", os.Getenv("CI_MERGE_REQUEST_SOURCE_BRANCH_NAME"), "source branch")
	pf.StringVar(&cfg.TargetBranch, "target-branch", os.Getenv("CI_MERGE_REQUEST_TARGET_BRANCH_NAME"), "target branch")
	pf.StringVar(&cfg.Commit, "commit", os.Getenv("CI_COMMIT_SHA"), "commit SHA")
	// CI_COMMIT_AUTHOR ("Name <email>") tracks the actual change author and is
	// stable across pipeline retries, unlike GITLAB_USER_LOGIN which reflects
	// whoever triggered the run. The email is stripped to avoid leaking it
	// into Slack notifications and the public API.
	pf.StringVar(&cfg.Author, "author", ctl.AuthorName(ctl.EnvDefault("CI_COMMIT_AUTHOR", os.Getenv("GITLAB_USER_LOGIN"))), "MR author")
	pf.StringVar(&cfg.MRTitle, "mr-title", os.Getenv("CI_MERGE_REQUEST_TITLE"), "MR title")
	pf.StringVar(&cfg.ExternalID, "external-id", os.Getenv("CI_MERGE_REQUEST_IID"), "external ID")
	pf.StringVar(&cfg.DiffBaseSHA, "diff-base-sha", os.Getenv("CI_MERGE_REQUEST_DIFF_BASE_SHA"), "diff base SHA")
	pf.StringVar(&cfg.SessionID, "session", "", "Claude session ID for --resume (reuses prompt cache)")
	pf.BoolVar(&cfg.ContinueSession, "continue", false, "continue last Claude session (auto-detect)")
	pf.BoolVar(&cfg.DebugUpload, "debug-upload", ctl.EnvBool("REVIEW_DEBUG_UPLOAD", false), "always upload artifacts to /v1/upload/debug/ (failures upload regardless)")
	pf.BoolVar(&cfg.AllowDangerousPermissions, "allow-dangerous-permissions", ctl.EnvBool("REVIEW_ALLOW_DANGEROUS_PERMISSIONS", true), "pass --dangerously-skip-permissions to opencode (default true; required for unattended CI)")
	pf.StringVar(&cfg.APIProvider, "api-provider", ctl.EnvDefault("REVIEW_API_PROVIDER", "deepseek"), "direct runner provider: deepseek | openai-compat | anthropic (key from ANTHROPIC_API_KEY/DEEPSEEK_API_KEY env)")
	pf.StringVar(&cfg.APIBaseURL, "api-base-url", os.Getenv("REVIEW_API_BASE_URL"), "direct runner API base URL (defaults to provider's standard endpoint)")
	pf.StringVar(&cfg.Effort, "effort", os.Getenv("REVIEW_EFFORT"), "direct runner reasoning effort for Anthropic: low|medium|high|xhigh|max")

	reviewCmd := &cobra.Command{
		Use:   "review",
		Short: "Full review cycle: prompt → Claude → upload → comment → HTML",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := cfg.Validate("review"); err != nil {
				return err
			}
			log := slog.Default()
			if err := applyReviewConfig(cmd, cfg, log); err != nil {
				return err
			}
			rr, err := buildRunner(cfg, log)
			if err != nil {
				return err
			}
			c := ctl.NewController(cfg, rr, log)
			return c.Review(cmd.Context())
		},
	}

	uploadCmd := &cobra.Command{
		Use:   "upload",
		Short: "Upload local review.json + R*.md to server",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := cfg.Validate("upload"); err != nil {
				return err
			}
			c := ctl.NewController(cfg, nil, slog.Default())
			return c.Upload(cmd.Context())
		},
	}

	commentCmd := &cobra.Command{
		Use:   "comment",
		Short: "Post MR comments for an existing review",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := cfg.Validate("comment"); err != nil {
				return err
			}
			c := ctl.NewController(cfg, nil, slog.Default())
			return c.Comment(cmd.Context())
		},
	}
	commentCmd.Flags().IntVar(&cfg.ReviewID, "review-id", 0, "existing review ID")

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print version",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := cmd.OutOrStdout().Write([]byte("reviewctl " + version + "\n"))
			return err
		},
	}

	rootCmd.AddCommand(reviewCmd, uploadCmd, commentCmd, versionCmd)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// applyReviewConfig fetches the project's runner profile from the server and
// applies it to cfg. Explicit flags win over the profile; the profile fills the
// rest, so CI needs only the image, project key, server URL and credentials.
func applyReviewConfig(cmd *cobra.Command, cfg *ctl.Config, log *slog.Logger) error {
	rc, err := ctl.NewPromptClient(log).FetchConfig(cmd.Context(), cfg.URL, cfg.Key)
	if err != nil {
		return fmt.Errorf("fetch review config: %w", err)
	}

	fl := cmd.Flags()
	cfg.RunnerProfileID = rc.RunnerProfileID
	cfg.RunnerProfileTitle = rc.Title
	cfg.Token = rc.Token
	if !fl.Changed("runner") && rc.Runner != "" {
		cfg.Runner = rc.Runner
	}
	if !fl.Changed("model") && rc.Model != "" {
		cfg.Model = rc.Model
	}
	if !fl.Changed("effort") && rc.Effort != "" {
		cfg.Effort = rc.Effort
	}
	if !fl.Changed("api-provider") && rc.APIProvider != "" {
		cfg.APIProvider = rc.APIProvider
	}
	if !fl.Changed("api-base-url") && rc.APIBaseURL != "" {
		cfg.APIBaseURL = rc.APIBaseURL
	}
	if !fl.Changed("allow-dangerous-permissions") {
		cfg.AllowDangerousPermissions = rc.Params.AllowDangerousPermissions
	}

	log.InfoContext(cmd.Context(), "applied runner profile",
		"profileId", rc.RunnerProfileID, "title", rc.Title,
		"runner", cfg.Runner, "model", cfg.Model, "effort", cfg.Effort, "provider", cfg.APIProvider)
	return nil
}

// applyTokenFallback exports the runner profile's token to the credential env
// var a CLI runner reads, but only when that env var is empty — env always wins.
// The direct runner consumes the token directly (see buildDirectRunner).
func applyTokenFallback(cfg *ctl.Config) {
	if cfg.Token == "" {
		return
	}
	switch cfg.Runner {
	case "", runner.RunnerClaude:
		if os.Getenv(envAnthropicAPIKey) == "" {
			_ = os.Setenv(envAnthropicAPIKey, cfg.Token)
		}
	case runner.RunnerCodex:
		if os.Getenv(envOpenAIAPIKey) == "" {
			_ = os.Setenv(envOpenAIAPIKey, cfg.Token)
		}
	}
}

func buildRunner(cfg *ctl.Config, log *slog.Logger) (runner.ReviewRunner, error) {
	cfg.ResolveDefaults()
	applyTokenFallback(cfg)
	switch cfg.Runner {
	case "", runner.RunnerClaude:
		return &runner.ExecClaudeRunner{Model: cfg.Model, Effort: cfg.Effort, Dir: cfg.Dir, SessionID: cfg.SessionID, ContinueSession: cfg.ContinueSession, Log: log}, nil
	case runner.RunnerOpenCode:
		return &runner.ExecOpenCodeRunner{
			Model:                     cfg.Model,
			Dir:                       cfg.Dir,
			SessionID:                 cfg.SessionID,
			ContinueSession:           cfg.ContinueSession,
			AllowDangerousPermissions: cfg.AllowDangerousPermissions,
			Log:                       log,
		}, nil
	case runner.RunnerCodex:
		return &runner.ExecCodexRunner{Model: cfg.Model, Dir: cfg.Dir, SessionID: cfg.SessionID, ContinueSession: cfg.ContinueSession, Log: log}, nil
	case runner.RunnerDirect:
		return buildDirectRunner(cfg, log)
	default:
		return nil, fmt.Errorf("unknown --runner %q (supported: %s, %s, %s, %s)", cfg.Runner, runner.RunnerClaude, runner.RunnerOpenCode, runner.RunnerCodex, runner.RunnerDirect)
	}
}

func buildDirectRunner(cfg *ctl.Config, log *slog.Logger) (runner.ReviewRunner, error) {
	apiKey := directAPIKey(cfg.APIProvider)
	if apiKey == "" {
		apiKey = cfg.Token // runner profile fallback (env still took priority above)
	}
	if apiKey == "" {
		return nil, fmt.Errorf("--runner direct: API key not found in environment (set %s) or runner profile token", strings.Join(directKeyEnvs(cfg.APIProvider), " or "))
	}
	prov, err := direct.NewProvider(direct.ProviderConfig{
		Provider: cfg.APIProvider,
		Model:    cfg.Model,
		BaseURL:  cfg.APIBaseURL,
		APIKey:   apiKey,
	})
	if err != nil {
		return nil, err
	}
	return &runner.DirectRunner{
		Provider: prov,
		Dir:      cfg.Dir,
		DiffBase: cfg.TargetBranch,
		DiffHead: cfg.SourceBranch,
		Effort:   cfg.Effort,
		Log:      log,
	}, nil
}

// Env var names that may carry the direct-runner API key.
const (
	envReviewAPIKey    = "REVIEW_API_KEY"
	envAnthropicAPIKey = "ANTHROPIC_API_KEY"
	envOpenAIAPIKey    = "OPENAI_API_KEY"
	envDeepSeekAPIKey  = "DEEPSEEK_API_KEY"
)

// directKeyEnvs reports the env vars that may hold the API key for the given
// provider, in priority order. REVIEW_API_KEY is a provider-agnostic override so
// an arbitrary OpenAI-compatible endpoint need not borrow the DEEPSEEK_API_KEY
// name; the provider-specific name is the fallback. Provider matching is
// case-insensitive, matching direct.NewProvider.
func directKeyEnvs(provider string) []string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "anthropic":
		return []string{envReviewAPIKey, envAnthropicAPIKey}
	case "openai", "openai-compat":
		return []string{envReviewAPIKey, envOpenAIAPIKey}
	default: // deepseek (the default) and any other openai-compatible backend
		return []string{envReviewAPIKey, envDeepSeekAPIKey}
	}
}

func directAPIKey(provider string) string {
	for _, name := range directKeyEnvs(provider) {
		if v := os.Getenv(name); v != "" {
			return v
		}
	}
	return ""
}

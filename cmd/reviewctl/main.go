package main

import (
	"fmt"
	"log/slog"
	"os"

	"reviewsrv/pkg/reviewer/ctl"

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
	pf.StringVar(&cfg.Runner, "runner", ctl.EnvDefault("REVIEW_RUNNER", ctl.RunnerClaude), "runner CLI: claude | opencode")
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

	reviewCmd := &cobra.Command{
		Use:   "review",
		Short: "Full review cycle: prompt → Claude → upload → comment → HTML",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := cfg.Validate("review"); err != nil {
				return err
			}
			log := slog.Default()
			runner, err := buildRunner(cfg, log)
			if err != nil {
				return err
			}
			c := ctl.NewController(cfg, runner, log)
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

func buildRunner(cfg *ctl.Config, log *slog.Logger) (ctl.ReviewRunner, error) {
	cfg.ResolveModel()
	switch cfg.Runner {
	case "", ctl.RunnerClaude:
		return &ctl.ExecClaudeRunner{Model: cfg.Model, Dir: cfg.Dir, SessionID: cfg.SessionID, ContinueSession: cfg.ContinueSession, Log: log}, nil
	case ctl.RunnerOpenCode:
		return &ctl.ExecOpenCodeRunner{
			Model:                     cfg.Model,
			Dir:                       cfg.Dir,
			SessionID:                 cfg.SessionID,
			ContinueSession:           cfg.ContinueSession,
			AllowDangerousPermissions: cfg.AllowDangerousPermissions,
			Log:                       log,
		}, nil
	default:
		return nil, fmt.Errorf("unknown --runner %q (supported: %s, %s)", cfg.Runner, ctl.RunnerClaude, ctl.RunnerOpenCode)
	}
}

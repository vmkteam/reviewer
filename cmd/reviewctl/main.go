package main

import (
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
	}

	pf := rootCmd.PersistentFlags()
	pf.StringVar(&cfg.Key, "key", os.Getenv("PROJECT_KEY"), "project key (UUID)")
	pf.StringVar(&cfg.URL, "url", os.Getenv("REVIEWSRV_URL"), "reviewsrv server URL")
	pf.StringVar(&cfg.Model, "model", envDefault("REVIEW_MODEL", "opus"), "Claude model")
	pf.StringVar(&cfg.Dir, "dir", envDefault("REVIEW_DIR", "."), "working directory with review files")
	pf.BoolVar(&cfg.Verbose, "verbose", os.Getenv("REVIEW_VERBOSE") == "true", "verbose output")
	pf.StringVar(&cfg.GitLabURL, "gitlab-url", os.Getenv("CI_API_V4_URL"), "GitLab API URL")
	pf.StringVar(&cfg.GitLabToken, "gitlab-token", os.Getenv("REVIEWER_GITLAB_TOKEN"), "GitLab API token")
	pf.StringVar(&cfg.MRIID, "mr-iid", os.Getenv("CI_MERGE_REQUEST_IID"), "MR IID")
	pf.StringVar(&cfg.ProjectID, "project-id", os.Getenv("CI_PROJECT_ID"), "GitLab project ID")
	pf.StringVar(&cfg.SourceBranch, "source-branch", os.Getenv("CI_MERGE_REQUEST_SOURCE_BRANCH_NAME"), "source branch")
	pf.StringVar(&cfg.TargetBranch, "target-branch", os.Getenv("CI_MERGE_REQUEST_TARGET_BRANCH_NAME"), "target branch")
	pf.StringVar(&cfg.Commit, "commit", os.Getenv("CI_COMMIT_SHA"), "commit SHA")
	pf.StringVar(&cfg.Author, "author", os.Getenv("GITLAB_USER_LOGIN"), "MR author")
	pf.StringVar(&cfg.MRTitle, "mr-title", os.Getenv("CI_MERGE_REQUEST_TITLE"), "MR title")
	pf.StringVar(&cfg.ExternalID, "external-id", os.Getenv("CI_MERGE_REQUEST_IID"), "external ID")
	pf.StringVar(&cfg.DiffBaseSHA, "diff-base-sha", os.Getenv("CI_MERGE_REQUEST_DIFF_BASE_SHA"), "diff base SHA")
	pf.StringVar(&cfg.SessionID, "session", "", "Claude session ID for --resume (reuses prompt cache)")

	reviewCmd := &cobra.Command{
		Use:   "review",
		Short: "Full review cycle: prompt → Claude → upload → comment → HTML",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := cfg.Validate("review"); err != nil {
				return err
			}
			log := slog.Default()
			runner := &ctl.ExecClaudeRunner{Model: cfg.Model, Dir: cfg.Dir, SessionID: cfg.SessionID, Log: log}
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

func envDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

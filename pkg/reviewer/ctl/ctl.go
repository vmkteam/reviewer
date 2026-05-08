package ctl

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"reviewsrv/pkg/rest"
)

// Controller orchestrates the review flow.
type Controller struct {
	cfg    *Config
	log    *slog.Logger
	prompt *PromptClient
	upload *UploadClient
	gitlab *GitLabClient
	runner ClaudeRunner
}

// NewController creates a new Controller from Config.
func NewController(cfg *Config, runner ClaudeRunner, log *slog.Logger) *Controller {
	c := &Controller{
		cfg:    cfg,
		log:    log,
		prompt: NewPromptClient(log),
		upload: NewUploadClient(log),
		runner: runner,
	}

	if cfg.HasGitLab() {
		c.gitlab = NewGitLabClient(cfg, log)
	}

	return c
}

// Review runs the full review flow: fetch prompt → Claude → parse → upload → comment → HTML.
func (c *Controller) Review(ctx context.Context) error {
	start := time.Now()
	c.log.InfoContext(ctx, "starting review", "projectKey", c.cfg.Key, "model", c.cfg.Model)

	// 1. Fetch prompt.
	prompt, err := c.prompt.FetchPrompt(ctx, c.cfg.URL, c.cfg.Key)
	if err != nil {
		return fmt.Errorf("fetch prompt: %w", err)
	}
	prompt = SubstituteVariables(prompt, c.cfg)

	// 2. Run Claude.
	result, err := c.runner.Run(ctx, prompt)
	if err != nil {
		return fmt.Errorf("run claude: %w", err)
	}

	// 3. Parse review.json.
	draft, err := ReadReviewJSON(c.cfg.Dir)
	if err != nil {
		return fmt.Errorf("read review: %w", err)
	}

	// 4. Merge cost data from Claude result.
	draft.Review.ModelInfo = result.ToModelInfo(c.cfg.Model)
	draft.Review.ModelInfo.Runner = c.runner.Name()
	draft.Review.DurationMs = result.DurationMs

	// 5. Fill MR metadata from CI env.
	c.fillMetadata(draft)

	// 6. Find MD files + upload + comment + HTML.
	mdFiles, err := FindMDFiles(c.cfg.Dir)
	if err != nil {
		return fmt.Errorf("find md files: %w", err)
	}

	reviewID, err := c.upload.UploadAll(ctx, c.cfg.URL, c.cfg.Key, draft, mdFiles)
	if err != nil {
		return fmt.Errorf("upload: %w", err)
	}

	c.postComments(ctx, draft, reviewID)
	c.generateHTML(draft, mdFiles)

	c.log.InfoContext(ctx, "review completed", "reviewId", reviewID, "duration", time.Since(start).Round(time.Second))
	return nil
}

// Upload uploads local review.json + R*.md files to the server.
func (c *Controller) Upload(ctx context.Context) error {
	c.log.InfoContext(ctx, "starting upload", "dir", c.cfg.Dir)

	draft, err := ReadReviewJSON(c.cfg.Dir)
	if err != nil {
		return fmt.Errorf("read review: %w", err)
	}

	c.fillMetadata(draft)

	mdFiles, err := FindMDFiles(c.cfg.Dir)
	if err != nil {
		return fmt.Errorf("find md files: %w", err)
	}

	reviewID, err := c.upload.UploadAll(ctx, c.cfg.URL, c.cfg.Key, draft, mdFiles)
	if err != nil {
		return fmt.Errorf("upload: %w", err)
	}

	c.postComments(ctx, draft, reviewID)
	c.generateHTML(draft, mdFiles)

	c.log.InfoContext(ctx, "upload completed", "reviewId", reviewID)
	return nil
}

// Comment posts MR comments for an existing review.
func (c *Controller) Comment(ctx context.Context) error {
	if c.gitlab == nil {
		c.log.WarnContext(ctx, "gitlab not configured, skipping comment")
		return nil
	}

	draft, err := ReadReviewJSON(c.cfg.Dir)
	if err != nil {
		return fmt.Errorf("read review: %w", err)
	}

	c.gitlab.PostAllComments(ctx, draft, c.reviewURL(c.cfg.ReviewID))

	c.log.InfoContext(ctx, "comment completed", "reviewId", c.cfg.ReviewID)
	return nil
}

func (c *Controller) fillMetadata(draft *rest.ReviewDraft) {
	if draft.Review.ExternalID == "" && c.cfg.ExternalID != "" {
		draft.Review.ExternalID = c.cfg.ExternalID
	}
	if draft.Review.Author == "" && c.cfg.Author != "" {
		draft.Review.Author = c.cfg.Author
	}
	if draft.Review.SourceBranch == "" && c.cfg.SourceBranch != "" {
		draft.Review.SourceBranch = c.cfg.SourceBranch
	}
	if draft.Review.TargetBranch == "" && c.cfg.TargetBranch != "" {
		draft.Review.TargetBranch = c.cfg.TargetBranch
	}
	if draft.Review.Title == "" && c.cfg.MRTitle != "" {
		draft.Review.Title = c.cfg.MRTitle
	}
	if draft.Review.CommitHash == "" && c.cfg.Commit != "" {
		draft.Review.CommitHash = c.cfg.Commit
	}
}

func (c *Controller) postComments(ctx context.Context, draft *rest.ReviewDraft, reviewID int) {
	if c.gitlab == nil {
		c.log.InfoContext(ctx, "gitlab not configured, skipping comments")
		return
	}
	c.log.InfoContext(ctx, "posting gitlab comments", "reviewId", reviewID)
	c.gitlab.PostAllComments(ctx, draft, c.reviewURL(reviewID))
}

func (c *Controller) reviewURL(reviewID int) string {
	return fmt.Sprintf("%s/reviews/%d/", strings.TrimRight(c.cfg.PublicBaseURL(), "/"), reviewID)
}

func (c *Controller) generateHTML(draft *rest.ReviewDraft, mdFiles map[string]string) {
	if err := GenerateHTML(c.cfg.Dir, draft.Review.Title, mdFiles); err != nil {
		c.log.WarnContext(context.Background(), "failed to generate HTML", "err", err)
	}
}

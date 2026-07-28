package ctl

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"

	"reviewsrv/pkg/rest"
	"reviewsrv/pkg/reviewer/runner"
)

// Controller orchestrates the review flow.
type Controller struct {
	cfg    *Config
	log    *slog.Logger
	prompt *PromptClient
	upload *UploadClient
	gitlab *GitLabClient
	runner runner.ReviewRunner

	// runnerFactory builds a runner from a per-member config; used by the panel
	// path to build one runner per member after creating its worktree.
	runnerFactory RunnerFactory
}

// RunnerFactory builds a runner from a (per-member) config.
type RunnerFactory func(*Config) (runner.ReviewRunner, error)

// Option configures a Controller.
type Option func(*Controller)

// WithRunnerFactory sets the factory the panel path uses to build per-member runners.
func WithRunnerFactory(f RunnerFactory) Option {
	return func(c *Controller) { c.runnerFactory = f }
}

// NewController creates a new Controller from Config.
func NewController(cfg *Config, rr runner.ReviewRunner, log *slog.Logger, opts ...Option) *Controller {
	c := &Controller{
		cfg:    cfg,
		log:    log,
		prompt: NewPromptClient(log),
		upload: NewUploadClient(log),
		runner: rr,
	}

	if cfg.HasGitLab() {
		c.gitlab = NewGitLabClient(cfg, log)
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Review runs the full review flow: fetch prompt → Claude → parse → upload → comment → HTML.
func (c *Controller) Review(ctx context.Context) (retErr error) {
	start := time.Now()
	c.log.InfoContext(ctx, "starting review", "projectKey", c.cfg.Key, "model", c.cfg.Model)

	// Multi-review: a configured panel fans out to one member review per runner,
	// each in its own git worktree. No judge/fusion yet — members are the output.
	if len(c.cfg.Multi) > 0 {
		return c.reviewPanel(ctx, start)
	}

	// Set when the runner skipped Step 2; forces a debug-bundle upload so
	// the silent skip can be post-mortemed even on otherwise-clean runs.
	var skipDetected bool
	retried := false

	// Publish artifacts to the debug ring buffer when something failed, when
	// --debug-upload was passed, or when we caught a Step-2 skip (so the
	// jsonl/MDs are kept for analysis). Detached context survives ctx
	// cancellation so a killed CI job still has a chance to ship its bundle.
	defer func() {
		if retErr == nil && !c.cfg.DebugUpload && !skipDetected {
			return
		}
		upCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		c.uploadDebugBundle(upCtx, c.cfg, retErr)
	}()

	// Wipe a previous run's outputs (R*.md, session logs, review.json) so the
	// runner reviews a clean tree — otherwise its glob/grep/read tools surface
	// stale artifacts as if they were part of the codebase. Best-effort.
	if err := CleanReviewArtifacts(c.cfg.Dir); err != nil {
		c.log.WarnContext(ctx, "clean review artifacts", "err", err)
	}

	// Drop a canonical empty review.json on disk first so the runner fills it
	// in place instead of inventing the schema. Done before fetching the prompt
	// so that even a quick failure here doesn't waste an HTTP round-trip.
	if err := WriteReviewSkeleton(c.cfg.Dir, c.cfg); err != nil {
		return fmt.Errorf("write review.json skeleton: %w", err)
	}

	prompt, err := c.prompt.FetchPrompt(ctx, c.cfg.URL, c.cfg.Key)
	if err != nil {
		return fmt.Errorf("fetch prompt: %w", err)
	}
	prompt = SubstituteVariables(prompt, c.cfg)

	result, err := c.runner.Run(ctx, prompt)
	if err != nil {
		return fmt.Errorf("run claude: %w", err)
	}

	draft, err := ReadReviewJSON(c.cfg.Dir)
	if err != nil {
		c.logReviewJSONFailure(ctx, draft)
		return fmt.Errorf("read review: %w", err)
	}

	c.applyRunResult(ctx, draft, c.cfg, c.runner, result)

	if isReviewJSONUnfilled(draft) {
		skipDetected = true
		retried = true
		if d2 := c.runStep2Recovery(ctx, draft, result); d2 != nil {
			draft = d2
			skipDetected = false
			// applyRunResult persisted the pre-recovery draft; keep review.json in
			// sync with what is actually uploaded.
			if werr := WriteReviewJSON(c.cfg.Dir, draft); werr != nil {
				c.log.WarnContext(ctx, "persist recovered review.json", "err", werr)
			}
		}
	}

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

	c.log.InfoContext(ctx, "review completed", "reviewId", reviewID, "duration", time.Since(start).Round(time.Second), "retried", retried)
	return nil
}

// uploadDebugBundle publishes on-disk artifacts so a failed CI run can be
// inspected via /v1/debug/storage/. Best-effort — never returns an error.
// The empty-bundle short-circuit lives in UploadClient.UploadDebugBundle.
// cfg is the run whose dir/identity to bundle — the base config for a single
// review, a member's clone for a panel member.
func (c *Controller) uploadDebugBundle(ctx context.Context, cfg *Config, runErr error) {
	files := CollectDebugArtifacts(cfg.Dir)

	meta := DebugMeta{
		MRIid:        cfg.MRIID,
		ExternalID:   cfg.ExternalID,
		Runner:       cfg.Runner,
		Model:        cfg.Model,
		SourceBranch: cfg.SourceBranch,
		TargetBranch: cfg.TargetBranch,
		CommitHash:   cfg.Commit,
	}
	if runErr != nil {
		meta.ErrorMsg = runErr.Error()
	}

	url, err := c.upload.UploadDebugBundle(ctx, cfg.URL, cfg.Key, meta, files)
	if err != nil {
		c.log.WarnContext(ctx, "failed to upload debug bundle", "runner", cfg.Runner, "model", cfg.Model, "err", err)
		return
	}
	if url == "" {
		return
	}

	// runner/model identify WHOSE bundle this is when several panel members
	// fail concurrently and their upload lines interleave.
	full := strings.TrimRight(cfg.PublicBaseURL(), "/") + url
	c.log.InfoContext(ctx, "debug bundle uploaded", "url", full, "files", len(files), "runner", cfg.Runner, "model", cfg.Model)
}

func (c *Controller) logReviewJSONFailure(ctx context.Context, draft *rest.ReviewDraft) {
	if draft == nil {
		c.log.WarnContext(ctx, "review.json could not be parsed")
		return
	}
	reviewTypes := make([]string, len(draft.Files))
	for i, f := range draft.Files {
		reviewTypes[i] = f.ReviewType
	}
	fileTypes := make([]string, len(draft.Issues))
	for i, iss := range draft.Issues {
		fileTypes[i] = iss.LocalID + "=" + iss.FileType
	}
	c.log.WarnContext(ctx, "review.json validation failed",
		"files", len(draft.Files),
		"issues", len(draft.Issues),
		"reviewTypes", reviewTypes,
		"fileTypes", fileTypes,
	)
}

// Upload uploads local review.json + R*.md files to the server.
func (c *Controller) Upload(ctx context.Context) error {
	c.log.InfoContext(ctx, "starting upload", "dir", c.cfg.Dir)

	draft, err := ReadReviewJSON(c.cfg.Dir)
	if err != nil {
		return fmt.Errorf("read review: %w", err)
	}

	c.fillMetadata(ctx, draft)
	if isReviewJSONUnfilled(draft) {
		c.log.WarnContext(ctx, "review.json appears unfilled (skeleton uploaded as-is) — Upload subcommand cannot retry, run `reviewctl review` to regenerate", "files", len(draft.Files), "issues", len(draft.Issues))
	}

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

// applyRunResult records the run's model/runner/timing metadata and the resolved
// profile snapshot on a freshly-read draft, then fills MR metadata. Shared by the
// single review, panel members and the judge so all three populate identically.
// The enriched draft is persisted back to review.json so the metadata survives a
// failed upload and a later standalone `reviewctl upload` re-sends it intact.
func (c *Controller) applyRunResult(ctx context.Context, draft *rest.ReviewDraft, cfg *Config, rr runner.ReviewRunner, result *runner.ClaudeResult) {
	draft.Review.ModelInfo = result.ToModelInfo(cfg.Model)
	draft.Review.ModelInfo.Runner = rr.Name()
	draft.Review.DurationMs = result.DurationMs
	draft.Review.RunnerProfile = cfg.RunnerProfileSnapshot()
	c.fillMetadata(ctx, draft)
	if err := WriteReviewJSON(cfg.Dir, draft); err != nil {
		c.log.WarnContext(ctx, "persist run metadata to review.json", "err", err)
	}
}

func (c *Controller) fillMetadata(ctx context.Context, draft *rest.ReviewDraft) {
	clearPlaceholders(draft)
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
	// Local runs have no CI metadata — recover what git itself knows. Best
	// effort: outside a git checkout the fields simply stay empty.
	if draft.Review.Author == "" {
		draft.Review.Author = gitMeta(ctx, c.cfg.Dir, "log", "-1", "--format=%an")
	}
	if draft.Review.SourceBranch == "" {
		draft.Review.SourceBranch = gitMeta(ctx, c.cfg.Dir, "branch", "--show-current")
	}
	if draft.Review.CommitHash == "" {
		draft.Review.CommitHash = gitMeta(ctx, c.cfg.Dir, "rev-parse", "HEAD")
	}
}

// gitMeta returns the trimmed output of a git command in dir, or "" on any
// error — callers treat the value as optional metadata.
func gitMeta(ctx context.Context, dir string, args ...string) string {
	out, err := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
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

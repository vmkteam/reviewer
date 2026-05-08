package ctl

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"reviewsrv/pkg/rest"
	"reviewsrv/pkg/reviewer"
)

// Controller orchestrates the review flow.
type Controller struct {
	cfg    *Config
	log    *slog.Logger
	prompt *PromptClient
	upload *UploadClient
	gitlab *GitLabClient
	runner ReviewRunner
}

// NewController creates a new Controller from Config.
func NewController(cfg *Config, runner ReviewRunner, log *slog.Logger) *Controller {
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
func (c *Controller) Review(ctx context.Context) (retErr error) {
	start := time.Now()
	c.log.InfoContext(ctx, "starting review", "projectKey", c.cfg.Key, "model", c.cfg.Model)

	// Tracks whether the runner skipped Step 2 (review.json left as skeleton).
	// Set after the runner finishes; used by the defer block below to force
	// a debug-bundle upload even on otherwise-successful runs so we can post-
	// mortem the silent skip.
	var skipDetected bool

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
		c.uploadDebugBundle(upCtx, retErr)
	}()

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

	draft.Review.ModelInfo = result.ToModelInfo(c.cfg.Model)
	draft.Review.ModelInfo.Runner = c.runner.Name()
	draft.Review.DurationMs = result.DurationMs

	c.fillMetadata(draft)

	if isReviewJSONUnfilled(draft) {
		skipDetected = true
		c.log.WarnContext(ctx, "review.json appears unfilled (skeleton uploaded as-is) — attempting Step 2 retry with session continuation",
			"files", len(draft.Files),
			"issues", len(draft.Issues),
			"sessionId", result.SessionID,
		)
		if d2 := c.retryStep2(ctx, result.SessionID); d2 != nil {
			d2.Review.ModelInfo = draft.Review.ModelInfo
			d2.Review.DurationMs = draft.Review.DurationMs
			c.fillMetadata(d2)
			if !isReviewJSONUnfilled(d2) {
				draft = d2
				skipDetected = false
				c.log.InfoContext(ctx, "Step 2 retry filled review.json", "issues", len(draft.Issues))
			} else {
				c.log.WarnContext(ctx, "Step 2 retry did not fill review.json")
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

	c.log.InfoContext(ctx, "review completed", "reviewId", reviewID, "duration", time.Since(start).Round(time.Second))
	return nil
}

// isReviewJSONUnfilled detects the "model skipped Step 2" failure mode:
// runner produced MD files but never edited review.json, so the skeleton is
// uploaded as-is. Heuristic: all files[].summary blank AND no issues — even
// a clean MR should yield non-empty summaries.
func isReviewJSONUnfilled(draft *rest.ReviewDraft) bool {
	if draft == nil {
		return false
	}
	for _, f := range draft.Files {
		if strings.TrimSpace(f.Summary) != "" {
			return false
		}
	}
	return len(draft.Issues) == 0
}

// retryStep2 invokes the runner a second time with a focused "fill review.json"
// prompt, resuming the previous session so the cached original prompt isn't
// re-billed. Returns the re-read draft on success; nil if retry can't happen
// or the runner failed (caller stays with the original skeleton draft).
func (c *Controller) retryStep2(ctx context.Context, lastSessionID string) *rest.ReviewDraft {
	if lastSessionID == "" {
		c.log.WarnContext(ctx, "Step 2 retry skipped: no sessionId from previous run")
		return nil
	}
	if c.runner == nil {
		return nil
	}

	setRunnerSession(c.runner, lastSessionID)

	if _, err := c.runner.Run(ctx, reviewer.PromptStep2Retry); err != nil {
		c.log.WarnContext(ctx, "Step 2 retry runner failed", "err", err)
		return nil
	}

	draft, err := ReadReviewJSON(c.cfg.Dir)
	if err != nil {
		c.log.WarnContext(ctx, "Step 2 retry: review.json still unparseable", "err", err)
		return nil
	}
	return draft
}

// setRunnerSession mutates a runner to resume sessionID on its next Run call.
// Silent no-op for unknown runner types.
func setRunnerSession(r ReviewRunner, sessionID string) {
	switch x := r.(type) {
	case *ExecClaudeRunner:
		x.SessionID = sessionID
		x.ContinueSession = false
	case *ExecOpenCodeRunner:
		x.SessionID = sessionID
		x.ContinueSession = false
	}
}

// uploadDebugBundle publishes on-disk artifacts so a failed CI run can be
// inspected via /v1/debug/storage/. Best-effort — never returns an error.
// The empty-bundle short-circuit lives in UploadClient.UploadDebugBundle.
func (c *Controller) uploadDebugBundle(ctx context.Context, runErr error) {
	files := CollectDebugArtifacts(c.cfg.Dir)

	meta := DebugMeta{
		MRIid:        c.cfg.MRIID,
		ExternalID:   c.cfg.ExternalID,
		Runner:       c.cfg.Runner,
		Model:        c.cfg.Model,
		SourceBranch: c.cfg.SourceBranch,
		TargetBranch: c.cfg.TargetBranch,
		CommitHash:   c.cfg.Commit,
	}
	if runErr != nil {
		meta.ErrorMsg = runErr.Error()
	}

	url, err := c.upload.UploadDebugBundle(ctx, c.cfg.URL, c.cfg.Key, meta, files)
	if err != nil {
		c.log.WarnContext(ctx, "failed to upload debug bundle", "err", err)
		return
	}
	if url == "" {
		return
	}

	full := strings.TrimRight(c.cfg.PublicBaseURL(), "/") + url
	c.log.InfoContext(ctx, "debug bundle uploaded", "url", full, "files", len(files))
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

	c.fillMetadata(draft)
	if isReviewJSONUnfilled(draft) {
		c.log.WarnContext(ctx, "review.json appears unfilled (skeleton uploaded as-is) — Upload subcommand cannot retry, run `reviewctl review` to regenerate",
			"files", len(draft.Files),
			"issues", len(draft.Issues),
		)
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

package ctl

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"sync"
	"time"

	"reviewsrv/pkg/db"
	"reviewsrv/pkg/rest"
	"reviewsrv/pkg/reviewer"
	"reviewsrv/pkg/reviewer/runner"
)

// Controller orchestrates the review flow.
type Controller struct {
	cfg    *Config
	log    *slog.Logger
	prompt *PromptClient
	upload *UploadClient
	gitlab *GitLabClient
	runner *spendTracker

	// runnerFactory builds a runner from a per-member config; used by the panel
	// path to build one runner per member after creating its worktree.
	runnerFactory RunnerFactory

	// spent adds up a panel's runs — members and judge, failed ones included —
	// for the closing log line. Members run concurrently, hence the mutex.
	spentMu sync.Mutex
	spent   float64
}

// RunnerFactory builds a runner from a (per-member) config.
type RunnerFactory func(*Config) (runner.ReviewRunner, error)

// Option configures a Controller.
type Option func(*Controller)

// WithRunnerFactory sets the factory the panel path uses to build per-member runners.
func WithRunnerFactory(f RunnerFactory) Option {
	return func(c *Controller) { c.runnerFactory = f }
}

// spendTracker wraps a ReviewRunner and keeps the result of each of its runs —
// failed ones included, as they are billed too — so the review record and the
// debug bundle cover the whole run: a Step 2 retry, a retried judge.
type spendTracker struct {
	runner.ReviewRunner
	results []*runner.ClaudeResult
}

// trackSpend wraps rr; a nil runner stays nil.
func trackSpend(rr runner.ReviewRunner) *spendTracker {
	if rr == nil {
		return nil
	}
	return &spendTracker{ReviewRunner: rr}
}

// Run implements runner.ReviewRunner.
func (t *spendTracker) Run(ctx context.Context, prompt string) (*runner.ClaudeResult, error) {
	res, err := t.ReviewRunner.Run(ctx, prompt)
	if res != nil {
		t.results = append(t.results, res)
	}
	return res, err
}

// total returns what the runs spent so far; 0 for a nil tracker.
func (t *spendTracker) total() float64 {
	if t == nil {
		return 0
	}
	var usd float64
	for _, r := range t.results {
		usd += r.TotalCostUSD
	}
	return usd
}

// usage merges the model usage and durations of every run.
func (t *spendTracker) usage(model string) (db.ReviewModelInfo, int) {
	var (
		mi  db.ReviewModelInfo
		dur int
	)
	for i, r := range t.results {
		if i == 0 {
			mi = r.ToModelInfo(model)
		} else {
			mi.Add(r.ToModelInfo(model))
		}
		dur += r.DurationMs
	}
	return mi, dur
}

// NewController creates a new Controller from Config.
func NewController(cfg *Config, rr runner.ReviewRunner, log *slog.Logger, opts ...Option) *Controller {
	c := &Controller{
		cfg:    cfg,
		log:    log,
		prompt: NewPromptClient(log),
		upload: NewUploadClient(log),
		runner: trackSpend(rr),
	}

	if cfg.HasGitLab() {
		c.gitlab = NewGitLabClient(cfg, log)
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

func (c *Controller) spentTotal() float64 {
	c.spentMu.Lock()
	defer c.spentMu.Unlock()
	return c.spent
}

// Review runs the full review flow: fetch prompt → Claude → parse → upload → comment → HTML.
func (c *Controller) Review(ctx context.Context) (retErr error) {
	start := time.Now()
	// Multi-review: a configured panel fans out to one member review per runner,
	// each in its own git worktree, then a judge fuses them.
	if len(c.cfg.Multi) > 0 {
		return c.reviewPanel(ctx, start)
	}
	c.log.InfoContext(ctx, "starting review", "projectKey", reviewer.ShortKey(c.cfg.Key), "runner", c.cfg.Runner, "model", c.cfg.Model)

	retried := false

	defer func() { c.settleRun(ctx, c.cfg, c.runner, retErr) }()

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
		return fmt.Errorf("run %s: %w", c.runner.Name(), err)
	}

	draft, err := ReadReviewJSON(c.cfg.Dir)
	if err != nil {
		c.logReviewJSONFailure(ctx, draft)
		return fmt.Errorf("read review: %w", err)
	}

	c.applyRunResult(ctx, draft, c.cfg, c.runner)

	if isReviewJSONUnfilled(draft) {
		retried = true
		if draft, err = c.runStep2Recovery(ctx, draft, result); err != nil {
			return err
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

	c.log.InfoContext(ctx, "review completed", "reviewId", reviewID, "duration", time.Since(start).Round(time.Second),
		"retried", retried, "costUsd", c.runner.total())
	return nil
}

// settleRun closes one runner's run: its spend joins the panel total, and a
// failed run — or any run under --debug-upload — ships its artifacts to the
// debug ring before a panel worktree goes. The upload gets a detached context:
// a cancelled or timed-out run is precisely what the bundle should explain.
func (c *Controller) settleRun(ctx context.Context, cfg *Config, rr *spendTracker, runErr error) {
	c.spentMu.Lock()
	c.spent += rr.total()
	c.spentMu.Unlock()
	if runErr == nil && !cfg.DebugUpload {
		return
	}
	upCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), debugUploadTimeout)
	defer cancel()
	c.uploadDebugBundle(upCtx, cfg, runErr, rr.total())
}

// debugUploadTimeout bounds shipping a debug bundle.
const debugUploadTimeout = 30 * time.Second

// uploadDebugBundle publishes on-disk artifacts so a failed CI run can be
// inspected via /v1/debug/storage/. Best-effort — never returns an error.
// The empty-bundle short-circuit lives in UploadClient.UploadDebugBundle.
// cfg is the run whose dir/identity to bundle — the base config for a single
// review, a member's clone for a panel member.
func (c *Controller) uploadDebugBundle(ctx context.Context, cfg *Config, runErr error, costUsd float64) {
	files := CollectDebugArtifacts(cfg.Dir)

	meta := DebugMeta{
		MRIid:        cfg.MRIID,
		ExternalID:   cfg.ExternalID,
		Runner:       cfg.Runner,
		Model:        cfg.Model,
		SourceBranch: cfg.SourceBranch,
		TargetBranch: cfg.TargetBranch,
		CommitHash:   cfg.Commit,
		CostUsd:      costUsd,
	}
	meta.Status, meta.Reason = reviewer.RunOutcome(runErr)
	var created *reviewCreatedError
	if errors.As(runErr, &created) {
		meta.ReviewID = created.reviewID
	}
	if runErr != nil {
		meta.ErrorMsg = runErr.Error()
		// A cancelled job is expected, not a failure worth a warning.
		lvl := slog.LevelWarn
		if meta.Status == reviewer.RunStatusCancelled {
			lvl = slog.LevelInfo
		}
		c.log.Log(ctx, lvl, "review run did not complete", "status", meta.Status, "reason", meta.Reason,
			"runner", cfg.Runner, "model", cfg.Model, "costUsd", costUsd, "err", runErr)
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

// applyRunResult records the run's model/runner/timing metadata — every run
// of rr so far, failed ones included — and the resolved profile snapshot on a
// freshly-read draft, then fills MR metadata. Shared by the single review
// (again after a Step 2 retry), panel members and the judge so all populate
// identically. The enriched draft is persisted back to review.json so the
// metadata survives a failed upload and a later standalone `reviewctl upload`
// re-sends it intact.
func (c *Controller) applyRunResult(ctx context.Context, draft *rest.ReviewDraft, cfg *Config, rr *spendTracker) {
	draft.Review.ModelInfo, draft.Review.DurationMs = rr.usage(cfg.Model)
	draft.Review.ModelInfo.Runner = rr.Name()
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
	cmd := exec.CommandContext(ctx, "git", reviewer.GitArgs(append([]string{"-C", dir}, args...)...)...)
	cmd.Env = reviewer.ChildEnv()
	out, err := cmd.Output()
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

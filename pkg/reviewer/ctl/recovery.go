package ctl

import (
	"context"
	"strings"

	"reviewsrv/pkg/rest"
	"reviewsrv/pkg/reviewer"
)

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

// runStep2Recovery wraps the retry path: logs the skip, invokes retryStep2,
// merges metrics from both passes, and returns the recovered draft (or nil
// if retry didn't help, so caller keeps the original skeleton).
func (c *Controller) runStep2Recovery(ctx context.Context, draft *rest.ReviewDraft, first *ClaudeResult) *rest.ReviewDraft {
	c.log.WarnContext(ctx, "review.json appears unfilled (skeleton uploaded as-is) — attempting Step 2 retry with session continuation", "files", len(draft.Files), "issues", len(draft.Issues), "sessionId", first.SessionID)

	d2, retryRes := c.retryStep2(ctx, first.SessionID)
	if d2 == nil {
		return nil
	}

	d2.Review.ModelInfo = draft.Review.ModelInfo
	d2.Review.DurationMs = draft.Review.DurationMs
	if retryRes != nil {
		d2.Review.ModelInfo.Add(retryRes.ToModelInfo(c.cfg.Model))
		d2.Review.DurationMs += retryRes.DurationMs
	}
	c.fillMetadata(d2)

	if isReviewJSONUnfilled(d2) {
		c.log.WarnContext(ctx, "Step 2 retry did not fill review.json")
		return nil
	}
	c.log.InfoContext(ctx, "Step 2 retry filled review.json", "issues", len(d2.Issues))
	return d2
}

// retryStep2 invokes the runner a second time with a focused "fill review.json"
// prompt, resuming the previous session so the cached original prompt isn't
// re-billed. Returns the re-read draft and the runner result for metric
// aggregation; nil draft signals "retry could not happen or runner failed"
// and the caller stays with the original skeleton draft.
func (c *Controller) retryStep2(ctx context.Context, lastSessionID string) (*rest.ReviewDraft, *ClaudeResult) {
	if lastSessionID == "" {
		c.log.WarnContext(ctx, "Step 2 retry skipped: no sessionId from previous run")
		return nil, nil
	}
	if c.runner == nil {
		return nil, nil
	}

	c.runner.SetSession(lastSessionID)

	res, err := c.runner.Run(ctx, reviewer.PromptStep2Retry)
	if err != nil {
		c.log.WarnContext(ctx, "Step 2 retry runner failed", "err", err)
		return nil, nil
	}

	draft, err := ReadReviewJSON(c.cfg.Dir)
	if err != nil {
		c.log.WarnContext(ctx, "Step 2 retry: review.json still unparseable", "err", err)
		return nil, nil
	}
	return draft, res
}

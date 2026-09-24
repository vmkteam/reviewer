package ctl

import (
	"context"
	"strings"

	"reviewsrv/pkg/rest"
	"reviewsrv/pkg/reviewer"
	"reviewsrv/pkg/reviewer/runner"
)

// isReviewJSONUnfilled detects a runner that never edited review.json — the
// skeleton would be uploaded as-is. Heuristic: all files[].summary blank AND
// no issues; even a clean MR should yield non-empty summaries. Single-review
// mode reacts with the Step 2 retry below; the panel treats such a member (or
// judge attempt) as failed.
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
// records both passes on the recovered draft, and returns it (or nil if the
// retry didn't help, which fails the run).
func (c *Controller) runStep2Recovery(ctx context.Context, draft *rest.ReviewDraft, first *runner.ClaudeResult) *rest.ReviewDraft {
	c.log.WarnContext(ctx, "review.json appears unfilled — attempting Step 2 retry with session continuation", "files", len(draft.Files), "issues", len(draft.Issues), "sessionId", first.SessionID)

	d2 := c.retryStep2(ctx, first.SessionID)
	if d2 == nil {
		return nil
	}
	// The tracker holds both passes, so the record covers the retry too.
	c.applyRunResult(ctx, d2, c.cfg, c.runner)

	if isReviewJSONUnfilled(d2) {
		c.log.WarnContext(ctx, "Step 2 retry did not fill review.json")
		return nil
	}
	c.log.InfoContext(ctx, "Step 2 retry filled review.json", "issues", len(d2.Issues))
	return d2
}

// retryStep2 invokes the runner a second time with a focused "fill review.json"
// prompt, resuming the previous session so the cached original prompt isn't
// re-billed. Returns the re-read draft; nil signals "retry could not happen or
// runner failed".
func (c *Controller) retryStep2(ctx context.Context, lastSessionID string) *rest.ReviewDraft {
	if lastSessionID == "" {
		c.log.WarnContext(ctx, "Step 2 retry skipped: no sessionId from previous run")
		return nil
	}
	if c.runner == nil {
		return nil
	}

	c.runner.SetSession(lastSessionID)

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

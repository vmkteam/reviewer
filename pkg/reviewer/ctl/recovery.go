package ctl

import (
	"context"
	"errors"
	"fmt"
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
// records both passes on the recovered draft, and returns it. A retry that
// could not fill review.json fails the run as not submitted — unless the retry
// run failed for a reason of its own (cancelled, timeout, billing).
func (c *Controller) runStep2Recovery(ctx context.Context, draft *rest.ReviewDraft, first *runner.ClaudeResult) (*rest.ReviewDraft, error) {
	c.log.WarnContext(ctx, "review.json appears unfilled — attempting Step 2 retry with session continuation", "files", len(draft.Files), "issues", len(draft.Issues), "sessionId", first.SessionID)

	d2, err := c.retryStep2(ctx, first.SessionID)
	if err != nil {
		return nil, reviewer.WithRunReason(reviewer.RunReasonNotSubmitted, err)
	}
	// The tracker holds both passes, so the record covers the retry too.
	c.applyRunResult(ctx, d2, c.cfg, c.runner)
	if isReviewJSONUnfilled(d2) {
		return nil, errEmptyReview("the Step 2 retry did not fill it")
	}
	c.log.InfoContext(ctx, "Step 2 retry filled review.json", "issues", len(d2.Issues))
	return d2, nil
}

// retryStep2 invokes the runner a second time with a focused "fill review.json"
// prompt, resuming the previous session so the cached original prompt isn't
// re-billed. Returns the re-read draft, or why there is none.
func (c *Controller) retryStep2(ctx context.Context, lastSessionID string) (*rest.ReviewDraft, error) {
	if lastSessionID == "" || c.runner == nil {
		return nil, errEmptyReview("there is no session to resume for a Step 2 retry")
	}

	c.runner.SetSession(lastSessionID)

	if _, err := c.runner.Run(ctx, reviewer.PromptStep2Retry); err != nil {
		return nil, fmt.Errorf("run %s: step 2 retry: %w", c.runner.Name(), err)
	}

	draft, err := ReadReviewJSON(c.cfg.Dir)
	if err != nil {
		return nil, errEmptyReview("the Step 2 retry left it unparseable: " + err.Error())
	}
	return draft, nil
}

// errEmptyReview fails a run whose review.json stayed an untouched skeleton,
// like an empty panel member: that is no review, so the run fails (and ships
// its debug bundle) instead of uploading it.
func errEmptyReview(why string) error {
	return reviewer.WithRunReason(reviewer.RunReasonNotSubmitted,
		errors.New("empty review: the runner left review.json unfilled, and "+why))
}

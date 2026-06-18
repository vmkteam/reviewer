package ctl

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"reviewsrv/pkg/reviewer"
)

// reviewPanel runs each --multi panel member in its own detached git worktree and
// uploads it as a reviewRole=member review. Members share the project prompt and
// MR metadata; only the runner/model/working dir differ. Sequential for now
// (parallelism is a later phase) and there is no judge/fusion yet, so the member
// reviews are the only output. Fails fast on the first member error.
func (c *Controller) reviewPanel(ctx context.Context, start time.Time) error {
	if c.runnerFactory == nil {
		return errors.New("panel review requires a runner factory")
	}

	prompt, err := c.prompt.FetchPrompt(ctx, c.cfg.URL, c.cfg.Key)
	if err != nil {
		return fmt.Errorf("fetch prompt: %w", err)
	}
	prompt = SubstituteVariables(prompt, c.cfg)

	base, err := os.MkdirTemp("", "reviewmulti-")
	if err != nil {
		return fmt.Errorf("create worktree base: %w", err)
	}
	// Safety net: individual worktrees are removed in runMember; this clears any
	// directory a failed removal left behind. Detached so a cancelled ctx still cleans up.
	defer func() { _ = os.RemoveAll(base) }()

	commit := c.panelCommit()
	memberIDs := make([]int, 0, len(c.cfg.Multi))
	for i, m := range c.cfg.Multi {
		label := memberLabel(i, m)
		id, err := c.runMember(ctx, base, commit, label, m, prompt)
		if err != nil {
			return fmt.Errorf("panel member %s: %w", label, err)
		}
		memberIDs = append(memberIDs, id)
		c.log.InfoContext(ctx, "panel member uploaded", "label", label, "reviewId", id)
	}

	c.log.InfoContext(ctx, "panel completed",
		"members", len(memberIDs), "reviewIds", memberIDs, "duration", time.Since(start).Round(time.Second))
	return nil
}

// runMember reviews a single panel member in an isolated detached worktree and
// uploads the result as a member review. The worktree is removed on return.
func (c *Controller) runMember(ctx context.Context, base, commit, label string, m MemberSpec, prompt string) (int, error) {
	dir := filepath.Join(base, "wt-"+label)
	if err := c.gitWorktreeAdd(ctx, dir, commit); err != nil {
		return 0, fmt.Errorf("worktree add: %w", err)
	}
	defer c.gitWorktreeRemove(ctx, dir)

	// Clone the base config and override only the runner, model and working dir;
	// MR metadata and credentials are shared. Clear Multi so the member runs a
	// plain single review, not another panel.
	mc := *c.cfg
	mc.Dir = dir
	mc.Runner = m.Runner
	mc.Model = m.Model
	mc.Multi = nil

	rr, err := c.runnerFactory(&mc)
	if err != nil {
		return 0, fmt.Errorf("build runner: %w", err)
	}

	if err = WriteReviewSkeleton(mc.Dir, &mc); err != nil {
		return 0, fmt.Errorf("write review.json skeleton: %w", err)
	}

	result, err := rr.Run(ctx, prompt)
	if err != nil {
		return 0, fmt.Errorf("run %s: %w", mc.Runner, err)
	}

	draft, err := ReadReviewJSON(mc.Dir)
	if err != nil {
		return 0, fmt.Errorf("read review: %w", err)
	}

	draft.Review.ModelInfo = result.ToModelInfo(mc.Model)
	draft.Review.ModelInfo.Runner = rr.Name()
	draft.Review.DurationMs = result.DurationMs
	draft.Review.RunnerProfile = mc.RunnerProfileSnapshot()
	draft.Review.ReviewRole = reviewer.ReviewRoleMember
	c.fillMetadata(draft)

	mdFiles, err := FindMDFiles(mc.Dir)
	if err != nil {
		return 0, fmt.Errorf("find md files: %w", err)
	}

	id, err := c.upload.UploadAll(ctx, c.cfg.URL, c.cfg.Key, draft, mdFiles)
	if err != nil {
		return 0, fmt.Errorf("upload: %w", err)
	}
	return id, nil
}

// panelCommit is the commit checked out into each member worktree — the MR commit
// when known, else the current HEAD of the working repo.
func (c *Controller) panelCommit() string {
	if c.cfg.Commit != "" {
		return c.cfg.Commit
	}
	return "HEAD"
}

// gitWorktreeAdd creates a detached worktree at dir checked out to commit, run from
// the working repo so git finds it.
func (c *Controller) gitWorktreeAdd(ctx context.Context, dir, commit string) error {
	cmd := exec.CommandContext(ctx, "git", "worktree", "add", "--detach", dir, commit)
	cmd.Dir = c.cfg.Dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// gitWorktreeRemove tears down a member worktree. Best-effort; uses a detached
// context so cleanup still runs when the review context was cancelled.
func (c *Controller) gitWorktreeRemove(ctx context.Context, dir string) {
	cmd := exec.CommandContext(context.WithoutCancel(ctx), "git", "worktree", "remove", "--force", dir)
	cmd.Dir = c.cfg.Dir
	if out, err := cmd.CombinedOutput(); err != nil {
		c.log.WarnContext(ctx, "worktree remove", "dir", dir, "err", err, "out", strings.TrimSpace(string(out)))
	}
}

// memberLabel builds a filesystem-safe, unique label for a member, e.g.
// "1-codex-gpt-5.5". The index keeps duplicate runner:model members distinct.
func memberLabel(i int, m MemberSpec) string {
	name := m.Runner
	if m.Model != "" {
		name += "-" + m.Model
	}
	return fmt.Sprintf("%d-%s", i+1, sanitizeLabel(name))
}

// sanitizeLabel reduces a member name to a path-safe slug.
func sanitizeLabel(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	return b.String()
}

package ctl

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"reviewsrv/pkg/rest"
	"reviewsrv/pkg/reviewer"
)

// panelConcurrency bounds how many members review at once. Panels are small
// (2–3 typically); the cap guards against a pathologically large --multi list.
const panelConcurrency = 4

// memberOutput is a panel member's produced review, kept until the judge has
// staged it. The worktree dir stays alive until panel cleanup so the judge can
// read review.json + R*.md from it.
type memberOutput struct {
	spec    MemberSpec
	label   string            // provenance/source label (model-based) → members/<label>/
	dir     string            // member worktree
	draft   *rest.ReviewDraft // filled review (role set at upload time)
	mdFiles map[string]string // reviewType → R*.md path in dir
}

// reviewPanel fans out the --multi panel into per-member git worktrees, then —
// when a judge is configured and ≥2 members succeed — runs the judge over the
// members' outputs to produce one fused review. Members and the judge share the
// project prompt and MR metadata; only runner/model/working dir differ. Members
// run concurrently (bounded by panelConcurrency), and per-member failures are
// tolerated as long as one member produces a review.
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
	defer func() { _ = os.RemoveAll(base) }()

	commit := c.panelCommit()
	labels := sourceLabels(c.cfg.Multi)

	// Create the worktrees sequentially — `git worktree add` mutates the repo's
	// shared worktree metadata and isn't safe to run concurrently — then review the
	// members in parallel. Worktrees stay alive until the judge has staged them;
	// remove them all (plus the judge's) on the way out.
	dirs := make([]string, len(c.cfg.Multi))
	defer func() {
		for _, d := range dirs {
			if d != "" {
				c.gitWorktreeRemove(ctx, d)
			}
		}
	}()
	for i, m := range c.cfg.Multi {
		dir := filepath.Join(base, "wt-"+memberLabel(i, m))
		if err = c.gitWorktreeAdd(ctx, dir, commit); err != nil {
			return fmt.Errorf("panel member %s: worktree add: %w", labels[i], err)
		}
		dirs[i] = dir
	}

	outputs := c.runMembers(ctx, dirs, labels, prompt)
	if len(outputs) == 0 {
		return errors.New("panel: every member failed to produce a review")
	}

	primaryID, err := c.finishPanel(ctx, base, commit, outputs)
	if err != nil {
		return err
	}

	c.log.InfoContext(ctx, "panel completed",
		"members", len(outputs), "primaryReviewId", primaryID, "duration", time.Since(start).Round(time.Second))
	return nil
}

// runMembers reviews all panel members concurrently (bounded by panelConcurrency)
// and returns the successful outputs in panel order. Member failures are logged
// and tolerated — the caller requires at least one success. Each run is bounded by
// cfg.Timeout.
//
// Safe to parallelize for --multi: produceMember clones the config per member and
// the local flow carries no profile token, so buildRunner never mutates process
// env. The server-driven path (per-member tokens) will need per-runner creds, not
// global env, before it can fan out concurrently.
func (c *Controller) runMembers(ctx context.Context, dirs, labels []string, prompt string) []*memberOutput {
	type result struct {
		out *memberOutput
		err error
	}
	results := make([]result, len(c.cfg.Multi))

	sem := make(chan struct{}, panelConcurrency)
	var wg sync.WaitGroup
	for i, m := range c.cfg.Multi {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			mctx, cancel := withTimeout(ctx, c.cfg.Timeout)
			defer cancel()
			out, err := c.produceMember(mctx, dirs[i], labels[i], m, prompt)
			results[i] = result{out: out, err: err}
		}()
	}
	wg.Wait()

	var outputs []*memberOutput
	for i := range results {
		if results[i].err != nil {
			c.log.ErrorContext(ctx, "panel member failed", "label", labels[i], "err", results[i].err)
			continue
		}
		outputs = append(outputs, results[i].out)
		c.log.InfoContext(ctx, "panel member reviewed", "label", labels[i], "issues", len(results[i].out.draft.Issues))
	}
	return outputs
}

// withTimeout derives a context bounded by d, or a plain cancellable one when d <= 0.
func withTimeout(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if d <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, d)
}

// finishPanel decides what to upload from the produced member outputs: a fused
// review when a judge is set and ≥2 members ran, the lone member promoted to a
// standalone review when exactly one ran, or plain member reviews when no judge
// is configured (Phase 2 behaviour). Returns the primary review id.
func (c *Controller) finishPanel(ctx context.Context, base, commit string, outputs []*memberOutput) (int, error) {
	switch {
	case c.cfg.Judge != nil && len(outputs) >= 2:
		return c.fuse(ctx, base, commit, outputs)

	case c.cfg.Judge != nil && len(outputs) == 1:
		c.log.InfoContext(ctx, "judge skipped: single member promoted to single review")
		return c.uploadMember(ctx, outputs[0], reviewer.ReviewRoleSingle)

	default: // no judge → member reviews only, no fusion
		var lastID int
		for _, o := range outputs {
			id, err := c.uploadMember(ctx, o, reviewer.ReviewRoleMember)
			if err != nil {
				return 0, err
			}
			c.log.InfoContext(ctx, "panel member uploaded", "label", o.label, "reviewId", id)
			lastID = id
		}
		return lastID, nil
	}
}

// fuse runs the judge over the staged member outputs and uploads one fused
// review that links the members as children. If the judge fails after a retry,
// it degrades to a single review from the primary member so CI never goes red on
// a judge flap.
func (c *Controller) fuse(ctx context.Context, base, commit string, outputs []*memberOutput) (int, error) {
	judgeDir := filepath.Join(base, "wt-judge")
	if err := c.gitWorktreeAdd(ctx, judgeDir, commit); err != nil {
		return 0, fmt.Errorf("judge worktree add: %w", err)
	}
	defer c.gitWorktreeRemove(ctx, judgeDir)

	fusion, err := c.runJudge(ctx, judgeDir, outputs)
	if err != nil {
		c.log.ErrorContext(ctx, "judge failed, promoting primary member to single", "err", err)
		return c.uploadMember(ctx, outputs[0], reviewer.ReviewRoleSingle)
	}

	// Upload members first to get their ids, then the fusion that links them.
	memberIDs := make([]int, 0, len(outputs))
	for _, o := range outputs {
		id, uerr := c.uploadMember(ctx, o, reviewer.ReviewRoleMember)
		if uerr != nil {
			return 0, uerr
		}
		memberIDs = append(memberIDs, id)
	}

	fusion.draft.Review.ReviewRole = reviewer.ReviewRoleFusion
	fusion.draft.Review.MemberReviewIDs = memberIDs
	id, err := c.upload.UploadAll(ctx, c.cfg.URL, c.cfg.Key, fusion.draft, fusion.mdFiles)
	if err != nil {
		return 0, fmt.Errorf("upload fusion: %w", err)
	}
	c.log.InfoContext(ctx, "fusion uploaded", "reviewId", id, "memberIds", memberIDs, "issues", len(fusion.draft.Issues))
	return id, nil
}

// produceMember reviews a single panel member in its worktree and returns the
// filled draft + R*.md paths (not yet uploaded; the worktree stays alive).
func (c *Controller) produceMember(ctx context.Context, dir, label string, m MemberSpec, prompt string) (*memberOutput, error) {
	mc := *c.cfg
	mc.Dir = dir
	mc.Runner = m.Runner
	mc.Model = m.Model
	mc.Multi = nil
	mc.Judge = nil

	rr, err := c.runnerFactory(&mc)
	if err != nil {
		return nil, fmt.Errorf("build runner: %w", err)
	}
	if err = WriteReviewSkeleton(mc.Dir, &mc); err != nil {
		return nil, fmt.Errorf("write review.json skeleton: %w", err)
	}

	result, err := rr.Run(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("run %s: %w", mc.Runner, err)
	}

	draft, err := ReadReviewJSON(mc.Dir)
	if err != nil {
		return nil, fmt.Errorf("read review: %w", err)
	}
	draft.Review.ModelInfo = result.ToModelInfo(mc.Model)
	draft.Review.ModelInfo.Runner = rr.Name()
	draft.Review.DurationMs = result.DurationMs
	draft.Review.RunnerProfile = mc.RunnerProfileSnapshot()
	c.fillMetadata(draft)

	mdFiles, err := FindMDFiles(mc.Dir)
	if err != nil {
		return nil, fmt.Errorf("find md files: %w", err)
	}
	return &memberOutput{spec: m, label: label, dir: dir, draft: draft, mdFiles: mdFiles}, nil
}

// runJudge stages each member's outputs into members/<label>/ inside the judge
// worktree, then runs the judge with the fusion prompt (one retry) and returns
// the fused draft + R*.md.
func (c *Controller) runJudge(ctx context.Context, judgeDir string, outputs []*memberOutput) (*memberOutput, error) {
	if err := stageMembers(judgeDir, outputs); err != nil {
		return nil, fmt.Errorf("stage members: %w", err)
	}

	jc := *c.cfg
	jc.Dir = judgeDir
	jc.Runner = c.cfg.Judge.Runner
	jc.Model = c.cfg.Judge.Model
	jc.Multi = nil
	jc.Judge = nil

	rr, err := c.runnerFactory(&jc)
	if err != nil {
		return nil, fmt.Errorf("build judge runner: %w", err)
	}
	prompt := SubstituteVariables(reviewer.FusionPrompt, &jc)

	var lastErr error
	for attempt := 1; attempt <= 2; attempt++ { // initial run + one retry
		// Wipe the previous attempt's root artifacts; the staged members/ subdirs
		// are untouched (CleanReviewArtifacts only looks at the dir root).
		if err = CleanReviewArtifacts(judgeDir); err != nil {
			c.log.WarnContext(ctx, "clean judge artifacts", "err", err)
		}
		if err = WriteReviewSkeleton(judgeDir, &jc); err != nil {
			return nil, fmt.Errorf("write judge skeleton: %w", err)
		}

		rctx, cancel := withTimeout(ctx, c.cfg.Timeout)
		result, err := rr.Run(rctx, prompt)
		cancel()
		if err != nil {
			lastErr = err
			c.log.WarnContext(ctx, "judge run failed", "attempt", attempt, "err", err)
			continue
		}
		draft, err := ReadReviewJSON(jc.Dir)
		if err != nil {
			lastErr = err
			c.log.WarnContext(ctx, "judge review.json invalid", "attempt", attempt, "err", err)
			continue
		}
		draft.Review.ModelInfo = result.ToModelInfo(jc.Model)
		draft.Review.ModelInfo.Runner = rr.Name()
		draft.Review.DurationMs = result.DurationMs
		draft.Review.RunnerProfile = jc.RunnerProfileSnapshot()
		c.fillMetadata(draft)

		mdFiles, err := FindMDFiles(jc.Dir)
		if err != nil {
			return nil, fmt.Errorf("find judge md files: %w", err)
		}
		return &memberOutput{label: "judge", dir: judgeDir, draft: draft, mdFiles: mdFiles}, nil
	}
	return nil, lastErr
}

// uploadMember uploads a produced output under the given review role.
func (c *Controller) uploadMember(ctx context.Context, o *memberOutput, role string) (int, error) {
	o.draft.Review.ReviewRole = role
	id, err := c.upload.UploadAll(ctx, c.cfg.URL, c.cfg.Key, o.draft, o.mdFiles)
	if err != nil {
		return 0, fmt.Errorf("upload %s: %w", o.label, err)
	}
	return id, nil
}

// stageMembers copies each member's review.json + R*.md into members/<label>/
// inside the judge worktree, where the fusion prompt expects them.
func stageMembers(judgeDir string, outputs []*memberOutput) error {
	for _, o := range outputs {
		dst := filepath.Join(judgeDir, "members", o.label)
		if err := os.MkdirAll(dst, 0o750); err != nil {
			return err
		}
		if err := copyFile(filepath.Join(o.dir, "review.json"), filepath.Join(dst, "review.json")); err != nil {
			return err
		}
		for _, p := range o.mdFiles {
			if err := copyFile(p, filepath.Join(dst, filepath.Base(p))); err != nil {
				return err
			}
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o600)
}

// sourceLabels returns the provenance label for each panel member — its model,
// or runner when no model — deduped with a -N suffix so duplicate members stay
// distinct. These name the members/<label>/ dirs the judge reads and the source
// tags it writes into issues[].sources.
func sourceLabels(members []MemberSpec) []string {
	seen := make(map[string]int, len(members))
	out := make([]string, len(members))
	for i, m := range members {
		base := m.Model
		if base == "" {
			base = m.Runner
		}
		base = sanitizeLabel(base)
		seen[base]++
		if seen[base] == 1 {
			out[i] = base
		} else {
			out[i] = fmt.Sprintf("%s-%d", base, seen[base])
		}
	}
	return out
}

// panelCommit is the commit checked out into each worktree — the MR commit when
// known, else the current HEAD of the working repo.
func (c *Controller) panelCommit() string {
	if c.cfg.Commit != "" {
		return c.cfg.Commit
	}
	return "HEAD"
}

// gitWorktreeAdd creates a detached worktree at dir checked out to commit, run
// from the working repo so git finds it.
func (c *Controller) gitWorktreeAdd(ctx context.Context, dir, commit string) error {
	cmd := exec.CommandContext(ctx, "git", "worktree", "add", "--detach", dir, commit)
	cmd.Dir = c.cfg.Dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// gitWorktreeRemove tears down a worktree. Best-effort; uses a detached context
// so cleanup still runs when the review context was cancelled.
func (c *Controller) gitWorktreeRemove(ctx context.Context, dir string) {
	cmd := exec.CommandContext(context.WithoutCancel(ctx), "git", "worktree", "remove", "--force", dir)
	cmd.Dir = c.cfg.Dir
	if out, err := cmd.CombinedOutput(); err != nil {
		c.log.WarnContext(ctx, "worktree remove", "dir", dir, "err", err, "out", strings.TrimSpace(string(out)))
	}
}

// memberLabel builds a filesystem-safe, unique label for a member worktree dir,
// e.g. "1-codex-gpt-5.5". The index keeps duplicate runner:model members distinct.
func memberLabel(i int, m MemberSpec) string {
	name := m.Runner
	if m.Model != "" {
		name += "-" + m.Model
	}
	return fmt.Sprintf("%d-%s", i+1, sanitizeLabel(name))
}

// sanitizeLabel reduces a name to a path-safe slug.
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

package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"reviewsrv/pkg/reviewer"
	"reviewsrv/pkg/reviewer/direct"
)

// directSessionLog is the transcript file written next to the run for analysis.
const directSessionLog = "direct-output.jsonl"

// ClaudeResult.Subtype values for a direct run.
const (
	directSubtypeSuccess = "success"
	directSubtypeError   = "error"
)

// RunnerDirect identifies the direct-API runner (no claude/opencode CLI).
const RunnerDirect = "direct"

// Compile-time assertion that DirectRunner satisfies ReviewRunner.
var _ ReviewRunner = (*DirectRunner)(nil)

// DirectRunner drives an LLM through a direct API with a narrow review tool set
// and maps the result onto ClaudeResult so the rest of the controller is unchanged.
type DirectRunner struct {
	Provider direct.LLMProvider
	Dir      string
	DiffBase string // target branch (CI_MERGE_REQUEST_TARGET_BRANCH_NAME)
	DiffHead string // source branch (CI_MERGE_REQUEST_SOURCE_BRANCH_NAME)
	// DiffBaseSHA is the MR diff base (CI_MERGE_REQUEST_DIFF_BASE_SHA), the
	// preferred base: branch names often don't resolve in a CI clone.
	DiffBaseSHA string
	Effort      string
	// MaxRounds overrides the loop's round budget; zero keeps the direct default.
	MaxRounds int
	// CompactAt overrides the loop's compaction threshold (estimated tokens);
	// zero keeps the direct package default. Set to direct.CompactAtLargeContext
	// for 1M-context providers so mid-run compaction (a full cache re-write)
	// doesn't fire on reviews that comfortably fit the window.
	CompactAt int
	// Tracker enables the tracker-scoped http_fetch tool (see direct.TrackerConfig).
	Tracker *direct.TrackerConfig
	Log     *slog.Logger
}

// Name implements ReviewRunner.
func (r *DirectRunner) Name() string { return RunnerDirect }

// SetSession implements ReviewRunner. The direct runner has no CLI session, so
// this is a no-op — the Step-2 retry path is unnecessary because submit_review
// always produces a filled review.json.
func (r *DirectRunner) SetSession(string) {}

// Run executes the agent loop and maps the result onto ClaudeResult.
func (r *DirectRunner) Run(ctx context.Context, prompt string) (*ClaudeResult, error) {
	// Cap the whole run like the CLI runners do (runnerTimeout), so a hung API
	// or runaway loop can't stall a CI job when the caller passed no deadline.
	ctx, cancel := context.WithTimeout(ctx, runnerTimeout)
	defer cancel()

	// CI branch names rarely resolve in a CI clone; map them onto refs that do.
	// Local runs carry no CI branch metadata; without a base the preload and the
	// git_diff defaults degrade to "working tree vs HEAD" — an empty diff on a
	// committed branch, and the model reviews the wrong thing. Detect the
	// integration branch from git instead.
	diffBase, diffHead := direct.ResolveDiffRange(ctx, r.Dir, r.DiffBaseSHA, r.DiffBase, r.DiffHead)
	if diffBase == "" {
		if diffBase = direct.DetectBaseRef(ctx, r.Dir); diffBase != "" && r.Log != nil {
			r.Log.InfoContext(ctx, "diff base not resolved from CI metadata, detected from git", "base", diffBase, "target", r.DiffBase)
		}
	}

	// Pre-load the diff and the full content of changed files into the kickoff so
	// the model reviews from them instead of fanning out one read_file per turn.
	// The pre-loaded paths seed read-dedup so the model isn't re-served them.
	preloadBlock, preloadedPaths := direct.PreloadContext(ctx, r.Dir, diffBase, diffHead)
	if r.Log != nil {
		if preloadBlock == "" {
			// Without the preload a review takes several times the rounds and cost.
			r.Log.WarnContext(ctx, "direct: diff preload is empty — the model will read every file itself",
				"base", diffBase, "head", diffHead, "target", r.DiffBase, "source", r.DiffHead, "baseSha", r.DiffBaseSHA)
		} else {
			r.Log.InfoContext(ctx, "direct: diff preloaded", "base", diffBase, "head", diffHead, "files", len(preloadedPaths), "bytes", len(preloadBlock))
		}
	}

	reg := direct.NewReviewRegistry(direct.ReviewToolsConfig{
		Dir:            r.Dir,
		DiffBase:       diffBase,
		DiffHead:       diffHead,
		PreloadedPaths: preloadedPaths,
		Tracker:        r.Tracker,
	})

	// Rebuild the AST index so ast_* tools see the current working tree. No-op if
	// ast-index isn't installed. Best-effort — a failure must not fail the review.
	if ran, err := direct.EnsureAstIndex(ctx, r.Dir); err != nil && r.Log != nil {
		r.Log.WarnContext(ctx, "ast-index rebuild failed; ast_* tools may be stale", "err", err)
	} else if ran && r.Log != nil {
		r.Log.InfoContext(ctx, "ast-index rebuilt")
	}

	opts := direct.DefaultOptions()
	opts.Effort = r.Effort
	if r.MaxRounds > 0 {
		opts.MaxRounds = r.MaxRounds
	}
	if r.CompactAt > 0 {
		opts.CompactAt = r.CompactAt
	}

	// Stream the session transcript to <dir>/direct-output.jsonl for later
	// analysis (mirrors claude-output.json / opencode-output.jsonl). Best-effort:
	// a log open failure must not fail the review.
	if closeLog := r.attachSessionLog(ctx, &opts); closeLog != nil {
		defer closeLog()
	}

	userPrompt := prompt
	if preloadBlock != "" {
		userPrompt = prompt + "\n\n" + preloadBlock
	}

	// The fetched reviewsrv prompt is the authoritative review task (project,
	// language, groups, severity, personas) — passed as the user message exactly
	// like the claude/opencode runners. SystemPrompt is only the generic
	// execution contract (tools + submit_review), not project/language specifics.
	start := time.Now()
	res, err := direct.Run(ctx, r.Provider, reg, direct.SystemPrompt, userPrompt, opts)
	elapsedMs := int(time.Since(start).Milliseconds())
	if res == nil {
		return nil, err
	}

	r.logResult(ctx, res)
	cr := directToClaudeResult(res)
	cr.DurationMs = elapsedMs
	cr.DurationAPIMs = res.DurationAPIMs // provider time only, not local tool time

	if err != nil {
		return cr, err
	}
	if !res.Submitted {
		if r.Log != nil {
			r.Log.ErrorContext(ctx, "direct: review not submitted", "stopReason", res.StopReason, "rounds", res.Rounds)
		}
		return cr, reviewer.WithRunReason(reviewer.RunReasonNotSubmitted, fmt.Errorf("direct: review not submitted (stop=%s)", res.StopReason))
	}
	return cr, nil
}

// attachSessionLog opens the transcript file and wires opts.OnEvent to it.
// Returns a close func (nil if the log could not be opened). Each event is one
// JSON line. The loop emits events from a single goroutine, so no locking is
// needed around the encoder.
func (r *DirectRunner) attachSessionLog(ctx context.Context, opts *direct.Options) func() {
	var enc *json.Encoder
	closeFn := func() {}
	path := filepath.Join(r.Dir, directSessionLog)
	if f, err := os.Create(path); err != nil {
		if r.Log != nil {
			r.Log.WarnContext(ctx, "direct: cannot open session log", "path", path, "err", err)
		}
	} else {
		enc = json.NewEncoder(f)
		closeFn = func() { _ = f.Close() }
	}
	// Write the full transcript to the file (when open) AND surface significant
	// events to the runner log, so a CI run shows live progress.
	opts.OnEvent = func(ev direct.Event) {
		if enc != nil {
			_ = enc.Encode(ev)
		}
		r.logEvent(ctx, ev)
	}
	return closeFn
}

// logEvent surfaces a significant direct-loop event to the runner log: tool
// calls and per-round timing and usage live, harness notices, and tool errors
// and provider retries at warn. The full transcript still goes to
// direct-output.jsonl.
func (r *DirectRunner) logEvent(ctx context.Context, ev direct.Event) {
	if r.Log == nil {
		return
	}
	switch ev.Kind {
	case "tool_call":
		r.Log.InfoContext(ctx, "direct tool", "round", ev.Round, "tool", ev.Tool, "args", truncate(string(ev.Args), 200))
	case "tool_result":
		// Only failures are surfaced live (a broken tracker or expired token is
		// otherwise invisible until the transcript is opened); successful results
		// stay in direct-output.jsonl to keep the CI log readable.
		if ev.IsError {
			r.Log.WarnContext(ctx, "direct tool error", "round", ev.Round, "tool", ev.Tool, "err", truncate(ev.Content, 300))
		}
	case "round":
		// A truncated round means the model burned the whole output budget and
		// its tool calls may have arrived empty — surface it live, not just in
		// the transcript.
		if direct.IsTruncated(ev.StopReason) {
			r.Log.WarnContext(ctx, "direct output truncated by token limit", "round", ev.Round, "stopReason", ev.StopReason)
		}
		if ev.Usage != nil {
			// At info: a heavy round (long thinking) otherwise leaves minutes of
			// silence in the CI log.
			r.Log.InfoContext(ctx, "direct round", "round", ev.Round,
				"duration", (time.Duration(ev.DurationMs) * time.Millisecond).Round(100*time.Millisecond),
				"inputTokens", ev.Usage.InputTokens, "outputTokens", ev.Usage.OutputTokens,
				"cacheRead", ev.Usage.CacheReadTokens, "cacheWrite", ev.Usage.CacheWriteTokens)
		}
	case "retry":
		r.Log.WarnContext(ctx, "direct provider call retried", "round", ev.Round, "err", truncate(ev.Text, 300))
	case "notice":
		r.Log.InfoContext(ctx, "direct notice", "round", ev.Round, "notice", truncate(ev.Text, 200))
	case "compact":
		r.Log.InfoContext(ctx, "direct history compacted", "round", ev.Round, "detail", ev.Text)
	}
}

func (r *DirectRunner) logResult(ctx context.Context, res *direct.Result) {
	if r.Log == nil {
		return
	}
	r.Log.InfoContext(ctx, "direct result",
		"model", res.Model,
		"rounds", res.Rounds,
		"submitted", res.Submitted,
		"stopReason", res.StopReason,
		"inputTokens", res.Usage.InputTokens,
		"outputTokens", res.Usage.OutputTokens,
		"cacheRead", res.Usage.CacheReadTokens,
		"cacheWrite", res.Usage.CacheWriteTokens,
		"cost", res.CostUsd,
	)
}

// directToClaudeResult maps a direct.Result onto the canonical ClaudeResult shape
// so ToModelInfo and downstream code stay unchanged.
func directToClaudeResult(res *direct.Result) *ClaudeResult {
	subtype := directSubtypeSuccess
	if !res.Submitted {
		subtype = directSubtypeError
	}
	return &ClaudeResult{
		Type:         claudeResultType,
		Subtype:      subtype,
		TotalCostUSD: res.CostUsd,
		NumTurns:     res.Rounds,
		StopReason:   res.StopReason,
		IsError:      !res.Submitted,
		Usage:        claudeUsage(res.Usage),
		ModelUsage:   map[string]ClaudeModelUse{res.Model: modelUse(res.Usage, res.CostUsd)},
	}
}

// claudeUsage maps direct token usage onto the ClaudeResult usage shape.
func claudeUsage(u direct.Usage) ClaudeUsage {
	return ClaudeUsage{
		InputTokens:              u.InputTokens,
		OutputTokens:             u.OutputTokens,
		CacheReadInputTokens:     u.CacheReadTokens,
		CacheCreationInputTokens: u.CacheWriteTokens,
	}
}

// modelUse maps direct token usage and its cost onto a ClaudeResult.ModelUsage entry.
func modelUse(u direct.Usage, cost float64) ClaudeModelUse {
	return ClaudeModelUse{
		InputTokens:              u.InputTokens,
		OutputTokens:             u.OutputTokens,
		CacheReadInputTokens:     u.CacheReadTokens,
		CacheCreationInputTokens: u.CacheWriteTokens,
		CostUSD:                  cost,
	}
}

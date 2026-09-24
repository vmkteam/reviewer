package runner

import (
	"bufio"
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"reviewsrv/pkg/reviewer/direct"
)

// Compile-time assertion that ExecCodexRunner satisfies ReviewRunner.
var _ ReviewRunner = (*ExecCodexRunner)(nil)

// ExecCodexRunner runs the real `codex exec` CLI subprocess.
//
// codex emits a JSONL event stream (`--json`): thread.started (session id),
// item.started/command_execution (tool calls), item.completed/agent_message
// (final text), turn.completed (token usage). codex does NOT report a dollar
// cost, so it is estimated from tokens (see toClaudeResult). Note codex
// input_tokens already include cached tokens — we split them out to match the
// rest of the pipeline (InputTokens = fresh, CacheReadInputTokens = cached).
//
// The reviewer must write review.json + R*.md into the workspace, so codex runs
// with the workspace-write sandbox (writes to the working dir, network disabled).
type ExecCodexRunner struct {
	Model string
	// Effort sets model_reasoning_effort; empty keeps the model's own default.
	// Supported levels vary by model and codex does not validate them.
	Effort string
	// Sandbox overrides the sandbox mode (default codexSandbox). Use
	// danger-full-access inside containers, where bubblewrap cannot start.
	Sandbox         string
	Dir             string
	SessionID       string // if set, resumes the thread via `exec resume <id>`
	ContinueSession bool   // codex has no auto-continue; kept for interface symmetry
	// Token is the runner-profile API key injected into the subprocess as
	// OPENAI_API_KEY when that env var is not already set (env wins).
	Token string
	// TrackerToken is injected as REVIEW_TRACKER_TOKEN (env wins; see
	// ExecClaudeRunner). The default workspace-write sandbox blocks network, so it
	// only matters with a Sandbox that allows it (danger-full-access).
	TrackerToken string
	Log          *slog.Logger

	// lastThreadID/lastUsage remember the previous run's cumulative usage so a
	// resumed run of the same thread is billed for its own tokens only.
	lastThreadID string
	lastUsage    codexUsage
}

// Name implements ReviewRunner.
func (r *ExecCodexRunner) Name() string { return RunnerCodex }

// SetSession implements ReviewRunner: resume an existing codex thread on the next Run.
func (r *ExecCodexRunner) SetSession(sessionID string) {
	r.SessionID = sessionID
	r.ContinueSession = false
}

// codexSandbox lets codex write the review outputs (review.json + R*.md) into the
// working dir while keeping the network closed.
const codexSandbox = "workspace-write"

// codexExecCmd is the codex subcommand that runs a non-interactive turn.
const codexExecCmd = "exec"

// buildArgs assembles argv for `codex exec`. With a session id it resumes the
// thread (round-2 follow-up); `exec resume` rejects --sandbox/--color, so the
// sandbox is set via `-c sandbox_mode=...` there.
func (r *ExecCodexRunner) buildArgs() []string {
	sandbox := cmp.Or(r.Sandbox, codexSandbox)
	var args []string
	if r.SessionID != "" {
		args = []string{
			codexExecCmd, "resume", r.SessionID,
			"--json",
			"--skip-git-repo-check",
			"-c", fmt.Sprintf("sandbox_mode=%q", sandbox),
		}
	} else {
		args = []string{
			codexExecCmd,
			"--json",
			"--sandbox", sandbox,
			"--skip-git-repo-check",
			"--color", "never",
		}
	}
	if r.Model != "" {
		args = append(args, "-m", r.Model)
	}
	if r.Effort != "" {
		args = append(args, "-c", fmt.Sprintf("model_reasoning_effort=%q", r.Effort))
	}
	return append(args, "-") // prompt is read from stdin
}

// Run executes `codex exec --json` and aggregates the streamed events.
func (r *ExecCodexRunner) Run(ctx context.Context, prompt string) (*ClaudeResult, error) {
	args := r.buildArgs()
	// Surface significant events (tool commands, failures) live as codex streams.
	env := append(credEnv(envOpenAIAPIKey, r.Token), credEnv(envTrackerToken, r.TrackerToken)...)
	out := runExec(ctx, r.Log, RunnerCodex, r.Dir, args, prompt, env, func(line []byte) { r.logEvent(ctx, line) })

	r.saveOutput(ctx, out.stdout.Bytes())

	if out.stdout.Len() == 0 {
		r.Log.WarnContext(ctx, "codex produced empty stdout", "stderr", truncate(out.stderr.String(), 2000))
		if out.err != nil {
			return nil, fmt.Errorf("codex exited with error: %w (stderr: %s)", out.err, truncate(out.stderr.String(), 500))
		}
		return nil, errors.New("codex produced empty output")
	}

	cr := r.result(parseCodexStream(out.stdout.Bytes()))
	// codex can report a structured failure with a zero exit code; conversely a
	// non-zero exit without a structured error is still a failure.
	if out.err != nil {
		cr.IsError = true
	}
	r.logResult(ctx, cr)

	if out.err != nil {
		r.Log.WarnContext(ctx, "codex error", "stderr", truncate(out.stderr.String(), 2000))
		return cr, fmt.Errorf("codex exited with error: %w", out.err)
	}
	return cr, nil
}

// result converts a parsed stream into this run's ClaudeResult. codex reports the
// thread's cumulative usage, so resuming the thread of the previous run (the
// Step 2 retry) subtracts that run's totals instead of billing them twice. A
// thread resumed from another process (--session) has no baseline and still
// reports the thread total.
func (r *ExecCodexRunner) result(agg *codexAggregate) *ClaudeResult {
	var base codexUsage
	if r.SessionID == r.lastThreadID {
		base = r.lastUsage
	}
	if agg.sessionID != "" {
		r.lastThreadID, r.lastUsage = agg.sessionID, agg.usage
	}
	return agg.toClaudeResult(r.Model, base)
}

// logEvent surfaces a significant codex stream event to the runner log: tool
// commands as they start, per-turn token usage, and structured failures. Invoked
// per stdout line while codex streams.
func (r *ExecCodexRunner) logEvent(ctx context.Context, line []byte) {
	line = bytes.TrimSpace(line)
	if len(line) == 0 || line[0] != '{' {
		return
	}
	var head struct {
		Type string `json:"type"`
	}
	if json.Unmarshal(line, &head) != nil {
		return
	}
	switch head.Type {
	case "item.started":
		var ev struct {
			Item struct {
				Type    string `json:"type"`
				Command string `json:"command"`
			} `json:"item"`
		}
		if json.Unmarshal(line, &ev) == nil && ev.Item.Type == "command_execution" && ev.Item.Command != "" {
			r.Log.InfoContext(ctx, "codex command", "cmd", truncate(strings.ReplaceAll(ev.Item.Command, "\n", " "), 200))
		}
	case "turn.completed":
		var ev struct {
			Usage struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
		}
		if json.Unmarshal(line, &ev) == nil {
			r.Log.InfoContext(ctx, "codex turn", "inputTokens", ev.Usage.InputTokens, "outputTokens", ev.Usage.OutputTokens)
		}
	case "error", "turn.failed":
		r.Log.WarnContext(ctx, "codex stream error", "event", truncate(string(line), 300))
	}
}

func (r *ExecCodexRunner) logResult(ctx context.Context, cr *ClaudeResult) {
	r.Log.InfoContext(ctx, "codex result parsed",
		"cost", cr.TotalCostUSD,
		"turns", cr.NumTurns,
		"inputTokens", cr.Usage.InputTokens,
		"outputTokens", cr.Usage.OutputTokens,
		"cacheRead", cr.Usage.CacheReadInputTokens,
		"sessionId", cr.SessionID,
		"isError", cr.IsError,
	)
}

func (r *ExecCodexRunner) saveOutput(ctx context.Context, data []byte) {
	if len(data) == 0 {
		return
	}
	path := filepath.Join(r.Dir, "codex-output.jsonl")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		r.Log.WarnContext(ctx, "failed to save codex output", "err", err)
	}
}

// codexUsage is codex token usage as reported by turn.completed: the thread's
// cumulative total_token_usage, so after `exec resume` it includes every earlier
// run of the thread. input includes the cached and cacheWrite tokens.
type codexUsage struct {
	input, cached, cacheWrite, output int
}

// since returns the usage accrued after base (a previous total of the thread).
func (u codexUsage) since(base codexUsage) codexUsage {
	return codexUsage{
		input:      max(u.input-base.input, 0),
		cached:     max(u.cached-base.cached, 0),
		cacheWrite: max(u.cacheWrite-base.cacheWrite, 0),
		output:     max(u.output-base.output, 0),
	}
}

// codexAggregate accumulates state while scanning the codex `--json` stream.
type codexAggregate struct {
	text      strings.Builder
	sessionID string
	turns     int
	usage     codexUsage
	isError   bool
	errMsg    string
}

// applyLine folds one JSONL event into the aggregate.
func (a *codexAggregate) applyLine(line []byte) {
	var head struct {
		Type string `json:"type"`
	}
	if json.Unmarshal(line, &head) != nil {
		return
	}
	switch head.Type {
	case "thread.started":
		var ev struct {
			ThreadID string `json:"thread_id"`
		}
		if json.Unmarshal(line, &ev) == nil && ev.ThreadID != "" && a.sessionID == "" {
			a.sessionID = ev.ThreadID
		}
	case "item.completed":
		var ev struct {
			Item struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"item"`
		}
		if json.Unmarshal(line, &ev) == nil && ev.Item.Type == "agent_message" {
			if t := strings.TrimSpace(ev.Item.Text); t != "" {
				a.text.WriteString(t)
			}
		}
	case "turn.completed":
		var ev struct {
			Usage struct {
				InputTokens           int `json:"input_tokens"`
				CachedInputTokens     int `json:"cached_input_tokens"`
				CacheWriteInputTokens int `json:"cache_write_input_tokens"`
				OutputTokens          int `json:"output_tokens"`
			} `json:"usage"`
		}
		if json.Unmarshal(line, &ev) == nil {
			a.turns++
			// Cumulative for the thread, so the last event wins.
			a.usage = codexUsage{
				input:      ev.Usage.InputTokens,
				cached:     ev.Usage.CachedInputTokens,
				cacheWrite: ev.Usage.CacheWriteInputTokens,
				output:     ev.Usage.OutputTokens,
			}
			// codex also emits transient `error` events ("Reconnecting... 2/5")
			// before a successful retry; a completed turn means it recovered.
			a.isError = false
			a.errMsg = ""
		}
	case "error":
		var ev struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(line, &ev) == nil {
			a.isError = true
			a.errMsg = strings.TrimSpace(ev.Message)
		}
	case "turn.failed":
		var ev struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(line, &ev) == nil {
			a.isError = true
			a.errMsg = strings.TrimSpace(ev.Error.Message)
		}
	}
}

// toClaudeResult bills the usage accrued since base (zero for a fresh thread).
func (a *codexAggregate) toClaudeResult(fallbackModel string, base codexUsage) *ClaudeResult {
	d := a.usage.since(base)
	// codex input_tokens include cache reads and writes; codex reports no cost,
	// so it is estimated from the shared price table (unknown models cost 0).
	u := direct.SplitInput(d.input, d.cached, d.cacheWrite, d.output)
	cost := direct.PricingFor(strings.TrimSpace(fallbackModel), time.Now()).Cost(u)

	stop, subtype := "end_turn", directSubtypeSuccess
	if a.isError {
		stop, subtype = directSubtypeError, directSubtypeError
	}

	cr := &ClaudeResult{
		Type:         claudeResultType,
		Subtype:      subtype,
		Result:       a.text.String(),
		TotalCostUSD: cost,
		NumTurns:     a.turns,
		SessionID:    a.sessionID,
		IsError:      a.isError,
		StopReason:   stop,
		Usage:        claudeUsage(u),
	}
	if a.errMsg != "" && cr.Result == "" {
		cr.Result = a.errMsg
	}
	if fallbackModel != "" {
		cr.ModelUsage = map[string]ClaudeModelUse{fallbackModel: modelUse(u, cost)}
	}
	return cr
}

// parseCodexStream folds the codex `--json` event stream into an aggregate.
func parseCodexStream(data []byte) *codexAggregate {
	var agg codexAggregate
	sc := bufio.NewScanner(bytes.NewReader(data))
	// Events can be large (long text chunks). Bump the buffer to 4 MiB to be safe.
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 || line[0] != '{' {
			continue
		}
		agg.applyLine(line)
	}
	return &agg
}

package ctl

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// Compile-time assertion that ExecCodexRunner satisfies ReviewRunner.
var _ ReviewRunner = (*ExecCodexRunner)(nil)

// ExecCodexRunner runs the real `codex exec` CLI subprocess.
//
// codex emits a JSONL event stream (`--json`): thread.started (session id),
// item.started/command_execution (tool calls), item.completed/agent_message
// (final text), turn.completed (token usage). codex does NOT report a dollar
// cost, so it is estimated from tokens (see codexEstimateCostUSD). Note codex
// input_tokens already include cached tokens — we split them out to match the
// rest of the pipeline (InputTokens = fresh, CacheReadInputTokens = cached).
//
// The reviewer must write review.json + R*.md into the workspace, so codex runs
// with the workspace-write sandbox (writes to the working dir, network disabled).
type ExecCodexRunner struct {
	Model           string
	Dir             string
	SessionID       string // if set, resumes the thread via `exec resume <id>`
	ContinueSession bool   // codex has no auto-continue; kept for interface symmetry
	Log             *slog.Logger
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

// buildArgs assembles argv for `codex exec`. With a session id it resumes the
// thread (round-2 follow-up); `exec resume` rejects --sandbox/--color, so the
// sandbox is set via `-c sandbox_mode=...` there.
func (r *ExecCodexRunner) buildArgs() []string {
	var args []string
	if r.SessionID != "" {
		args = []string{
			"exec", "resume", r.SessionID,
			"--json",
			"--skip-git-repo-check",
			"-c", fmt.Sprintf("sandbox_mode=%q", codexSandbox),
		}
	} else {
		args = []string{
			"exec",
			"--json",
			"--sandbox", codexSandbox,
			"--skip-git-repo-check",
			"--color", "never",
		}
	}
	if r.Model != "" {
		args = append(args, "-m", r.Model)
	}
	return append(args, "-") // prompt is read from stdin
}

// Run executes `codex exec --json` and aggregates the streamed events.
func (r *ExecCodexRunner) Run(ctx context.Context, prompt string) (*ClaudeResult, error) {
	args := r.buildArgs()
	// Surface significant events (tool commands, failures) live as codex streams.
	out := runExec(ctx, r.Log, RunnerCodex, r.Dir, args, prompt, func(line []byte) { r.logEvent(ctx, line) })

	r.saveOutput(ctx, out.stdout.Bytes())

	if out.stdout.Len() == 0 {
		r.Log.WarnContext(ctx, "codex produced empty stdout", "stderr", truncate(out.stderr.String(), 2000))
		if out.err != nil {
			return nil, fmt.Errorf("codex exited with error: %w (stderr: %s)", out.err, truncate(out.stderr.String(), 500))
		}
		return nil, errors.New("codex produced empty output")
	}

	cr := ParseCodexResult(out.stdout.Bytes(), r.Model)
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

// codexAggregate accumulates state while scanning the codex `--json` stream.
type codexAggregate struct {
	text       strings.Builder
	sessionID  string
	turns      int
	inputTotal int // includes cached
	cached     int
	output     int
	isError    bool
	errMsg     string
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
				InputTokens       int `json:"input_tokens"`
				CachedInputTokens int `json:"cached_input_tokens"`
				OutputTokens      int `json:"output_tokens"`
			} `json:"usage"`
		}
		if json.Unmarshal(line, &ev) == nil {
			a.turns++
			a.inputTotal += ev.Usage.InputTokens
			a.cached += ev.Usage.CachedInputTokens
			a.output += ev.Usage.OutputTokens
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

func (a *codexAggregate) toClaudeResult(fallbackModel string) *ClaudeResult {
	// codex input_tokens include cached; split so InputTokens is the fresh count.
	cached := a.cached
	if cached > a.inputTotal {
		cached = a.inputTotal
	}
	freshInput := a.inputTotal - cached
	cost := codexEstimateCostUSD(fallbackModel, freshInput, a.output, cached)

	stop, subtype := "end_turn", "success"
	if a.isError {
		stop, subtype = "error", "error"
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
		Usage: ClaudeUsage{
			InputTokens:          freshInput,
			OutputTokens:         a.output,
			CacheReadInputTokens: cached,
		},
	}
	if a.errMsg != "" && cr.Result == "" {
		cr.Result = a.errMsg
	}
	if fallbackModel != "" {
		cr.ModelUsage = map[string]ClaudeModelUse{
			fallbackModel: {
				InputTokens:          freshInput,
				OutputTokens:         a.output,
				CacheReadInputTokens: cached,
				CostUSD:              cost,
			},
		}
	}
	return cr
}

// ParseCodexResult aggregates the codex `--json` event stream into a ClaudeResult.
// The fallback model names the run for cost estimation (codex reports no cost).
func ParseCodexResult(data []byte, fallbackModel string) *ClaudeResult {
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
	return agg.toClaudeResult(fallbackModel)
}

// codexTokenPrice is the USD-per-MTok rate for a codex/OpenAI model. cached input
// is a subset of input (the caller passes fresh and cached separately).
type codexTokenPrice struct {
	input, cachedInput, output float64
}

// codexModelPrices mirrors published OpenAI/codex token rates
// (USD per 1M tokens). Unknown models estimate to 0 (cost reported as 0).
var codexModelPrices = map[string]codexTokenPrice{
	"codex-default":     {1.75, 0.175, 14.00},
	"gpt-5.3-codex":     {1.75, 0.175, 14.00},
	"gpt-5.2-codex":     {1.75, 0.175, 14.00},
	"gpt-5.1-codex-max": {1.25, 0.125, 10.00},
	"gpt-5.1-codex":     {1.25, 0.125, 10.00},
	"gpt-5-codex":       {1.25, 0.125, 10.00},
	"gpt-5.5":           {5.00, 0.50, 30.00},
	"gpt-5.5-pro":       {30.00, 30.00, 180.00},
	"gpt-5.4":           {2.50, 0.25, 15.00},
	"gpt-5.4-mini":      {0.75, 0.075, 4.50},
	"gpt-5.4-nano":      {0.20, 0.02, 1.25},
	"gpt-5.4-pro":       {30.00, 30.00, 180.00},
}

// codexEstimateCostUSD estimates a codex run's cost from tokens. freshInput must
// exclude cached tokens (cached is billed at the lower cachedInput rate).
func codexEstimateCostUSD(model string, freshInput, output, cached int) float64 {
	p, ok := codexModelPrices[strings.TrimSpace(model)]
	if !ok {
		return 0
	}
	return (float64(freshInput)*p.input +
		float64(cached)*p.cachedInput +
		float64(output)*p.output) / 1_000_000
}

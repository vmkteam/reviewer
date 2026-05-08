package ctl

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"reviewsrv/pkg/db"
)

const claudeResultType = "result"

// runnerTimeout caps how long a single runner subprocess may take.
// Guards against a hung CLI (claude/opencode) stalling a CI job indefinitely
// when the caller passed a context without a deadline.
const runnerTimeout = 30 * time.Minute

// ClaudeResult represents the JSON output from claude --output-format json.
type ClaudeResult struct {
	Type              string                    `json:"type"`
	Subtype           string                    `json:"subtype"`
	Result            string                    `json:"result"`
	TotalCostUSD      float64                   `json:"total_cost_usd"`
	DurationMs        int                       `json:"duration_ms"`
	DurationAPIMs     int                       `json:"duration_api_ms"`
	NumTurns          int                       `json:"num_turns"`
	SessionID         string                    `json:"session_id"`
	IsError           bool                      `json:"is_error"`
	StopReason        string                    `json:"stop_reason"`
	TerminalReason    string                    `json:"terminal_reason"`
	PermissionDenials []any                     `json:"permission_denials"`
	Usage             ClaudeUsage               `json:"usage"`
	ModelUsage        map[string]ClaudeModelUse `json:"modelUsage"`
}

// ClaudeUsage captures aggregated token usage across all models.
type ClaudeUsage struct {
	InputTokens              int                 `json:"input_tokens"`
	CacheCreationInputTokens int                 `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int                 `json:"cache_read_input_tokens"`
	OutputTokens             int                 `json:"output_tokens"`
	ServerToolUse            ClaudeServerToolUse `json:"server_tool_use"`
	CacheCreation            ClaudeCacheCreation `json:"cache_creation"`
}

// ClaudeServerToolUse counts billable server-side tool invocations.
type ClaudeServerToolUse struct {
	WebSearchRequests int `json:"web_search_requests"`
	WebFetchRequests  int `json:"web_fetch_requests"`
}

// ClaudeCacheCreation splits cache-write tokens by TTL ($3.75 vs $1.25 per MTok on Opus).
type ClaudeCacheCreation struct {
	Ephemeral1hInputTokens int `json:"ephemeral_1h_input_tokens"`
	Ephemeral5mInputTokens int `json:"ephemeral_5m_input_tokens"`
}

// ClaudeModelUse is the per-model breakdown under "modelUsage".
type ClaudeModelUse struct {
	InputTokens              int     `json:"inputTokens"`
	OutputTokens             int     `json:"outputTokens"`
	CacheReadInputTokens     int     `json:"cacheReadInputTokens"`
	CacheCreationInputTokens int     `json:"cacheCreationInputTokens"`
	WebSearchRequests        int     `json:"webSearchRequests"`
	CostUSD                  float64 `json:"costUSD"`
	ContextWindow            int     `json:"contextWindow"`
	MaxOutputTokens          int     `json:"maxOutputTokens"`
}

// ParseClaudeResult parses the JSON output from Claude CLI.
// Supports both single JSON object and JSON array (from --resume or --verbose).
// Tolerates truncated JSON arrays by using streaming decoder.
func ParseClaudeResult(data []byte) (*ClaudeResult, error) {
	if len(data) == 0 {
		return nil, errors.New("empty claude output")
	}

	data = bytes.TrimSpace(data)

	// JSON array: [{...}, {...}, ...] — stream-decode to find the last "result" entry.
	// Uses json.Decoder to tolerate truncated arrays where the closing ] is missing.
	if data[0] == '[' {
		dec := json.NewDecoder(bytes.NewReader(data))

		// Read opening '['.
		if _, err := dec.Token(); err != nil {
			return nil, fmt.Errorf("parse claude JSON array: %w", err)
		}

		var lastResult json.RawMessage
		for dec.More() {
			var raw json.RawMessage
			if err := dec.Decode(&raw); err != nil {
				break // truncated array — use what we have
			}
			var peek struct {
				Type string `json:"type"`
			}
			if json.Unmarshal(raw, &peek) == nil && peek.Type == claudeResultType {
				lastResult = raw
			}
		}

		if lastResult == nil {
			return nil, errors.New("no result message found in claude output")
		}
		return parseResultObject(lastResult)
	}

	// Single JSON object.
	return parseResultObject(data)
}

func parseResultObject(data []byte) (*ClaudeResult, error) {
	var cr ClaudeResult
	if err := json.Unmarshal(data, &cr); err != nil {
		return nil, fmt.Errorf("parse claude result: %w", err)
	}

	if cr.Type != claudeResultType {
		return nil, fmt.Errorf("unexpected claude output type: %q", cr.Type)
	}

	if cr.Subtype == "error_max_turns" || cr.Subtype == "error" {
		return &cr, fmt.Errorf("claude returned error: %s", cr.Result)
	}

	return &cr, nil
}

// ToModelInfo converts ClaudeResult to db.ReviewModelInfo.
// The fallback model name (CLI -m flag) is replaced by the full model id
// from modelUsage when available — e.g. "opus" → "claude-opus-4-7".
// The Runner field is left empty; callers set it from the runner that produced cr.
func (cr *ClaudeResult) ToModelInfo(model string) db.ReviewModelInfo {
	mi := db.ReviewModelInfo{
		Model:        primaryModelName(cr.ModelUsage, model),
		InputTokens:  cr.Usage.InputTokens,
		OutputTokens: cr.Usage.OutputTokens,
		CostUsd:      cr.TotalCostUSD,

		CacheCreationInputTokens: cr.Usage.CacheCreationInputTokens,
		CacheReadInputTokens:     cr.Usage.CacheReadInputTokens,
		NumTurns:                 cr.NumTurns,
		SessionID:                cr.SessionID,
		DurationAPIMs:            cr.DurationAPIMs,

		DurationTotalMs:          cr.DurationMs,
		CacheCreate1hInputTokens: cr.Usage.CacheCreation.Ephemeral1hInputTokens,
		CacheCreate5mInputTokens: cr.Usage.CacheCreation.Ephemeral5mInputTokens,
		WebSearchRequests:        cr.Usage.ServerToolUse.WebSearchRequests,
		WebFetchRequests:         cr.Usage.ServerToolUse.WebFetchRequests,
		StopReason:               cr.StopReason,
		TerminalReason:           cr.TerminalReason,
		IsError:                  cr.IsError,
	}

	if len(cr.ModelUsage) > 0 {
		mi.Models = make(map[string]db.ModelUseStats, len(cr.ModelUsage))
		for name, u := range cr.ModelUsage {
			mi.Models[name] = db.ModelUseStats{
				InputTokens:              u.InputTokens,
				OutputTokens:             u.OutputTokens,
				CacheReadInputTokens:     u.CacheReadInputTokens,
				CacheCreationInputTokens: u.CacheCreationInputTokens,
				CostUsd:                  u.CostUSD,
			}
		}
	}

	return mi
}

// primaryModelName returns the full model id of the most expensive run
// (typically the Opus pass vs. a Haiku compaction side-run). Falls back
// to the CLI alias when modelUsage is missing.
func primaryModelName(modelUsage map[string]ClaudeModelUse, fallback string) string {
	var (
		best     string
		bestCost = -1.0
	)
	for name, u := range modelUsage {
		if u.CostUSD > bestCost {
			best = name
			bestCost = u.CostUSD
		}
	}
	if best == "" {
		return fallback
	}
	return best
}

// ReviewRunner abstracts the review LLM subprocess for testability.
// Name returns a stable runner identifier (RunnerClaude | RunnerOpenCode) that
// gets stored alongside model usage in db.ReviewModelInfo.
//
// Implementations normalize their CLI output into ClaudeResult — the name
// stays for backwards compatibility with persisted records.
type ReviewRunner interface {
	Run(ctx context.Context, prompt string) (*ClaudeResult, error)
	Name() string
	// SetSession switches the runner to resume an existing CLI session on the
	// next Run, reusing the prompt cache. Used by the auto-retry path to send
	// a small Step 2 follow-up without re-billing the full original prompt.
	//
	// Security: today the only caller (Controller.runStep2Recovery) feeds in a
	// sessionID returned by the previous Run — i.e. runner-supplied, never
	// user-supplied. If a future caller exposes sessionID through a CLI flag
	// or HTTP parameter, validate against the local CLI's session-id format
	// (claude: alphanumeric+dash; opencode: `ses_` prefix + base32) before
	// passing it here, or risk argv-injection through e.g. `; rm -rf /`.
	SetSession(sessionID string)
}

// Compile-time assertions that both runners satisfy ReviewRunner.
var (
	_ ReviewRunner = (*ExecClaudeRunner)(nil)
	_ ReviewRunner = (*ExecOpenCodeRunner)(nil)
)

// ExecClaudeRunner runs the real claude CLI subprocess.
type ExecClaudeRunner struct {
	Model           string
	Dir             string
	SessionID       string // if set, uses --resume to reuse prompt cache
	ContinueSession bool   // if true, uses --continue to resume last session
	Log             *slog.Logger
}

// Name implements ReviewRunner.
func (r *ExecClaudeRunner) Name() string { return RunnerClaude }

// SetSession implements ReviewRunner. Sets SessionID for --resume and clears
// ContinueSession so the explicit ID wins over auto-continue.
func (r *ExecClaudeRunner) SetSession(sessionID string) {
	r.SessionID = sessionID
	r.ContinueSession = false
}

func (r *ExecClaudeRunner) buildArgs() []string {
	args := []string{
		"--print",
		"--output-format", "json",
		"--permission-mode", "bypassPermissions",
	}

	if r.Model != "" {
		args = append(args, "--model", r.Model)
	}

	if r.ContinueSession {
		args = append(args, "--continue")
	} else if r.SessionID != "" {
		args = append(args, "--resume", r.SessionID)
	}

	args = append(args, "-p", "-") // read prompt from stdin
	return args
}

// Run executes claude --print --output-format json and parses the result.
func (r *ExecClaudeRunner) Run(ctx context.Context, prompt string) (*ClaudeResult, error) {
	args := r.buildArgs()
	out := runExec(ctx, r.Log, RunnerClaude, r.Dir, args, prompt)

	r.saveOutput(ctx, out.stdout.Bytes())

	if out.err != nil {
		r.Log.WarnContext(ctx, "claude error",
			"stderr", truncate(out.stderr.String(), 2000),
			"stdout", truncate(out.stdout.String(), 2000),
		)
		return r.handleClaudeError(out.err, out.stdout.Bytes(), out.stderr.String())
	}

	if out.stdout.Len() == 0 {
		r.Log.WarnContext(ctx, "claude produced empty stdout", "stderr", truncate(out.stderr.String(), 2000))
		return nil, errors.New("claude produced empty output")
	}

	cr, parseErr := ParseClaudeResult(out.stdout.Bytes())
	if parseErr != nil {
		r.Log.WarnContext(ctx, "failed to parse claude output",
			"err", parseErr,
			"stdoutPreview", truncate(out.stdout.String(), 500),
			"stderr", truncate(out.stderr.String(), 500),
		)
		return nil, parseErr
	}

	r.logResult(ctx, cr)

	return cr, nil
}

func (r *ExecClaudeRunner) logResult(ctx context.Context, cr *ClaudeResult) {
	r.Log.InfoContext(ctx, "claude result parsed",
		"cost", cr.TotalCostUSD,
		"turns", cr.NumTurns,
		"inputTokens", cr.Usage.InputTokens,
		"outputTokens", cr.Usage.OutputTokens,
		"cacheRead", cr.Usage.CacheReadInputTokens,
		"cacheCreate1h", cr.Usage.CacheCreation.Ephemeral1hInputTokens,
		"cacheCreate5m", cr.Usage.CacheCreation.Ephemeral5mInputTokens,
		"webFetch", cr.Usage.ServerToolUse.WebFetchRequests,
		"webSearch", cr.Usage.ServerToolUse.WebSearchRequests,
		"models", len(cr.ModelUsage),
		"stopReason", cr.StopReason,
	)

	if cr.IsError {
		r.Log.ErrorContext(ctx, "claude run reported is_error",
			"stopReason", cr.StopReason,
			"terminalReason", cr.TerminalReason,
			"sessionId", cr.SessionID,
			"subtype", cr.Subtype,
		)
	}

	if len(cr.PermissionDenials) > 0 {
		r.Log.WarnContext(ctx, "claude permission denials",
			"count", len(cr.PermissionDenials),
			"sessionId", cr.SessionID,
		)
	}
}

func (r *ExecClaudeRunner) saveOutput(ctx context.Context, data []byte) {
	if len(data) == 0 {
		return
	}
	path := filepath.Join(r.Dir, "claude-output.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		r.Log.WarnContext(ctx, "failed to save claude output", "err", err)
	}
}

func (r *ExecClaudeRunner) handleClaudeError(err error, stdout []byte, stderr string) (*ClaudeResult, error) {
	if len(stdout) > 0 {
		if cr, parseErr := ParseClaudeResult(stdout); parseErr == nil {
			return cr, fmt.Errorf("claude exited with error: %w", err)
		}
	}
	return nil, fmt.Errorf("claude exited with error: %w (stderr: %s)", err, truncate(stderr, 500))
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

type runOutput struct {
	stdout bytes.Buffer
	stderr bytes.Buffer
	err    error
}

// runExec spawns the runner CLI under the shared runnerTimeout and captures
// stdout/stderr. Centralising I/O wiring keeps the per-runner Run() bodies
// focused on argv and result parsing.
func runExec(ctx context.Context, log *slog.Logger, binary, dir string, args []string, prompt string) *runOutput {
	ctx, cancel := context.WithTimeout(ctx, runnerTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(prompt)

	out := &runOutput{}
	cmd.Stdout = &out.stdout
	cmd.Stderr = &out.stderr

	log.InfoContext(ctx, "running "+binary, "dir", dir, "promptLen", len(prompt), "args", args)

	out.err = cmd.Run()

	log.InfoContext(ctx, binary+" finished", "exitErr", out.err, "stdoutLen", out.stdout.Len(), "stderrLen", out.stderr.Len())

	if log.Enabled(ctx, slog.LevelDebug) {
		if out.stderr.Len() > 0 {
			log.DebugContext(ctx, binary+" stderr", "stderr", truncate(out.stderr.String(), 2000))
		}
		if out.stdout.Len() > 0 {
			log.DebugContext(ctx, binary+" stdout", "stdout", truncate(out.stdout.String(), 2000))
		}
	}

	return out
}

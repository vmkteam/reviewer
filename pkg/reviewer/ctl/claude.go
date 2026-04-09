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

	"reviewsrv/pkg/db"
)

const claudeResultType = "result"

// ClaudeResult represents the JSON output from claude --output-format json.
type ClaudeResult struct {
	Type          string  `json:"type"`
	Subtype       string  `json:"subtype"`
	Result        string  `json:"result"`
	TotalCostUSD  float64 `json:"total_cost_usd"`
	DurationMs    int     `json:"duration_ms"`
	DurationAPIMs int     `json:"duration_api_ms"`
	NumTurns      int     `json:"num_turns"`
	SessionID     string  `json:"session_id"`
	Usage         struct {
		InputTokens              int `json:"input_tokens"`
		CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
		CacheReadInputTokens     int `json:"cache_read_input_tokens"`
		OutputTokens             int `json:"output_tokens"`
	} `json:"usage"`
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
func (cr *ClaudeResult) ToModelInfo(model string) db.ReviewModelInfo {
	return db.ReviewModelInfo{
		Model:        model,
		InputTokens:  cr.Usage.InputTokens,
		OutputTokens: cr.Usage.OutputTokens,
		CostUsd:      cr.TotalCostUSD,

		CacheCreationInputTokens: cr.Usage.CacheCreationInputTokens,
		CacheReadInputTokens:     cr.Usage.CacheReadInputTokens,
		NumTurns:                 cr.NumTurns,
		SessionID:                cr.SessionID,
		DurationAPIMs:            cr.DurationAPIMs,
	}
}

// ClaudeRunner abstracts the Claude CLI subprocess for testability.
type ClaudeRunner interface {
	Run(ctx context.Context, prompt string) (*ClaudeResult, error)
}

// ExecClaudeRunner runs the real claude CLI subprocess.
type ExecClaudeRunner struct {
	Model           string
	Dir             string
	SessionID       string // if set, uses --resume to reuse prompt cache
	ContinueSession bool   // if true, uses --continue to resume last session
	Log             *slog.Logger
}

// Run executes claude --print --output-format json and parses the result.
func (r *ExecClaudeRunner) Run(ctx context.Context, prompt string) (*ClaudeResult, error) {
	args := []string{
		"--print",
		"--output-format", "json",
		"--model", r.Model,
		"--permission-mode", "bypassPermissions",
	}

	if r.ContinueSession {
		args = append(args, "--continue")
	} else if r.SessionID != "" {
		args = append(args, "--resume", r.SessionID)
	}

	args = append(args, "-p", "-") // read prompt from stdin

	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Dir = r.Dir
	cmd.Stdin = strings.NewReader(prompt)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	r.Log.InfoContext(ctx, "running claude",
		"model", r.Model,
		"dir", r.Dir,
		"promptLen", len(prompt),
		"args", args,
	)

	err := cmd.Run()

	// Always log output sizes for diagnostics.
	r.Log.InfoContext(ctx, "claude finished",
		"exitErr", err,
		"stdoutLen", stdout.Len(),
		"stderrLen", stderr.Len(),
	)

	if r.Log.Enabled(ctx, slog.LevelDebug) {
		if stderr.Len() > 0 {
			r.Log.DebugContext(ctx, "claude stderr", "stderr", truncate(stderr.String(), 2000))
		}
		if stdout.Len() > 0 {
			r.Log.DebugContext(ctx, "claude stdout", "stdout", truncate(stdout.String(), 2000))
		}
	}

	// Save raw output for diagnostics.
	r.saveOutput(stdout.Bytes())

	if err != nil {
		r.Log.WarnContext(ctx, "claude error",
			"stderr", truncate(stderr.String(), 2000),
			"stdout", truncate(stdout.String(), 2000),
		)
		return r.handleClaudeError(err, stdout.Bytes(), stderr.String())
	}

	if stdout.Len() == 0 {
		r.Log.WarnContext(ctx, "claude produced empty stdout", "stderr", truncate(stderr.String(), 2000))
		return nil, errors.New("claude produced empty output")
	}

	cr, parseErr := ParseClaudeResult(stdout.Bytes())
	if parseErr != nil {
		r.Log.WarnContext(ctx, "failed to parse claude output",
			"err", parseErr,
			"stdoutPreview", truncate(stdout.String(), 500),
			"stderr", truncate(stderr.String(), 500),
		)
		return nil, parseErr
	}

	r.Log.InfoContext(ctx, "claude result parsed",
		"cost", cr.TotalCostUSD,
		"turns", cr.NumTurns,
		"inputTokens", cr.Usage.InputTokens,
		"outputTokens", cr.Usage.OutputTokens,
		"cacheRead", cr.Usage.CacheReadInputTokens,
	)

	return cr, nil
}

func (r *ExecClaudeRunner) saveOutput(data []byte) {
	if len(data) == 0 {
		return
	}
	path := filepath.Join(r.Dir, "claude-output.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		r.Log.WarnContext(context.Background(), "failed to save claude output", "err", err)
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

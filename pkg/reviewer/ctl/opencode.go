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
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// exportTimeout caps `opencode export` — it should return in seconds,
// far below the overall runner budget.
const exportTimeout = 60 * time.Second

// ExecOpenCodeRunner runs the real opencode CLI subprocess.
// Implements ReviewRunner so the rest of the controller stays runner-agnostic.
//
// opencode emits a stream of NDJSON events (step_start / text / step_finish / tool).
// This runner aggregates them into the same ClaudeResult shape that downstream
// code (ToModelInfo, HTML renderer) already understands.
type ExecOpenCodeRunner struct {
	Model           string
	Dir             string
	SessionID       string // if set, uses -s to continue a specific session
	ContinueSession bool   // if true, uses -c to continue the last session
	// AllowDangerousPermissions toggles `--dangerously-skip-permissions`, which
	// disables interactive permission prompts. Required for unattended CI runs
	// but should stay off when the reviewer config trusts the working tree less.
	AllowDangerousPermissions bool
	Log                       *slog.Logger
}

// Name implements ReviewRunner.
func (r *ExecOpenCodeRunner) Name() string { return RunnerOpenCode }

// SetSession implements ReviewRunner. Sets SessionID for `-s <id>` and clears
// ContinueSession so the explicit ID wins over auto-continue.
func (r *ExecOpenCodeRunner) SetSession(sessionID string) {
	r.SessionID = sessionID
	r.ContinueSession = false
}

func (r *ExecOpenCodeRunner) buildArgs() []string {
	args := []string{
		"run",
		"--format", "json",
	}

	if r.AllowDangerousPermissions {
		args = append(args, "--dangerously-skip-permissions")
	}

	if r.Model != "" {
		args = append(args, "-m", r.Model)
	}

	if r.ContinueSession {
		args = append(args, "-c")
	} else if r.SessionID != "" {
		args = append(args, "-s", r.SessionID)
	}

	return args
}

// Run executes `opencode run --format json` and parses the streamed events.
func (r *ExecOpenCodeRunner) Run(ctx context.Context, prompt string) (*ClaudeResult, error) {
	args := r.buildArgs()
	out := runExec(ctx, r.Log, RunnerOpenCode, r.Dir, args, prompt)

	r.saveOutput(ctx, out.stdout.Bytes())

	if out.err != nil {
		r.Log.WarnContext(ctx, "opencode error",
			"stderr", truncate(out.stderr.String(), 2000),
			"stdout", truncate(out.stdout.String(), 2000),
		)
		// Try to parse whatever arrived before the error — matches Claude runner behaviour.
		if out.stdout.Len() > 0 {
			if cr, parseErr := ParseOpenCodeResult(out.stdout.Bytes(), r.Model); parseErr == nil {
				return cr, fmt.Errorf("opencode exited with error: %w", out.err)
			}
		}
		return nil, fmt.Errorf("opencode exited with error: %w (stderr: %s)", out.err, truncate(out.stderr.String(), 500))
	}

	if out.stdout.Len() == 0 {
		r.Log.WarnContext(ctx, "opencode produced empty stdout", "stderr", truncate(out.stderr.String(), 2000))
		return nil, errors.New("opencode produced empty output")
	}

	cr, parseErr := ParseOpenCodeResult(out.stdout.Bytes(), r.Model)
	if parseErr != nil {
		r.Log.WarnContext(ctx, "failed to parse opencode output",
			"err", parseErr,
			"stdoutPreview", truncate(out.stdout.String(), 500),
			"stderr", truncate(out.stderr.String(), 500),
		)
		return nil, parseErr
	}

	r.resolveSessionModel(ctx, cr)
	r.logResult(ctx, cr)

	return cr, nil
}

// resolveSessionModel enriches cr.ModelUsage when streaming events did not expose
// the model — opencode v1.4.x omits it. Falls back to `opencode export <sessionID>`,
// which reliably returns messages[*].info.model.
func (r *ExecOpenCodeRunner) resolveSessionModel(ctx context.Context, cr *ClaudeResult) {
	if len(cr.ModelUsage) > 0 || cr.SessionID == "" {
		return
	}
	name := fetchOpenCodeSessionModel(ctx, cr.SessionID)
	if name == "" {
		r.Log.WarnContext(ctx, "opencode session model not resolved", "sessionId", cr.SessionID)
		return
	}
	cr.ModelUsage = map[string]ClaudeModelUse{
		name: {
			InputTokens:              cr.Usage.InputTokens,
			OutputTokens:             cr.Usage.OutputTokens,
			CacheReadInputTokens:     cr.Usage.CacheReadInputTokens,
			CacheCreationInputTokens: cr.Usage.CacheCreationInputTokens,
			CostUSD:                  cr.TotalCostUSD,
		},
	}
	r.Log.InfoContext(ctx, "resolved opencode model via session export", "model", name)
}

// fetchOpenCodeSessionModel queries `opencode export <sessionID>` to extract
// the model used in the session. Returns "" on any failure — caller must
// treat the result as optional (it's a best-effort enrichment).
func fetchOpenCodeSessionModel(ctx context.Context, sessionID string) string {
	if sessionID == "" {
		return ""
	}
	exportCtx, cancel := context.WithTimeout(ctx, exportTimeout)
	defer cancel()
	// --sanitize redacts transcript/file bodies. Without it the output can exceed
	// opencode's internal 128 KiB cap and arrive truncated, breaking json.Unmarshal.
	// We only need messages[*].info.model, which --sanitize preserves.
	cmd := exec.CommandContext(exportCtx, "opencode", "export", "--sanitize", sessionID)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	// `opencode export` prints "Exporting session: <id>\n" before the JSON body.
	if idx := bytes.IndexByte(out, '{'); idx > 0 {
		out = out[idx:]
	}
	var exp struct {
		Messages []struct {
			Info struct {
				Model struct {
					ProviderID string `json:"providerID"`
					ModelID    string `json:"modelID"`
				} `json:"model"`
			} `json:"info"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(out, &exp); err != nil {
		return ""
	}
	for _, m := range exp.Messages {
		if m.Info.Model.ModelID == "" {
			continue
		}
		if m.Info.Model.ProviderID != "" {
			return m.Info.Model.ProviderID + "/" + m.Info.Model.ModelID
		}
		return m.Info.Model.ModelID
	}
	return ""
}

func (r *ExecOpenCodeRunner) logResult(ctx context.Context, cr *ClaudeResult) {
	r.Log.InfoContext(ctx, "opencode result parsed",
		"cost", cr.TotalCostUSD,
		"turns", cr.NumTurns,
		"inputTokens", cr.Usage.InputTokens,
		"outputTokens", cr.Usage.OutputTokens,
		"cacheRead", cr.Usage.CacheReadInputTokens,
		"cacheCreate5m", cr.Usage.CacheCreation.Ephemeral5mInputTokens,
		"sessionId", cr.SessionID,
		"stopReason", cr.StopReason,
	)
}

func (r *ExecOpenCodeRunner) saveOutput(ctx context.Context, data []byte) {
	if len(data) == 0 {
		return
	}
	path := filepath.Join(r.Dir, "opencode-output.jsonl")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		r.Log.WarnContext(ctx, "failed to save opencode output", "err", err)
	}
}

// opencodeEvent is the common envelope for NDJSON events emitted by `opencode run --format json`.
type opencodeEvent struct {
	Type      string          `json:"type"`
	Timestamp int64           `json:"timestamp"`
	SessionID string          `json:"sessionID"`
	Part      json.RawMessage `json:"part"`
}

type opencodeTextPart struct {
	Text string `json:"text"`
}

type opencodeStepFinishPart struct {
	Reason     string         `json:"reason"`
	Tokens     opencodeTokens `json:"tokens"`
	Cost       float64        `json:"cost"`
	ModelID    string         `json:"modelID"`
	ProviderID string         `json:"providerID"`
}

type opencodeTokens struct {
	Input  int                 `json:"input"`
	Output int                 `json:"output"`
	Cache  opencodeCacheTokens `json:"cache"`
}

type opencodeCacheTokens struct {
	Read  int `json:"read"`
	Write int `json:"write"`
}

// opencodeAggregate accumulates state while scanning the NDJSON event stream.
type opencodeAggregate struct {
	text          strings.Builder
	sessionID     string
	firstTS       int64
	lastTS        int64
	turns         int
	stopReason    string
	modelID       string
	totalCost     float64
	inputTokens   int
	outputTokens  int
	cacheRead     int
	cacheWrite    int
	sawStepFinish bool
}

func (a *opencodeAggregate) apply(ev *opencodeEvent) {
	if ev.SessionID != "" && a.sessionID == "" {
		a.sessionID = ev.SessionID
	}
	if ev.Timestamp > 0 {
		if a.firstTS == 0 || ev.Timestamp < a.firstTS {
			a.firstTS = ev.Timestamp
		}
		if ev.Timestamp > a.lastTS {
			a.lastTS = ev.Timestamp
		}
	}

	switch ev.Type {
	case "text":
		var p opencodeTextPart
		if err := json.Unmarshal(ev.Part, &p); err == nil {
			a.text.WriteString(p.Text)
		}
	case "step_finish":
		var p opencodeStepFinishPart
		if err := json.Unmarshal(ev.Part, &p); err == nil {
			a.applyStepFinish(&p)
		}
	}
}

func (a *opencodeAggregate) applyStepFinish(p *opencodeStepFinishPart) {
	a.sawStepFinish = true
	a.turns++
	a.inputTokens += p.Tokens.Input
	a.outputTokens += p.Tokens.Output
	a.cacheRead += p.Tokens.Cache.Read
	a.cacheWrite += p.Tokens.Cache.Write
	a.totalCost += p.Cost
	if p.Reason != "" {
		a.stopReason = p.Reason
	}
	if p.ModelID != "" {
		a.modelID = p.ModelID
		if p.ProviderID != "" {
			a.modelID = p.ProviderID + "/" + p.ModelID
		}
	}
}

func (a *opencodeAggregate) toClaudeResult(fallbackModel string) *ClaudeResult {
	isError := a.stopReason != "" && a.stopReason != "stop" && a.stopReason != "end_turn"
	subtype := "success"
	if isError {
		subtype = "error"
	}

	modelName := a.modelID
	if modelName == "" {
		modelName = fallbackModel
	}

	cr := &ClaudeResult{
		Type:         claudeResultType,
		Subtype:      subtype,
		Result:       a.text.String(),
		TotalCostUSD: a.totalCost,
		NumTurns:     a.turns,
		SessionID:    a.sessionID,
		IsError:      isError,
		StopReason:   a.stopReason,
		Usage: ClaudeUsage{
			InputTokens:              a.inputTokens,
			OutputTokens:             a.outputTokens,
			CacheReadInputTokens:     a.cacheRead,
			CacheCreationInputTokens: a.cacheWrite,
			CacheCreation: ClaudeCacheCreation{
				Ephemeral5mInputTokens: a.cacheWrite,
			},
		},
	}

	if a.lastTS > a.firstTS {
		cr.DurationMs = int(a.lastTS - a.firstTS)
	}

	if modelName != "" {
		cr.ModelUsage = map[string]ClaudeModelUse{
			modelName: {
				InputTokens:              a.inputTokens,
				OutputTokens:             a.outputTokens,
				CacheReadInputTokens:     a.cacheRead,
				CacheCreationInputTokens: a.cacheWrite,
				CostUSD:                  a.totalCost,
			},
		}
	}

	return cr
}

// ParseOpenCodeResult aggregates the NDJSON event stream from `opencode run --format json`
// into a ClaudeResult. The fallback model name is used when the stream does not expose one.
func ParseOpenCodeResult(data []byte, fallbackModel string) (*ClaudeResult, error) {
	if len(data) == 0 {
		return nil, errors.New("empty opencode output")
	}

	var agg opencodeAggregate

	sc := bufio.NewScanner(bytes.NewReader(data))
	// Events can be large (long text chunks). Bump the buffer to 4 MiB to be safe.
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 || line[0] != '{' {
			continue
		}
		var ev opencodeEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			continue
		}
		agg.apply(&ev)
	}

	if err := sc.Err(); err != nil {
		// Not fatal if we already collected something — matches Claude's truncation tolerance.
		if agg.text.Len() == 0 && !agg.sawStepFinish {
			return nil, fmt.Errorf("read opencode stream: %w", err)
		}
	}

	if agg.text.Len() == 0 && !agg.sawStepFinish {
		return nil, errors.New("no text or step_finish events in opencode output")
	}

	return agg.toClaudeResult(fallbackModel), nil
}

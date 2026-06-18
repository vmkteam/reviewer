package db

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

type ReviewFileIssueStats struct {
	Critical int `json:"critical"`
	High     int `json:"high"`
	Medium   int `json:"medium"`
	Low      int `json:"low"`
	Total    int `json:"total"`
}
type ReviewModelInfo struct {
	Model        string  `json:"model"`
	Runner       string  `json:"runner,omitempty"` // "claude" | "opencode" — which CLI produced the result
	InputTokens  int     `json:"inputTokens"`
	OutputTokens int     `json:"outputTokens"`
	CostUsd      float64 `json:"costUsd"`

	CacheCreationInputTokens int    `json:"cacheCreationInputTokens,omitempty"`
	CacheReadInputTokens     int    `json:"cacheReadInputTokens,omitempty"`
	NumTurns                 int    `json:"numTurns,omitempty"`
	SessionID                string `json:"sessionId,omitempty"`
	DurationAPIMs            int    `json:"durationApiMs,omitempty"`

	// Wall-clock timing — diverges from API timing on network/ratelimit overhead.
	DurationTotalMs int `json:"durationTotalMs,omitempty"`

	// Cache-write split by TTL: 1h is ×3 the price of 5m on Opus.
	CacheCreate1hInputTokens int `json:"cacheCreate1hInputTokens,omitempty"`
	CacheCreate5mInputTokens int `json:"cacheCreate5mInputTokens,omitempty"`

	// Server-side tools (billed separately from tokens).
	WebSearchRequests int `json:"webSearchRequests,omitempty"`
	WebFetchRequests  int `json:"webFetchRequests,omitempty"`

	StopReason     string `json:"stopReason,omitempty"`
	TerminalReason string `json:"terminalReason,omitempty"`
	IsError        bool   `json:"isError,omitempty"`

	// Per-model breakdown (e.g. opus + haiku for compaction).
	Models map[string]ModelUseStats `json:"models,omitempty"`
}

// ModelUseStats — per-model tokens and cost within a single run.
type ModelUseStats struct {
	InputTokens              int     `json:"inputTokens"`
	OutputTokens             int     `json:"outputTokens"`
	CacheReadInputTokens     int     `json:"cacheReadInputTokens,omitempty"`
	CacheCreationInputTokens int     `json:"cacheCreationInputTokens,omitempty"`
	CostUsd                  float64 `json:"costUsd"`
}

// RunnerProfileParams holds runner-specific, extensible knobs for a runner
// profile (runnerProfiles.params jsonb). New runner options live here so they
// need no schema migration.
type RunnerProfileParams struct {
	// AllowDangerousPermissions maps to opencode's --dangerously-skip-permissions.
	AllowDangerousPermissions bool `json:"allowDangerousPermissions,omitempty"`
}

// ReviewRunnerProfile is the snapshot of the resolved runner profile used to
// produce a review (reviews.runnerProfile jsonb). It records exactly how the
// review was run, independent of later edits or deletion of the source profile.
// The profile token is deliberately NOT included — secrets never land here.
type ReviewRunnerProfile struct {
	RunnerProfileID int                 `json:"runnerProfileId,omitempty"`
	Title           string              `json:"title,omitempty"`
	Runner          string              `json:"runner"`
	Model           string              `json:"model,omitempty"`
	Effort          string              `json:"effort,omitempty"`
	APIProvider     string              `json:"apiProvider,omitempty"`
	APIBaseURL      string              `json:"apiBaseURL,omitempty"`
	Params          RunnerProfileParams `json:"params,omitempty"`
}

// Add accumulates numeric counters and Models map entries from o into m.
// Used by the Step 2 retry path to merge first-pass + retry billable spend
// into one record so dashboards reflect total cost. Identity-shaped fields
// (Model, Runner, SessionID, StopReason, TerminalReason, IsError) are left
// alone — they describe the primary run.
func (m *ReviewModelInfo) Add(o ReviewModelInfo) {
	m.InputTokens += o.InputTokens
	m.OutputTokens += o.OutputTokens
	m.CostUsd += o.CostUsd
	m.CacheCreationInputTokens += o.CacheCreationInputTokens
	m.CacheReadInputTokens += o.CacheReadInputTokens
	m.NumTurns += o.NumTurns
	m.DurationAPIMs += o.DurationAPIMs
	m.DurationTotalMs += o.DurationTotalMs
	m.CacheCreate1hInputTokens += o.CacheCreate1hInputTokens
	m.CacheCreate5mInputTokens += o.CacheCreate5mInputTokens
	m.WebSearchRequests += o.WebSearchRequests
	m.WebFetchRequests += o.WebFetchRequests

	if len(o.Models) == 0 {
		return
	}
	if m.Models == nil {
		m.Models = make(map[string]ModelUseStats, len(o.Models))
	}
	for name, s := range o.Models {
		cur := m.Models[name]
		cur.InputTokens += s.InputTokens
		cur.OutputTokens += s.OutputTokens
		cur.CacheReadInputTokens += s.CacheReadInputTokens
		cur.CacheCreationInputTokens += s.CacheCreationInputTokens
		cur.CostUsd += s.CostUsd
		m.Models[name] = cur
	}
}

// IssueSources is the provenance of a fused issue (issues.sources jsonb) — the
// member model labels that flagged it (e.g. ["gpt-5.5","deepseek-v4-pro"], plus
// "judge" for a verified net-new finding). Empty for single/member reviews;
// agreement count = len(sources). Stored as a JSON array.
type IssueSources []string //nolint:recvcheck // Valuer value-recv + Scanner ptr-recv: required idiom

func (s IssueSources) Value() (driver.Value, error) { return marshalJSONB(s) }

func (s *IssueSources) Scan(src any) error { return scanJSONB(src, s) }

// ProjectRunnerProfileIDs is the ordered list of additional panel members
// (projects.runnerProfileIds jsonb) — runnerProfileId values for a project's
// multi-review panel. The full panel is runnerProfileId + runnerProfileIds;
// duplicates are allowed (self-fusion). Stored as a JSON array.
type ProjectRunnerProfileIDs []int //nolint:recvcheck // Valuer value-recv + Scanner ptr-recv: required idiom

func (p ProjectRunnerProfileIDs) Value() (driver.Value, error) { return marshalJSONB(p) }

func (p *ProjectRunnerProfileIDs) Scan(src any) error { return scanJSONB(src, p) }

// marshalJSONB encodes a slice as a JSON-array string for a jsonb column. A nil
// slice becomes "[]" (the column default) instead of JSON null, so NOT NULL
// jsonb columns stay well-formed. The string is appended as a quoted literal
// that PostgreSQL casts to jsonb.
func marshalJSONB(v any) (driver.Value, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	if string(b) == "null" {
		return "[]", nil
	}
	return string(b), nil
}

// scanJSONB decodes a jsonb column (text or bytes) into dst. A NULL or empty
// value leaves dst untouched.
func scanJSONB(src, dst any) error {
	switch v := src.(type) {
	case nil:
		return nil
	case []byte:
		if len(v) == 0 {
			return nil
		}
		return json.Unmarshal(v, dst)
	case string:
		if v == "" {
			return nil
		}
		return json.Unmarshal([]byte(v), dst)
	default:
		return fmt.Errorf("db: cannot scan %T into jsonb", src)
	}
}

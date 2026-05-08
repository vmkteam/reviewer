package db

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

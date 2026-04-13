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
	InputTokens  int     `json:"inputTokens"`
	OutputTokens int     `json:"outputTokens"`
	CostUsd      float64 `json:"costUsd"`

	CacheCreationInputTokens int    `json:"cacheCreationInputTokens,omitempty"`
	CacheReadInputTokens     int    `json:"cacheReadInputTokens,omitempty"`
	NumTurns                 int    `json:"numTurns,omitempty"`
	SessionID                string `json:"sessionId,omitempty"`
	DurationAPIMs            int    `json:"durationApiMs,omitempty"`
}

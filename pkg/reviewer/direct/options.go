package direct

// Options tunes the agent loop.
type Options struct {
	// MaxRounds caps how many provider round-trips the loop may make before
	// giving up — a backstop against a model that never calls submit_review.
	MaxRounds int
	// CompactAt is the estimated-token threshold above which the middle of the
	// conversation is pruned. Zero disables compaction.
	CompactAt int
	// KeepTail is how many trailing messages compaction preserves verbatim.
	KeepTail int
	// Effort is passed to providers that support it (Anthropic output_config.effort).
	Effort string
	// OnEvent, if set, receives a transcript event per assistant turn, tool call,
	// tool result, round and final result. Used to persist the session for later
	// analysis. Called only from the loop's main goroutine.
	OnEvent Sink
}

// DefaultOptions returns sensible loop defaults for a review run. The CompactAt
// default is sized for ~128k-context models (DeepSeek); large-context providers
// should raise it via Options.CompactAt — see CompactAtLargeContext.
func DefaultOptions() Options {
	return Options{MaxRounds: 60, CompactAt: 150_000, KeepTail: 12}
}

// CompactAtLargeContext is the compaction threshold for 1M-context models
// (Claude). Compacting early is costly there: the rebuilt history invalidates
// the provider prompt cache (a full re-write at the cache-write rate) and drops
// review detail, while the big window makes it unnecessary. estimateTokens is a
// chars/4 heuristic that undershoots real tokens on code by up to ~2x, so
// 500k estimated ≈ 700-800k real — still inside the window.
const CompactAtLargeContext = 500_000

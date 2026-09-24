package direct

// Options tunes the agent loop.
type Options struct {
	// MaxRounds caps the provider round-trips with the full tool set — a
	// backstop against a model that never calls submit_review. The model is
	// warned as it nears the cap, and up to graceRounds more review-tools-only
	// rounds follow it so a review mid-delivery still lands.
	MaxRounds int
	// CompactAt is the estimated-token threshold above which the middle of the
	// conversation is pruned. Zero disables compaction.
	CompactAt int
	// KeepTail is how many trailing messages compaction preserves verbatim.
	KeepTail int
	// Effort is the reasoning effort passed to the provider (see Request.Effort).
	Effort string
	// OnEvent, if set, receives a transcript event per assistant turn, tool call,
	// tool result, round and final result. Used to persist the session for later
	// analysis. Called only from the loop's main goroutine.
	OnEvent Sink
}

// Bounds for a configured MaxRounds: below the minimum a review cannot finish,
// above the maximum the backstop no longer protects the budget.
const (
	MinMaxRounds = 10
	MaxMaxRounds = 500
)

// IsValidMaxRounds reports whether n is an acceptable MaxRounds override.
func IsValidMaxRounds(n int) bool { return n >= MinMaxRounds && n <= MaxMaxRounds }

// DefaultOptions returns sensible loop defaults for a review run. The CompactAt
// default is conservative (sized for ~128k-context backends); large-context
// providers should raise it via Options.CompactAt — see CompactAtLargeContext.
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

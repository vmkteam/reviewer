package direct

import (
	"context"
	"fmt"
	"time"
)

const (
	// maxRetries is the transient-error retry budget of a provider call.
	maxRetries = 3
	// maxRetryBackoff caps the doubling retry delay.
	maxRetryBackoff = 60 * time.Second
)

// retryPolicy is how a provider retries a failed call (see withRetry).
type retryPolicy struct {
	name      string        // error prefix
	backoff   time.Duration // first delay, doubling per attempt up to maxRetryBackoff
	transient func(error) bool
}

var (
	// openaiRetry covers the OpenAI-protocol providers: chat completions and Responses.
	openaiRetry = retryPolicy{name: "openai", backoff: 500 * time.Millisecond, transient: isOpenAITransient}
	// anthropicRetry covers failures inside the response stream too, and waits
	// longer to ride out overload/5xx bursts.
	anthropicRetry = retryPolicy{name: "anthropic", backoff: 5 * time.Second, transient: isAnthropicTransient}
)

// withRetry calls fn, retrying transient errors with exponential backoff up to
// maxRetries times; a definitive error (bad request, auth, billing) fails at
// once. onRetry, when set, hears of each retried error before the wait.
func withRetry[T any](ctx context.Context, p retryPolicy, onRetry func(error), fn func() (T, error)) (T, error) {
	backoff := p.backoff
	for attempt := 0; ; attempt++ {
		resp, err := fn()
		if err == nil {
			return resp, nil
		}
		if attempt >= maxRetries || ctx.Err() != nil || !p.transient(err) {
			return resp, fmt.Errorf("%s: %w", p.name, err)
		}
		if onRetry != nil {
			onRetry(err)
		}
		select {
		case <-ctx.Done():
			return resp, fmt.Errorf("%s: %w (retrying after: %w)", p.name, ctx.Err(), err)
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, maxRetryBackoff)
	}
}

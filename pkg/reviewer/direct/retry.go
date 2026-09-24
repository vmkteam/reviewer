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
// once. The retried errors are returned for the transcript.
func withRetry[T any](ctx context.Context, p retryPolicy, fn func() (T, error)) (T, []string, error) {
	backoff := p.backoff
	var retried []string
	for attempt := 0; ; attempt++ {
		resp, err := fn()
		if err == nil {
			return resp, retried, nil
		}
		if attempt >= maxRetries || ctx.Err() != nil || !p.transient(err) {
			return resp, retried, fmt.Errorf("%s: %w", p.name, err)
		}
		retried = append(retried, err.Error())
		select {
		case <-ctx.Done():
			return resp, retried, fmt.Errorf("%s: %w (retrying after: %w)", p.name, ctx.Err(), err)
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, maxRetryBackoff)
	}
}

package direct

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"reviewsrv/pkg/reviewer"

	"github.com/anthropics/anthropic-sdk-go"
	openai "github.com/sashabaranov/go-openai"
)

// providerReason maps a failed LLMProvider.Complete (after the provider's own
// retries) to a reviewer.RunReason*.
func providerReason(err error) string {
	var ae *anthropic.Error
	if errors.As(err, &ae) {
		return anthropicReason(ae)
	}
	var oe *openai.APIError
	if errors.As(err, &oe) {
		return openaiReason(oe)
	}
	var rf *responseFailedError
	if errors.As(err, &rf) {
		return rf.reason()
	}
	// Not an API error: a transport failure (reset stream, EOF, refused
	// connection) that outlived the retries.
	return reviewer.RunReasonAPIError
}

// anthropicReason classifies an Anthropic API error by its body type, falling
// back to the HTTP status. Errors inside the SSE stream carry the stream's 200.
func anthropicReason(e *anthropic.Error) string {
	if isCreditBalanceError(e.RawJSON()) {
		return reviewer.RunReasonBilling
	}
	switch e.Type() {
	case anthropic.ErrorTypeBillingError:
		return reviewer.RunReasonBilling
	case anthropic.ErrorTypeAuthenticationError, anthropic.ErrorTypePermissionError:
		return reviewer.RunReasonAuth
	case anthropic.ErrorTypeRateLimitError:
		return reviewer.RunReasonRateLimit
	case anthropic.ErrorTypeOverloadedError, anthropic.ErrorTypeAPIError, anthropic.ErrorTypeTimeoutError:
		return reviewer.RunReasonAPIError
	case anthropic.ErrorTypeInvalidRequestError, anthropic.ErrorTypeNotFoundError:
	}
	return reviewer.ReasonForHTTPStatus(e.StatusCode)
}

// isCreditBalanceError catches an exhausted balance reported as a plain
// invalid_request_error ("Your credit balance is too low ...").
func isCreditBalanceError(body string) bool {
	return strings.Contains(strings.ToLower(body), "credit balance")
}

// openaiReason classifies an OpenAI-protocol API error (OpenAI, DeepSeek, ...)
// by its code or type, falling back to the HTTP status.
func openaiReason(e *openai.APIError) string {
	code, _ := e.Code.(string)
	return cmp.Or(openaiCodeReason(code), openaiCodeReason(e.Type), reviewer.ReasonForHTTPStatus(e.HTTPStatusCode))
}

// openaiCodeReason maps an OpenAI error code (or type) to a reviewer.RunReason*,
// or "" when it names no known cause.
func openaiCodeReason(code string) string {
	switch code {
	case "insufficient_quota":
		return reviewer.RunReasonBilling
	case "rate_limit_exceeded":
		return reviewer.RunReasonRateLimit
	case "server_error":
		return reviewer.RunReasonAPIError
	}
	return ""
}

// responseFailedError is a Responses API reply that came back with status
// "failed": HTTP 200, the failure in its body.
type responseFailedError struct {
	id, code, message string
}

func newResponseFailedError(r openai.CreateResponseResponse) *responseFailedError {
	e := &responseFailedError{id: r.ID}
	if r.Error != nil {
		e.code, e.message = r.Error.Code, r.Error.Message
	}
	return e
}

func (e *responseFailedError) Error() string {
	if e.code == "" {
		return fmt.Sprintf("response %s failed", e.id)
	}
	return fmt.Sprintf("response %s failed: %s: %s", e.id, e.code, e.message)
}

// reason maps the failure code to a reviewer.RunReason*; a failure without a
// code is taken for a server error.
func (e *responseFailedError) reason() string {
	if e.code == "" {
		return reviewer.RunReasonAPIError
	}
	return cmp.Or(openaiCodeReason(e.code), reviewer.RunReasonOther)
}

// isOpenAITransient reports whether an OpenAI-protocol call is worth retrying:
// a rate limit or server-side API error — as an HTTP status or inside a failed
// Responses reply — or a network-level request error. Billing
// (insufficient_quota also comes as a 429) and auth are definitive.
func isOpenAITransient(err error) bool {
	var apiErr *openai.APIError
	if errors.As(err, &apiErr) {
		return isTransientReason(openaiReason(apiErr))
	}
	var rf *responseFailedError
	if errors.As(err, &rf) {
		return isTransientReason(rf.reason())
	}
	var reqErr *openai.RequestError
	return errors.As(err, &reqErr)
}

// isTransientReason reports whether a failure of that reason may pass on retry.
func isTransientReason(reason string) bool {
	return reason == reviewer.RunReasonRateLimit || reason == reviewer.RunReasonAPIError
}

// isAnthropicTransient reports whether a failed Messages call is worth
// retrying: anything but a definitive rejection (billing, auth, a bad request).
// It covers what the SDK would retry on request open (408/409/429/5xx), an
// error event inside the SSE stream (200 + api_error/overloaded_error) and a
// reset HTTP/2 stream — a bare transport error, hence retry-by-default.
func isAnthropicTransient(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var ae *anthropic.Error
	if !errors.As(err, &ae) {
		return true
	}
	switch ae.Type() {
	case anthropic.ErrorTypeRateLimitError, anthropic.ErrorTypeOverloadedError,
		anthropic.ErrorTypeAPIError, anthropic.ErrorTypeTimeoutError:
		return !isCreditBalanceError(ae.RawJSON())
	case anthropic.ErrorTypeInvalidRequestError, anthropic.ErrorTypeAuthenticationError, anthropic.ErrorTypePermissionError,
		anthropic.ErrorTypeNotFoundError, anthropic.ErrorTypeBillingError:
		return false
	}
	// Unrecognized body: decide by status (200 = an error event mid-stream).
	switch ae.StatusCode {
	case http.StatusOK, http.StatusRequestTimeout, http.StatusConflict, http.StatusTooManyRequests:
		return true
	}
	return ae.StatusCode >= http.StatusInternalServerError
}

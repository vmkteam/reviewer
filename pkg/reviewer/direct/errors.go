package direct

import (
	"context"
	"errors"
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

// openaiReason classifies an OpenAI-protocol API error (OpenAI, DeepSeek, ...).
func openaiReason(e *openai.APIError) string {
	if e.Type == "insufficient_quota" || e.Code == "insufficient_quota" {
		return reviewer.RunReasonBilling
	}
	return reviewer.ReasonForHTTPStatus(e.HTTPStatusCode)
}

// isOpenAITransient reports whether an OpenAI-protocol call is worth retrying:
// a rate limit or server-side API error, or a network-level request error.
// Billing (insufficient_quota also comes as a 429) and auth are definitive.
func isOpenAITransient(err error) bool {
	var apiErr *openai.APIError
	if errors.As(err, &apiErr) {
		r := openaiReason(apiErr)
		return r == reviewer.RunReasonRateLimit || r == reviewer.RunReasonAPIError
	}
	var reqErr *openai.RequestError
	return errors.As(err, &reqErr)
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

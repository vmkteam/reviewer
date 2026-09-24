package direct

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"

	"reviewsrv/pkg/reviewer"

	"github.com/anthropics/anthropic-sdk-go"
	openai "github.com/sashabaranov/go-openai"
)

// providerReason maps a failed LLMProvider.Complete (after the provider's own
// retries) to a reviewer.RunReason*.
func providerReason(err error) string {
	var (
		ae     *anthropic.Error
		apiErr *openai.APIError
		reqErr *openai.RequestError
		rf     *responseFailedError
	)
	switch {
	case errors.As(err, &ae):
		return anthropicReason(ae)
	case errors.As(err, &apiErr):
		return openaiReason(apiErr)
	case errors.As(err, &reqErr):
		// An error body go-openai could not decode (a proxy's HTML page).
		return reviewer.ReasonForHTTPStatus(reqErr.HTTPStatusCode)
	case errors.As(err, &rf):
		return rf.reason()
	}
	// No API answer: a transport failure (reset stream, EOF, refused
	// connection) that outlived the retries.
	return reviewer.RunReasonAPIError
}

// anthropicReason classifies an Anthropic API error by its body type, falling
// back to the HTTP status. Errors inside the SSE stream carry the stream's 200.
func anthropicReason(e *anthropic.Error) string {
	if reviewer.IsBillingMessage(e.RawJSON()) {
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
// a rate limit or server-side error — by HTTP status or inside a failed
// Responses reply — or a transport failure with no answer (go-openai returns it
// bare, e.g. a *url.Error). Billing (insufficient_quota also comes as a 429),
// auth and bad requests are definitive, and so is anything else: go-openai's
// own request validation fails the same way every time. A cancelled context is
// withRetry's call.
func isOpenAITransient(err error) bool {
	var (
		apiErr *openai.APIError
		reqErr *openai.RequestError
		rf     *responseFailedError
	)
	if errors.As(err, &apiErr) || errors.As(err, &reqErr) || errors.As(err, &rf) {
		return isTransientReason(providerReason(err))
	}
	return isTransportError(err)
}

// isTransportError reports a failure with no HTTP answer: a reset, refused or
// timed-out connection, or a body cut short.
func isTransportError(err error) bool {
	var (
		urlErr *url.Error
		netErr net.Error
	)
	return errors.As(err, &urlErr) || errors.As(err, &netErr) ||
		errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF)
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
	var ae *anthropic.Error
	if !errors.As(err, &ae) {
		return true
	}
	switch ae.Type() {
	case anthropic.ErrorTypeRateLimitError, anthropic.ErrorTypeOverloadedError,
		anthropic.ErrorTypeAPIError, anthropic.ErrorTypeTimeoutError:
		return !reviewer.IsBillingMessage(ae.RawJSON())
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

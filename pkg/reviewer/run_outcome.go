package reviewer

import (
	"context"
	"errors"
	"net/http"
)

// Status of a reviewctl run, reported with its debug bundle. The status and
// reason sets are closed: reviewsrv turns them into Prometheus labels, so an
// unknown client value is normalized (NormalizeRunOutcome), never trusted.
const (
	RunStatusOK        = "ok"
	RunStatusFailed    = "failed"
	RunStatusCancelled = "cancelled" // the CI job was cancelled — not a failure
	RunStatusTimeout   = "timeout"
)

// Reasons of a non-ok run. Every reason but cancelled/timeout means RunStatusFailed.
const (
	RunReasonBilling      = "billing"       // provider credit balance exhausted
	RunReasonAuth         = "auth"          // invalid or missing API key
	RunReasonRateLimit    = "rate_limit"    // provider rate limit (after retries)
	RunReasonAPIError     = "api_error"     // provider 5xx / overloaded / dropped stream (after retries)
	RunReasonMaxRounds    = "max_rounds"    // direct: round budget exhausted without submit_review
	RunReasonTruncated    = "truncated"     // direct: consecutive rounds cut by the output-token cap
	RunReasonNotSubmitted = "not_submitted" // direct: the model stopped without submit_review
	RunReasonCancelled    = "cancelled"
	RunReasonTimeout      = "timeout"
	RunReasonOther        = "other"
)

var failureReasons = map[string]bool{
	RunReasonBilling:      true,
	RunReasonAuth:         true,
	RunReasonRateLimit:    true,
	RunReasonAPIError:     true,
	RunReasonMaxRounds:    true,
	RunReasonTruncated:    true,
	RunReasonNotSubmitted: true,
	RunReasonOther:        true,
}

// ReasonForHTTPStatus maps the HTTP status of a failed API call to a
// RunReason*: the fallback when the error names no more specific kind.
func ReasonForHTTPStatus(code int) string {
	switch {
	case code == http.StatusPaymentRequired:
		return RunReasonBilling
	case code == http.StatusUnauthorized, code == http.StatusForbidden:
		return RunReasonAuth
	case code == http.StatusTooManyRequests:
		return RunReasonRateLimit
	case code >= http.StatusInternalServerError:
		return RunReasonAPIError
	}
	return RunReasonOther
}

// RunError tags a runner failure with a RunReason* so the controller reports a
// bounded reason without parsing the message. The message is the wrapped error's.
type RunError struct {
	Reason string
	Err    error
}

func (e *RunError) Error() string { return e.Err.Error() }
func (e *RunError) Unwrap() error { return e.Err }

// WithRunReason tags err with a failure reason. A nil err or an empty reason
// returns err unchanged.
func WithRunReason(reason string, err error) error {
	if err == nil || reason == "" {
		return err
	}
	return &RunError{Reason: reason, Err: err}
}

// RunOutcome classifies the error of a run into a status and a reason. A
// cancelled or expired context wins over any runner-reported reason: a job
// cancelled mid-request surfaces as a transport error, not as a cancellation.
func RunOutcome(err error) (status, reason string) {
	var re *RunError
	switch {
	case err == nil:
		return RunStatusOK, ""
	case errors.Is(err, context.DeadlineExceeded):
		return RunStatusTimeout, RunReasonTimeout
	case errors.Is(err, context.Canceled):
		return RunStatusCancelled, RunReasonCancelled
	case errors.As(err, &re):
		if re.Reason == RunReasonCancelled {
			return RunStatusCancelled, RunReasonCancelled
		}
		if failureReasons[re.Reason] {
			return RunStatusFailed, re.Reason
		}
	}
	return RunStatusFailed, RunReasonOther
}

// NormalizeRunOutcome sanitizes a client-reported status/reason pair. Older
// reviewctl builds send neither; the status then follows hasError.
func NormalizeRunOutcome(status, reason string, hasError bool) (string, string) {
	switch status {
	case RunStatusOK:
		return RunStatusOK, ""
	case RunStatusCancelled:
		return RunStatusCancelled, RunReasonCancelled
	case RunStatusTimeout:
		return RunStatusTimeout, RunReasonTimeout
	case RunStatusFailed:
	default:
		if !hasError {
			return RunStatusOK, ""
		}
	}
	if !failureReasons[reason] {
		reason = RunReasonOther
	}
	return RunStatusFailed, reason
}

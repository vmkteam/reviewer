package app

import (
	"encoding/json"

	"reviewsrv/pkg/reviewer"

	"github.com/getsentry/sentry-go"
)

// MaskSentryKeys is a sentry BeforeSend hook that keeps project keys out of
// events: zenrpc attaches each call's raw params, the echo integration the
// request URL, and both can carry a key.
func MaskSentryKeys(e *sentry.Event, _ *sentry.EventHint) *sentry.Event {
	if p, ok := e.Extra["params"].(json.RawMessage); ok {
		e.Extra["params"] = json.RawMessage(reviewer.MaskKeys(string(p)))
	}
	if e.Request != nil {
		e.Request.URL = reviewer.MaskKeys(e.Request.URL)
	}
	return e
}

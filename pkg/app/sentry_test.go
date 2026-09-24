package app

import (
	"encoding/json"
	"testing"

	"github.com/getsentry/sentry-go"
	"github.com/stretchr/testify/assert"
)

func TestMaskSentryKeys(t *testing.T) {
	const key = "11111111-2222-3333-4444-555555555555"
	e := MaskSentryKeys(&sentry.Event{
		Extra:   map[string]any{"params": json.RawMessage(`{"projectKey":"` + key + `"}`), "ip": "10.0.0.1"},
		Request: &sentry.Request{URL: "https://srv/v1/reviewctl/upload/" + key + "/"},
	}, nil)

	assert.JSONEq(t, `{"projectKey":"11111111…"}`, string(e.Extra["params"].(json.RawMessage)))
	assert.Equal(t, "https://srv/v1/reviewctl/upload/11111111…/", e.Request.URL)
	assert.Equal(t, "10.0.0.1", e.Extra["ip"])
	assert.NotNil(t, MaskSentryKeys(&sentry.Event{}, nil), "an event without params or request")
}

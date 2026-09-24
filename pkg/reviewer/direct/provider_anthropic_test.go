package direct

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"reviewsrv/pkg/reviewer"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/stretchr/testify/require"
)

func TestToAnthropicMessagesReplaysRawAssistant(t *testing.T) {
	raw := anthropic.NewAssistantMessage(anthropic.NewTextBlock("verbatim with thinking"))

	out := toAnthropicMessages(Request{Messages: []Message{
		{Role: RoleUser, Text: "go"},
		// Text here must be ignored in favour of the verbatim Raw turn.
		{Role: RoleAssistant, Text: "rebuilt-should-not-be-used", Raw: raw},
		{Role: RoleTool, ToolResults: []ToolResult{{CallID: "1", Content: "ok"}}},
	}})

	require.Len(t, out, 3)
	require.Equal(t, raw, out[1], "assistant turn must be replayed verbatim from Raw")
}

func TestToAnthropicMessagesRebuildsWithoutRaw(t *testing.T) {
	out := toAnthropicMessages(Request{Messages: []Message{
		{Role: RoleAssistant, Text: "hi", ToolCalls: []ToolCall{{ID: "1", Name: "t", Args: []byte(`{}`)}}},
	}})
	require.Len(t, out, 1)
	require.Equal(t, anthropic.MessageParamRoleAssistant, out[0].Role)
}

// sseOK is a minimal successful Messages stream answering "hi".
const sseOK = `event: message_start
data: {"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"m","content":[],"stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":10,"output_tokens":1}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hi"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":5}}

event: message_stop
data: {"type":"message_stop"}

`

// sseAPIError is the 200 OK + in-stream error seen in CI.
const sseAPIError = `event: error
data: {"type":"error","error":{"details":null,"type":"api_error","message":"Internal server error"}}

`

// anthropicStub serves the scripted responses in order, one per request.
func anthropicStub(t *testing.T, responses ...func(w http.ResponseWriter)) (LLMProvider, *atomic.Int32) {
	t.Helper()
	prev := anthropicRetry.backoff
	anthropicRetry.backoff = time.Millisecond
	t.Cleanup(func() { anthropicRetry.backoff = prev })

	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		i := int(calls.Add(1)) - 1
		if i >= len(responses) {
			t.Errorf("unexpected extra request #%d", i+1)
			w.WriteHeader(http.StatusTeapot)
			return
		}
		responses[i](w)
	}))
	t.Cleanup(srv.Close)

	p, err := NewAnthropicProvider(AnthropicConfig{APIKey: "k", BaseURL: srv.URL, Model: "m"})
	require.NoError(t, err)
	return p, &calls
}

func sse(body string) func(w http.ResponseWriter) {
	return func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, body)
	}
}

// jsonError answers with an API error. The SDK's own retries are off, so every
// request the stub counts is one attempt of the provider's retry loop.
func jsonError(status int, body string) func(w http.ResponseWriter) {
	return func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}
}

// dropStream starts a stream and kills the connection mid-way, like the reset
// HTTP/2 stream seen in CI.
func dropStream(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	_, _ = io.WriteString(w, strings.SplitAfter(sseOK, "\n\n")[0])
	w.(http.Flusher).Flush()
	panic(http.ErrAbortHandler)
}

func TestAnthropicCompleteRetriesTransientFailures(t *testing.T) {
	p, calls := anthropicStub(t,
		sse(sseAPIError),
		jsonError(http.StatusInternalServerError, `{"type":"error","error":{"type":"api_error","message":"Internal server error"}}`),
		dropStream,
		sse(sseOK),
	)

	var retries []string
	resp, err := p.Complete(t.Context(), Request{
		Messages: []Message{{Role: RoleUser, Text: "go"}},
		OnRetry:  func(err error) { retries = append(retries, err.Error()) },
	})
	require.NoError(t, err)
	require.Equal(t, "hi", resp.Text)
	require.Equal(t, int32(4), calls.Load())
	require.Len(t, retries, 3, "each retried failure is reported")
	require.Contains(t, retries[0], "api_error")
}

func TestAnthropicCompleteGivesUpAfterRetryBudget(t *testing.T) {
	p, calls := anthropicStub(t, sse(sseAPIError), sse(sseAPIError), sse(sseAPIError), sse(sseAPIError))

	_, err := p.Complete(t.Context(), Request{Messages: []Message{{Role: RoleUser, Text: "go"}}})
	require.Error(t, err)
	require.Equal(t, int32(maxRetries+1), calls.Load())
	require.Equal(t, reviewer.RunReasonAPIError, providerReason(err))
}

func TestAnthropicCompleteFailsFastOnBilling(t *testing.T) {
	p, calls := anthropicStub(t,
		jsonError(http.StatusBadRequest, `{"type":"error","error":{"type":"invalid_request_error","message":"Your credit balance is too low to access the Anthropic API."}}`),
	)

	_, err := p.Complete(t.Context(), Request{Messages: []Message{{Role: RoleUser, Text: "go"}}})
	require.Error(t, err)
	require.Equal(t, int32(1), calls.Load(), "billing must not be retried")
	require.Equal(t, reviewer.RunReasonBilling, providerReason(err))
}

func TestIsAnthropicTransient(t *testing.T) {
	require.True(t, isAnthropicTransient(errors.New("stream error: stream ID 41; INTERNAL_ERROR; received from peer")))
}

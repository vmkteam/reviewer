package runner

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"syscall"

	"reviewsrv/pkg/reviewer"
)

// Claude Code error kinds: the top-level "error" of the synthetic assistant
// event it emits for a failed API call (SDKAssistantMessageError).
const (
	claudeErrBilling   = "billing_error"
	claudeErrAuth      = "authentication_failed"
	claudeErrRateLimit = "rate_limit"
	claudeErrServer    = "server_error"
	claudeErrUnknown   = "unknown"
)

// claudeAPIErrorPrefix starts the result text of an API failure that Claude Code
// could not classify (kind "unknown"), e.g. "API Error: Internal server error".
const claudeAPIErrorPrefix = "API Error"

// claudeRunError builds the error of a claude run that exited non-zero. The exit
// status alone ("exit status 1") says nothing, so the API failure reported inside
// the stream-json output — the last assistant "error" kind and the final result
// (is_error, result text, api_error_status) — leads the message and picks the
// RunReason. A SIGTERM/SIGINT exit is a cancelled CI job, not a failure; runExec
// has already tagged it.
func claudeRunError(err error, cr *ClaudeResult, stdout []byte, stderr string) error {
	if isInterrupted(err) {
		return fmt.Errorf("claude interrupted by a signal: %w", err)
	}

	kind, text := lastAssistantError(stdout)
	status := 0
	if cr != nil && cr.IsError {
		text = cmp.Or(strings.TrimSpace(cr.Result), text)
		status = cr.APIErrorStatus
	}
	if kind == "" && text == "" {
		return fmt.Errorf("claude exited with error: %w (stderr: %s)", err, truncate(stderr, 500))
	}

	msg := text
	if kind != "" && kind != claudeErrUnknown {
		msg = kind + ": " + text
	}
	api := ""
	if status > 0 {
		api = fmt.Sprintf("api %d, ", status)
	}
	return reviewer.WithRunReason(claudeReason(kind, status, text), fmt.Errorf("%s (%sclaude %w)", msg, api, err))
}

// claudeReason maps a Claude Code error kind (falling back to the HTTP status and
// the result text) to a RunReason.
func claudeReason(kind string, status int, text string) string {
	if kind == claudeErrBilling || reviewer.IsBillingMessage(text) {
		// Before the status: an exhausted balance can come as a 400 or a 429.
		return reviewer.RunReasonBilling
	}
	switch kind {
	case claudeErrAuth:
		return reviewer.RunReasonAuth
	case claudeErrRateLimit:
		return reviewer.RunReasonRateLimit
	case claudeErrServer:
		return reviewer.RunReasonAPIError
	}
	if r := reviewer.ReasonForHTTPStatus(status); r != reviewer.RunReasonOther {
		return r
	}
	if r := reviewer.ReasonFromMessage(text); r != "" {
		return r
	}
	if strings.HasPrefix(text, claudeAPIErrorPrefix) {
		return reviewer.RunReasonAPIError
	}
	return reviewer.RunReasonOther
}

// lastAssistantError returns the error kind and text of the last assistant event
// carrying a top-level "error" in a stream-json output, or empty strings.
func lastAssistantError(stdout []byte) (kind, text string) {
	for line := range bytes.SplitSeq(stdout, []byte("\n")) {
		line = bytes.TrimSpace(line)
		// Cheap pre-filter: most lines are tool traffic without an error field.
		if len(line) == 0 || line[0] != '{' || !bytes.Contains(line, []byte(`"error"`)) {
			continue
		}
		var ev struct {
			Type    string `json:"type"`
			Error   string `json:"error"`
			Message struct {
				Content []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
			} `json:"message"`
		}
		if json.Unmarshal(line, &ev) != nil || ev.Type != "assistant" || ev.Error == "" {
			continue
		}
		var sb strings.Builder
		for _, c := range ev.Message.Content {
			if c.Type == "text" {
				sb.WriteString(c.Text)
			}
		}
		kind, text = ev.Error, strings.TrimSpace(sb.String())
	}
	return kind, text
}

// errTail is the end of a CLI's error output, where its final error sits: the
// lines before it (stack traces, the checkout's paths) would only mislead
// ReasonFromMessage.
func errTail(s string) string {
	const n = 2000
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

// isInterrupted reports whether a subprocess was stopped by SIGTERM/SIGINT —
// killed outright ("signal: terminated") or exiting 128+signo after handling it
// (143/130), as Claude Code does. In CI that is a cancelled job.
func isInterrupted(err error) bool {
	var ee *exec.ExitError
	if !errors.As(err, &ee) {
		return false
	}
	if ws, ok := ee.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		return ws.Signal() == syscall.SIGTERM || ws.Signal() == syscall.SIGINT
	}
	switch ee.ExitCode() {
	case 128 + int(syscall.SIGTERM), 128 + int(syscall.SIGINT):
		return true
	}
	return false
}

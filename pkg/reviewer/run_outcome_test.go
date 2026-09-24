package reviewer

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunOutcome(t *testing.T) {
	billing := WithRunReason(RunReasonBilling, errors.New("billing_error: Credit balance is too low"))
	tests := []struct {
		name       string
		err        error
		wantStatus string
		wantReason string
	}{
		{"nil", nil, RunStatusOK, ""},
		{"plain error", errors.New("boom"), RunStatusFailed, RunReasonOther},
		{"tagged", fmt.Errorf("run claude: %w", billing), RunStatusFailed, RunReasonBilling},
		{"unknown tag", WithRunReason("weird", errors.New("x")), RunStatusFailed, RunReasonOther},
		{"deadline", fmt.Errorf("round 3: %w", context.DeadlineExceeded), RunStatusTimeout, RunReasonTimeout},
		{"cancelled ctx", fmt.Errorf("run: %w", context.Canceled), RunStatusCancelled, RunReasonCancelled},
		{"signal tag", WithRunReason(RunReasonCancelled, errors.New("exit status 143")), RunStatusCancelled, RunReasonCancelled},
		// A cancelled context wins over the reason the runner derived from the
		// resulting transport error.
		{"ctx wins over tag", WithRunReason(RunReasonAPIError, fmt.Errorf("stream: %w", context.Canceled)), RunStatusCancelled, RunReasonCancelled},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, reason := RunOutcome(tt.err)
			assert.Equal(t, tt.wantStatus, status)
			assert.Equal(t, tt.wantReason, reason)
		})
	}
}

func TestWithRunReasonKeepsNil(t *testing.T) {
	require.NoError(t, WithRunReason(RunReasonBilling, nil))
	err := errors.New("x")
	assert.Same(t, err, WithRunReason("", err))
}

func TestNormalizeRunOutcome(t *testing.T) {
	tests := []struct {
		name               string
		status, reason     string
		hasError           bool
		wantStatus, wantRs string
	}{
		{"ok drops reason", RunStatusOK, RunReasonBilling, false, RunStatusOK, ""},
		{"failed keeps known reason", RunStatusFailed, RunReasonMaxRounds, true, RunStatusFailed, RunReasonMaxRounds},
		{"failed unknown reason", RunStatusFailed, "rm -rf", true, RunStatusFailed, RunReasonOther},
		{"cancelled", RunStatusCancelled, "", true, RunStatusCancelled, RunReasonCancelled},
		{"timeout", RunStatusTimeout, "whatever", true, RunStatusTimeout, RunReasonTimeout},
		{"legacy client with error", "", "", true, RunStatusFailed, RunReasonOther},
		{"legacy client without error", "", "", false, RunStatusOK, ""},
		{"garbage status with error", "boom", RunReasonAuth, true, RunStatusFailed, RunReasonAuth},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, reason := NormalizeRunOutcome(tt.status, tt.reason, tt.hasError)
			assert.Equal(t, tt.wantStatus, status)
			assert.Equal(t, tt.wantRs, reason)
		})
	}
}

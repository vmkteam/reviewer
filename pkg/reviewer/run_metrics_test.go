package reviewer

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// counterValue reads a counter without the (unvendored) testutil package.
func counterValue(t *testing.T, c prometheus.Counter) float64 {
	t.Helper()
	var m dto.Metric
	require.NoError(t, c.Write(&m))
	return m.GetCounter().GetValue()
}

func TestRunMetricsObserve(t *testing.T) {
	m := NewRunMetrics("claude", "direct")

	m.Observe("demo", "direct", RunStatusFailed, RunReasonMaxRounds, 7.9)
	m.Observe("demo", "direct", RunStatusFailed, RunReasonMaxRounds, 10.7)
	m.Observe("", "rm -rf /", RunStatusFailed, RunReasonOther, 0)
	m.Observe("demo", "direct", RunStatusFailed, RunReasonMaxRounds, 1e9) // implausible: counted, cost dropped

	assert.InDelta(t, 3, counterValue(t, m.runs.WithLabelValues("demo", "direct", RunStatusFailed, RunReasonMaxRounds)), 0)
	assert.InDelta(t, 18.6, counterValue(t, m.cost.WithLabelValues("demo", "direct", RunStatusFailed)), 1e-9)
	// Unknown projects and runners collapse into fixed label values.
	assert.InDelta(t, 1, counterValue(t, m.runs.WithLabelValues(metricUnknownProject, metricOtherRunner, RunStatusFailed, RunReasonOther)), 0)

	var nilMetrics *RunMetrics
	assert.NotPanics(t, func() { nilMetrics.Observe("p", "claude", RunStatusOK, "", 1) })
}

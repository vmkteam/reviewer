package reviewer

import "github.com/prometheus/client_golang/prometheus"

// Label values for runs whose project or runner is not known to reviewsrv, so a
// client can't mint unbounded label values.
const (
	metricUnknownProject = "unknown"
	metricOtherRunner    = "other"
)

// PanelRunner is the runner label of a multi-review panel's own failure (prompt
// fetch, worktree, upload), outside any member or judge run.
const PanelRunner = "panel"

// maxRunCostUsd bounds a plausible run cost (a 500-round Opus run stays well
// below it): a higher client-reported value is not added to the cost counter,
// which only ever grows.
const maxRunCostUsd = 1000

// RunMetrics counts reviewctl runs on reviewsrv by outcome: an ok run when its
// review is uploaded, any other run when its debug bundle arrives. A nil
// *RunMetrics is a no-op.
type RunMetrics struct {
	runners map[string]bool
	runs    *prometheus.CounterVec
	cost    *prometheus.CounterVec
}

// NewRunMetrics builds the collectors; runners is the closed set of runner
// names accepted as a label value (anything else is counted as "other").
func NewRunMetrics(runners ...string) *RunMetrics {
	m := &RunMetrics{
		runners: make(map[string]bool, len(runners)),
		runs: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "reviewer_runs_total",
			Help: "reviewctl runs by project, runner, status (ok|failed|cancelled|timeout) and failure reason.",
		}, []string{"project", "runner", "status", "reason"}),
		cost: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "reviewer_run_cost_usd_total",
			Help: "Dollars spent by reviewctl runs, by project, runner and status — incl. failed runs.",
		}, []string{"project", "runner", "status"}),
	}
	for _, r := range runners {
		m.runners[r] = true
	}
	return m
}

// Register adds the collectors to r.
func (m *RunMetrics) Register(r prometheus.Registerer) {
	r.MustRegister(m.runs, m.cost)
}

// Observe records one run. project is the project title ("" = unknown project);
// status and reason must already be normalized (NormalizeRunOutcome).
func (m *RunMetrics) Observe(project, runner, status, reason string, costUsd float64) {
	if m == nil {
		return
	}
	if project == "" {
		project = metricUnknownProject
	}
	if !m.runners[runner] {
		runner = metricOtherRunner
	}
	m.runs.WithLabelValues(project, runner, status, reason).Inc()
	if costUsd > 0 && costUsd <= maxRunCostUsd {
		m.cost.WithLabelValues(project, runner, status).Add(costUsd)
	}
}

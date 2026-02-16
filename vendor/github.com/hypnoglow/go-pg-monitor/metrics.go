package monitor

import "github.com/prometheus/client_golang/prometheus"

const (
	// defaultNamespace is default metric namespace.
	defaultNamespace = ""

	// defaultSubsystem is default metric subsystem.
	defaultSubsystem = "go_pg"
)

// Metrics represent a set of Prometheus metrics for database client stats.
type Metrics struct {
	namespace string
	subsystem string

	constLabels prometheus.Labels

	hits     *prometheus.GaugeVec
	misses   *prometheus.GaugeVec
	timeouts *prometheus.GaugeVec

	totalConns *prometheus.GaugeVec
	idleConns  *prometheus.GaugeVec
	staleConns *prometheus.GaugeVec

	registerer prometheus.Registerer
}

// MetricsOption is an option for NewMetrics.
type MetricsOption func(metrics *Metrics)

// MetricsWithNamespace is an option that sets metric namespace.
func MetricsWithNamespace(namespace string) MetricsOption {
	return func(metrics *Metrics) {
		metrics.namespace = namespace
	}
}

// MetricsWithSubsystem is an option that sets metric subsystem.
func MetricsWithSubsystem(subsystem string) MetricsOption {
	return func(metrics *Metrics) {
		metrics.subsystem = subsystem
	}
}

// MetricsWithConstLabels is an option that sets metric constant labels.
func MetricsWithConstLabels(constLabels prometheus.Labels) MetricsOption {
	return func(metrics *Metrics) {
		metrics.constLabels = constLabels
	}
}

// MetricsWithRegisterer is an option that sets custom registerer for metrics.
func MetricsWithRegisterer(registerer prometheus.Registerer) MetricsOption {
	return func(metrics *Metrics) {
		metrics.registerer = registerer
	}
}

// NewMetrics returns a new configured Metrics.
func NewMetrics(opts ...MetricsOption) *Metrics {
	m := &Metrics{
		namespace:  defaultNamespace,
		subsystem:  defaultSubsystem,
		registerer: prometheus.DefaultRegisterer,
	}

	for _, opt := range opts {
		opt(m)
	}

	hits := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   m.namespace,
			Subsystem:   m.subsystem,
			ConstLabels: m.constLabels,
			Name:        "pool_hits",
			Help:        "Number of times free connection was found in the pool",
		},
		[]string{"pool_name"},
	)
	m.registerer.MustRegister(hits)
	m.hits = hits

	misses := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   m.namespace,
			Subsystem:   m.subsystem,
			ConstLabels: m.constLabels,
			Name:        "pool_misses",
			Help:        "Number of times free connection was NOT found in the pool",
		},
		[]string{"pool_name"},
	)
	m.registerer.MustRegister(misses)
	m.misses = misses

	timeouts := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   m.namespace,
			Subsystem:   m.subsystem,
			ConstLabels: m.constLabels,
			Name:        "pool_timeouts",
			Help:        "Number of times a wait timeout occurred",
		},
		[]string{"pool_name"},
	)
	m.registerer.MustRegister(timeouts)
	m.timeouts = timeouts

	totalConns := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   m.namespace,
			Subsystem:   m.subsystem,
			ConstLabels: m.constLabels,
			Name:        "pool_total_connections",
			Help:        "Number of total connections in the pool",
		},
		[]string{"pool_name"},
	)
	m.registerer.MustRegister(totalConns)
	m.totalConns = totalConns

	idleConns := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   m.namespace,
			Subsystem:   m.subsystem,
			ConstLabels: m.constLabels,
			Name:        "pool_idle_connections",
			Help:        "Number of idle connections in the pool",
		},
		[]string{"pool_name"},
	)
	m.registerer.MustRegister(idleConns)
	m.idleConns = idleConns

	staleConns := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   m.namespace,
			Subsystem:   m.subsystem,
			ConstLabels: m.constLabels,
			Name:        "pool_stale_connections",
			Help:        "Number of stale connections removed from the pool",
		},
		[]string{"pool_name"},
	)
	m.registerer.MustRegister(staleConns)
	m.staleConns = staleConns

	return m
}

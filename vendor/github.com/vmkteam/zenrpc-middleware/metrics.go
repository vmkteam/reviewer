package middleware

import (
	"context"
	"encoding/json"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/vmkteam/appkit"
	"github.com/vmkteam/zenrpc/v2"
)

const methodNotFound = "methodNotFound"

//nolint:gochecknoglobals // need for once metrics registration
var (
	registerMetricsOnce sync.Once

	rpcErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "app",
		Subsystem: "rpc",
		Name:      "error_requests_total",
		Help:      "Error requests count by method and error code.",
	}, []string{"method", "code", "platform", "version", "server"})
	rpcDurations = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Namespace: "app",
		Subsystem: "rpc",
		Name:      "responses_duration_seconds",
		Help:      "Response time by method and error code.",
	}, []string{"method", "code", "platform", "version", "server"})
)

// WithMetrics logs duration of RPC requests via Prometheus. Default serverName is rpc will be in server label.
// It exposes two metrics: `app_rpc_error_requests_total` and `app_rpc_responses_duration_seconds`.
// Labels: method, code, platform, version, server.
func WithMetrics(serverName string) zenrpc.MiddlewareFunc {
	if serverName == "" {
		serverName = "rpc"
	}

	registerMetricsOnce.Do(func() {
		prometheus.MustRegister(rpcErrors, rpcDurations)
	})

	return func(h zenrpc.InvokeFunc) zenrpc.InvokeFunc {
		return func(ctx context.Context, method string, params json.RawMessage) zenrpc.Response {
			start, code := time.Now(), ""
			r := h(ctx, method, params)

			// log metrics
			if n := zenrpc.NamespaceFromContext(ctx); n != "" {
				method = n + "." + method
			}

			// set platform & version
			platform, version := appkit.PlatformFromContext(ctx), appkit.VersionFromContext(ctx)

			if r.Error != nil {
				if r.Error.Code == zenrpc.MethodNotFound {
					method = methodNotFound
				}

				code = strconv.Itoa(r.Error.Code)
				rpcErrors.WithLabelValues(method, code, platform, version, serverName).Inc()
			}

			rpcDurations.WithLabelValues(method, code, platform, version, serverName).Observe(time.Since(start).Seconds())

			return r
		}
	}
}

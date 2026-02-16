package zenrpc

import (
	"context"
	"encoding/json"
	"log"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Logger is middleware for JSON-RPC 2.0 Server.
// It's just an example for middleware, will be refactored later.
func Logger(l *log.Logger) MiddlewareFunc {
	return func(h InvokeFunc) InvokeFunc {
		return func(ctx context.Context, method string, params json.RawMessage) Response {
			start, ip := time.Now(), "<nil>"
			if req, ok := RequestFromContext(ctx); ok && req != nil {
				ip = req.RemoteAddr
			}

			r := h(ctx, method, params)
			l.Printf("ip=%s method=%s.%s duration=%v params=%s err=%s", ip, NamespaceFromContext(ctx), method, time.Since(start), params, r.Error)

			return r
		}
	}
}

// Metrics is a middleware for logging duration of RPC requests via Prometheus.
// It exposes two metrics: app_rpc_error_requests_total and app_rpc_responses_duration_seconds.
func Metrics(serverName string) MiddlewareFunc {
	if serverName == "" {
		serverName = "rpc"
	}

	rpcErrors := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "app",
		Subsystem: "rpc",
		Name:      "error_requests_total",
		Help:      "Error requests count by method and error code.",
	}, []string{"server", "method", "code"})

	rpcDurations := prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Namespace: "app",
		Subsystem: "rpc",
		Name:      "responses_duration_seconds",
		Help:      "Response time by method and error code.",
	}, []string{"server", "method", "code"})

	prometheus.MustRegister(rpcErrors, rpcDurations)

	return func(h InvokeFunc) InvokeFunc {
		return func(ctx context.Context, method string, params json.RawMessage) Response {
			start, code := time.Now(), ""
			r := h(ctx, method, params)

			// log metrics
			if n := NamespaceFromContext(ctx); n != "" {
				method = n + "." + method
			}

			if r.Error != nil {
				code = strconv.Itoa(r.Error.Code)
				rpcErrors.WithLabelValues(serverName, method, code).Inc()
			}

			rpcDurations.WithLabelValues(serverName, method, code).Observe(time.Since(start).Seconds())

			return r
		}
	}
}

package appkit

import (
	"fmt"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	sentryecho "github.com/getsentry/sentry-go/echo"
	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus"
)

const DefaultServerName = "default"

// NewEcho returns echo.Echo with default settings and middlewares.
func NewEcho() *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	_, mask, _ := net.ParseCIDR("0.0.0.0/0")
	e.IPExtractor = echo.ExtractIPFromRealIPHeader(echo.TrustIPRange(mask))

	// use sentry middleware
	e.Use(sentryecho.New(sentryecho.Options{
		Repanic:         true,
		WaitForDelivery: true,
	}))

	// use zenrpc middlewares
	e.Use(EchoIPContext(), EchoSentryHubContext())

	return e
}

// RenderRoutes is a simple echo handler that renders all routes as HTML.
func RenderRoutes(appName string, e *echo.Echo) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		// collect paths
		routesByPaths := make(map[string]struct{})
		var paths []string
		for _, route := range e.Routes() {
			if _, ok := routesByPaths[route.Path]; !ok {
				routesByPaths[route.Path] = struct{}{}
				paths = append(paths, strings.TrimRight(route.Path, "*"))
			}
		}
		sort.Strings(paths)

		// render template
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("<html><body><h1>%s</h1><ul>", appName))
		for _, path := range paths {
			sb.WriteString(fmt.Sprintf(`<li><a href="%s">%s</a></li>`, path, path))
		}
		sb.WriteString("</ul></body></html>")

		return ctx.HTML(http.StatusOK, sb.String())
	}
}

// PprofHandler returns echo.HandlerFunc with pprof http server.
func PprofHandler(c echo.Context) error {
	if h, p := http.DefaultServeMux.Handler(c.Request()); p != "" {
		h.ServeHTTP(c.Response(), c.Request())
		return nil
	}
	return echo.NewHTTPError(http.StatusNotFound)
}

// EchoHandlerFunc is http.HandlerFunc wrapper for Echo.
func EchoHandlerFunc(next http.HandlerFunc) echo.HandlerFunc {
	return echo.WrapHandler(next)
}

// HTTPMetrics is a echo.MiddlewareFunc function that logs duration of responses.
func HTTPMetrics(serverName string) echo.MiddlewareFunc {
	if serverName == "" {
		serverName = DefaultServerName
	}

	labels := []string{"method", "uri", "code", "server"}

	echoRequests := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "app",
		Subsystem: "http",
		Name:      "requests_total",
		Help:      "Requests count by method/path/status.",
	}, labels)

	echoDurations := prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Namespace: "app",
		Subsystem: "http",
		Name:      "responses_duration_seconds",
		Help:      "Response time by method/path/status.",
	}, labels)

	prometheus.MustRegister(echoRequests, echoDurations)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			if err := next(c); err != nil {
				c.Error(err)
			}

			metrics := []string{c.Request().Method, c.Path(), strconv.Itoa(c.Response().Status), serverName}

			echoDurations.WithLabelValues(metrics...).Observe(time.Since(start).Seconds())
			echoRequests.WithLabelValues(metrics...).Inc()

			return nil
		}
	}
}

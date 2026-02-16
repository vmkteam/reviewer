package app

import (
	"fmt"

	monitor "github.com/hypnoglow/go-pg-monitor"
	"github.com/hypnoglow/go-pg-monitor/gopgv10"
	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/vmkteam/appkit"
)

// registerMetrics is a function that initializes a.stat* variables and adds /metrics endpoint to echo.
func (a *App) registerMetrics() {
	// add db conn metrics
	dbMetrics := monitor.NewMetrics(monitor.MetricsWithConstLabels(prometheus.Labels{"connection_name": "default"}))
	dbOpts := a.db.Options()
	a.mon = monitor.NewMonitor(
		gopgv10.NewObserver(a.db.DB),
		dbMetrics,
		monitor.MonitorWithPoolName(fmt.Sprintf("%s/%s", dbOpts.Addr, dbOpts.Database)),
	)
	a.mon.Open()

	a.echo.Use(appkit.HTTPMetrics(appkit.DefaultServerName))
	a.echo.Any("/metrics", echo.WrapHandler(promhttp.Handler()))
}

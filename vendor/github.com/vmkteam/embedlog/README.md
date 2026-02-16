# embedlog

[![Linter Status](https://github.com/vmkteam/embedlog/actions/workflows/golangci-lint.yml/badge.svg?branch=master)](https://github.com/vmkteam/embedlog/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/vmkteam/embedlog)](https://goreportcard.com/report/github.com/vmkteam/embedlog)
[![Go Reference](https://pkg.go.dev/badge/github.com/vmkteam/embedlog.svg)](https://pkg.go.dev/github.com/vmkteam/embedlog)

Enhanced logging library for Go with Prometheus metrics integration and structured logging capabilities.

### Features
* **Dual-level logging:** Automatically splits logs between stdout (info) and stderr (errors)
* **Flexible output:** Supports both JSON and text formats
* **Simple API:** Familiar Print/Error style interface with context support
* **Embeddable:** Designed to be embedded in your application structures
* **Prometheus integration:** Built-in metrics for log events (app_log_events_total)
* **Source location:** Automatic file/line logging for better debugging
* **PrintOrEr:** Function for conditional logging.
* **NewDevLogger:** [Colored](https://github.com/lmittmann/tint) logger for development.

### Custom Metrics Integration
The library automatically exposes Prometheus metrics:

* `app_log_events_total{type="info"}` - Count of info-level logs
* `app_log_events_total{type="error"}` - Count of error-level logs

Use these metrics to set up alerts for error rate spikes.

### Best Practices
* For services: Use JSON format in production
* For CLI tools: Use text format with verbose flag
* For error tracking: Monitor `app_log_events_total{type="error"}`

### Quick Start
Please, see:
* `examples/main.go` for basic usage.
* `examples/dblog.go` for go-pg usage.

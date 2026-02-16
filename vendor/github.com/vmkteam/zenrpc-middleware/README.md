# zenrpc-middleware: Middlewares for vmkteam/zenrpc

[![Linter Status](https://github.com/vmkteam/zenrpc-middleware/actions/workflows/golangci-lint.yml/badge.svg?branch=master)](https://github.com/vmkteam/zenrpc-middleware/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/vmkteam/zenrpc-middleware)](https://goreportcard.com/report/github.com/vmkteam/zenrpc-middleware)
[![Go Reference](https://pkg.go.dev/badge/github.com/vmkteam/zenrpc-middleware.svg)](https://pkg.go.dev/github.com/vmkteam/zenrpc-middleware)

`zenrpc-middleware` is a set of common middlewares for [zenrpc](https://github.com/vmkteam/zenrpc) implementing logging,
metrics and error tracking.

## Middlewares

### WithDevel

Sets bool flag to context for detecting development environment.

### WithHeaders
    
Sets User-Agent, Platform, Version, X-Country headers to context. User-Agent strips to 2048 chars, Platform and Version – to 64, X-Country - to 16.

### WithAPILogger

Logs via Printf function (e.g. log.Printf) all requests. For example

```text
ip= platform="" version="" method=nodevel.arith.divide duration=63.659µs params="{ \"a\": 1, \"b\": 24 }" err=<nil> userAgent="Go-http-client/1.1"
```

### WithSLog

Logs via slog.InfoContext (or similar) function all requests with custom attrs support.

### WithSentry

Sets additional parameters for current Sentry scope. Extras: params, duration, ip. Tags: platform,
version, method.

### WithNoCancelContext

Ignores Cancel func from context. This is useful for passing context to `go-pg`.

### WithMetrics

Logs duration of RPC requests via Prometheus. Default serverName is rpc (will be in server label).
It exposes two metrics: `app_rpc_error_requests_total` and `app_rpc_responses_duration_seconds`.
Labels: method, code, platform, version, server.

### WithTiming

Adds timings in JSON-RPC 2.0 Response via `extensions` field (not in spec). Middleware is active
when `isDevel=true` or AllowDebugFunc returns `true`. Sample AllowDebugFunc (checks GET/POST parameters for "true"
value, like `?d=true`):

```go
allowDebugFn := func (param string) middleware.AllowDebugFunc {
    return func (req *http.Request) bool {
        return req.FormValue(param) == "true"
    }
}
```

`DurationLocal` – total method execution time in ms.

If `DurationRemote` or `DurationDiff` are set then `DurationLocal` excludes these values.

### WithSQLLogger

Adds `SQL` or `DurationSQL` fields in JSON-RPC 2.0 Response `extensions` field (not in spec).

`DurationSQL` field is set then  `isDevel=true` or AllowDebugFunc(allowDebugFunc) returns `true`.

`SQL` field is set then `isDevel=true` or AllowDebugFunc(allowDebugFunc, allowSqlDebugFunc) returns `true`.

### WithErrorLogger

Logs all errors (ErrorCode==500 or < 0) via Printf func and sends them to Sentry. It also removes
sensitive error data from response. It is good to use pkg/errors for stack trace support in sentry.

### WithErrorSLog

Same as `WithErrorLogger`, but for slog.

## Examples

### Basic usage

```go
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/go-pg/pg/v10"
	"github.com/vmkteam/zenrpc-middleware"
	"github.com/vmkteam/zenrpc/v2"
)

func main() {
	dbс := pg.Connect(&pg.Options{User: "postgres"})
	defer dbс.Close()

	isDevel := true
	elog := log.New(os.Stderr, "E", log.LstdFlags|log.Lshortfile)
	dlog := log.New(os.Stdout, "D", log.LstdFlags|log.Lshortfile)

	allowDebug := func(param string) middleware.AllowDebugFunc {
		return func(req *http.Request) bool {
			return req.FormValue(param) == "true"
		}
	}

	rpc := zenrpc.NewServer(zenrpc.Options{
		ExposeSMD: true,
		AllowCORS: true,
	})

	rpc.Use(
		middleware.WithDevel(isDevel),
		middleware.WithHeaders(),
		middleware.WithAPILogger(dlog.Printf, middleware.DefaultServerName),
		middleware.WithSentry(middleware.DefaultServerName),
		middleware.WithNoCancelContext(),
		middleware.WithMetrics(middleware.DefaultServerName),
		middleware.WithTiming(isDevel, allowDebug("d")),
		middleware.WithSQLLogger(dbc, isDevel, allowDebug("d"), allowDebug("s")),
		middleware.WithErrorLogger(elog.Printf, middleware.DefaultServerName),
	)
}


    // rpc.Register and server
```


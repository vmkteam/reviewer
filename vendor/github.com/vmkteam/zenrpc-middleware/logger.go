package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/vmkteam/appkit"
	"github.com/vmkteam/zenrpc/v2"
)

// ErrSkipLog is a special error for LogAttrs func. Log lines can be skipped.
var ErrSkipLog = errors.New("skip log")

// WithAPILogger logs via Printf function (e.g. log.Printf) all requests.
func WithAPILogger(pf Printf, serverName string) zenrpc.MiddlewareFunc {
	return func(h zenrpc.InvokeFunc) zenrpc.InvokeFunc {
		return func(ctx context.Context, method string, params json.RawMessage) zenrpc.Response {
			start := time.Now()
			r := h(ctx, method, params)

			methodName := fullMethodName(serverName, zenrpc.NamespaceFromContext(ctx), method)
			pf("ip=%s platform=%q version=%q method=%s duration=%v params=%q err=%q userAgent=%q xRequestId=%q",
				appkit.IPFromContext(ctx),
				appkit.PlatformFromContext(ctx),
				appkit.VersionFromContext(ctx),
				methodName,
				time.Since(start),
				params,
				r.Error,
				appkit.UserAgentFromContext(ctx),
				appkit.XRequestIDFromContext(ctx),
			)

			return r
		}
	}
}

// WithSLog logs via slog function (e.g. slog.InfoContext) all requests.
func WithSLog(pf Print, serverName string, fn LogAttrs) zenrpc.MiddlewareFunc {
	return func(h zenrpc.InvokeFunc) zenrpc.InvokeFunc {
		return func(ctx context.Context, method string, params json.RawMessage) zenrpc.Response {
			start := time.Now()
			r := h(ctx, method, params)

			// get additional args, check for ErrSkipLog
			var args []any
			if fn != nil {
				args = fn(ctx, r)
				if len(args) == 1 {
					if e, ok := args[0].(error); ok && errors.Is(e, ErrSkipLog) {
						return r
					}
				}
			}

			t := time.Since(start)
			logArgs := append(additionalArgs(ctx), []any{
				"method", fullMethodName(serverName, zenrpc.NamespaceFromContext(ctx), method),
				"duration", t.String(),
				"durationMS", t.Milliseconds(),
				"params", params,
				"err", r.Error,
				"userAgent", appkit.UserAgentFromContext(ctx),
				"xRequestId", appkit.XRequestIDFromContext(ctx),
			}...)

			pf(ctx, "rpc", append(logArgs, args...)...)
			return r
		}
	}
}

func additionalArgs(ctx context.Context) []any {
	r := make([]any, 0, 4)
	r = append(r, "ip", appkit.IPFromContext(ctx))
	if v := appkit.CountryFromContext(ctx); v != "" {
		r = append(r, "country", v)
	}

	if v := appkit.PlatformFromContext(ctx); v != "" {
		r = append(r, "platform", v)
	}

	if v := appkit.VersionFromContext(ctx); v != "" {
		r = append(r, "version", v)
	}

	return r
}

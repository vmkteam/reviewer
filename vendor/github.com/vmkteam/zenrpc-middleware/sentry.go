package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/vmkteam/appkit"
	"github.com/vmkteam/zenrpc/v2"
)

// WithSentry sets additional parameters for current Sentry scope. Extras: params, duration, ip. Tags: platform,
// version, method. It's also handles panic.
func WithSentry(serverName string) zenrpc.MiddlewareFunc {
	return func(h zenrpc.InvokeFunc) zenrpc.InvokeFunc {
		return func(ctx context.Context, method string, params json.RawMessage) zenrpc.Response {
			defer func() {
				var err error
				var rec any
				if rec = recover(); rec != nil {
					switch e := rec.(type) {
					case error:
						err = e
					default:
						err = fmt.Errorf("%v", e)
					}
				}

				if hub := sentry.GetHubFromContext(ctx); hub != nil {
					start, platform, version, ip, xRequestID := time.Now(), appkit.PlatformFromContext(ctx), appkit.VersionFromContext(ctx), appkit.IPFromContext(ctx), appkit.XRequestIDFromContext(ctx)

					methodName := fullMethodName(serverName, zenrpc.NamespaceFromContext(ctx), method)

					hub.Scope().SetExtras(map[string]interface{}{
						"params":   params,
						"duration": time.Since(start).String(),
						"ip":       ip,
					})

					hub.Scope().SetTags(map[string]string{
						"platform":   platform,
						"version":    version,
						"method":     methodName,
						"xRequestId": xRequestID,
					})

					if err != nil {
						hub.CaptureException(err)
					}
				}
			}()

			return h(ctx, method, params)
		}
	}
}

// WithErrorLogger logs all errors (ErrorCode==500 or < 0) via Printf func and sends them to Sentry. It also removes
// sensitive error data from response. It is good to use pkg/errors for stack trace support in sentry.
func WithErrorLogger(pf Printf, serverName string) zenrpc.MiddlewareFunc {
	return func(h zenrpc.InvokeFunc) zenrpc.InvokeFunc {
		return func(ctx context.Context, method string, params json.RawMessage) zenrpc.Response {
			start, platform, version, ip, xRequestID := time.Now(), appkit.PlatformFromContext(ctx), appkit.VersionFromContext(ctx), appkit.IPFromContext(ctx), appkit.XRequestIDFromContext(ctx)
			namespace := zenrpc.NamespaceFromContext(ctx)

			r := h(ctx, method, params)
			if r.Error != nil && (r.Error.Code == http.StatusInternalServerError || r.Error.Code < 0) {
				duration := time.Since(start)
				methodName := fullMethodName(serverName, namespace, method)

				pf("ip=%s platform=%q version=%q method=%s duration=%v params=%s xRequestId=%q err=%q", ip, platform, version, methodName, duration, params, xRequestID, r.Error)

				// initialize hub and scope
				currentHub, scope := sentry.CurrentHub(), sentry.NewScope()

				// set hub and scope from context, if present
				if hub := sentry.GetHubFromContext(ctx); hub != nil {
					scope = hub.Scope()
					currentHub = hub
				}

				scope.SetExtras(map[string]interface{}{
					"params":     params,
					"duration":   duration.String(),
					"ip":         ip,
					"error.data": r.Error.Data,
					"error.code": r.Error.Code,
				})
				scope.SetTags(map[string]string{
					"platform":   platform,
					"version":    version,
					"method":     methodName,
					"xRequestId": xRequestID,
				})
				currentHub.CaptureException(r.Error)

				// remove sensitive error data from response
				r.Error.Err = nil
				r.Error.Message = "Internal error"
			}

			return r
		}
	}
}

// WithErrorSLog logs all errors (ErrorCode==500 or < 0) via [slog.ErrorContext] func and sends them to Sentry. It also removes
// sensitive error data from response. It is good to use pkg/errors for stack trace support in sentry.
func WithErrorSLog(pf Print, serverName string, fn LogAttrs) zenrpc.MiddlewareFunc {
	return func(h zenrpc.InvokeFunc) zenrpc.InvokeFunc {
		return func(ctx context.Context, method string, params json.RawMessage) zenrpc.Response {
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

			if r.Error != nil && (r.Error.Code == http.StatusInternalServerError || r.Error.Code < 0) {
				start, platform, version, ip, xRequestID := time.Now(), appkit.PlatformFromContext(ctx), appkit.VersionFromContext(ctx), appkit.IPFromContext(ctx), appkit.XRequestIDFromContext(ctx)
				namespace := zenrpc.NamespaceFromContext(ctx)

				duration := time.Since(start)
				methodName := fullMethodName(serverName, namespace, method)

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

				pf(ctx, "rpc error", append(logArgs, args...)...)

				// initialize hub and scope
				currentHub, scope := sentry.CurrentHub(), sentry.NewScope()

				// set hub and scope from context, if present
				if hub := sentry.GetHubFromContext(ctx); hub != nil {
					scope = hub.Scope()
					currentHub = hub
				}

				scope.SetExtras(map[string]interface{}{
					"params":     params,
					"duration":   duration.String(),
					"ip":         ip,
					"error.data": r.Error.Data,
					"error.code": r.Error.Code,
				})
				scope.SetTags(map[string]string{
					"platform":   platform,
					"version":    version,
					"method":     methodName,
					"xRequestId": xRequestID,
				})
				currentHub.CaptureException(r.Error)

				// remove sensitive error data from response
				r.Error.Err = nil
				r.Error.Message = "Internal error"
			}

			return r
		}
	}
}

package middleware

import (
	"context"
	"encoding/json"

	"github.com/labstack/echo/v4"
	"github.com/vmkteam/appkit"
	"github.com/vmkteam/zenrpc/v2"
)

const (
	// DefaultServerName is a global default name, mostly used for metrics.
	DefaultServerName = ""
)

type (
	Printf   func(format string, v ...interface{})
	Print    func(ctx context.Context, msg string, args ...any)
	LogAttrs func(ctx context.Context, r zenrpc.Response) []any
)

// WithDevel sets bool flag to context for detecting development environment.
func WithDevel(isDevel bool) zenrpc.MiddlewareFunc {
	return func(h zenrpc.InvokeFunc) zenrpc.InvokeFunc {
		return func(ctx context.Context, method string, params json.RawMessage) zenrpc.Response {
			ctx = appkit.NewIsDevelContext(ctx, isDevel)
			return h(ctx, method, params)
		}
	}
}

// WithNoCancelContext ignores Cancel func from context. This is useful for passing context to `go-pg`.
func WithNoCancelContext() zenrpc.MiddlewareFunc {
	return func(h zenrpc.InvokeFunc) zenrpc.InvokeFunc {
		return func(ctx context.Context, method string, params json.RawMessage) zenrpc.Response {
			return h(context.WithoutCancel(ctx), method, params)
		}
	}
}

// WithHeaders sets User-Agent, Platform, Version, X-Country headers to context. User-Agent strips to 2048 chars, Platform and Version – to 64, X-Country - to 16.
func WithHeaders() zenrpc.MiddlewareFunc {
	return func(h zenrpc.InvokeFunc) zenrpc.InvokeFunc {
		return func(ctx context.Context, method string, params json.RawMessage) zenrpc.Response {
			if req, ok := zenrpc.RequestFromContext(ctx); ok && req != nil {
				ctx = appkit.NewUserAgentContext(ctx, req.UserAgent())
				ctx = appkit.NewPlatformContext(ctx, req.Header.Get("Platform"))
				ctx = appkit.NewVersionContext(ctx, req.Header.Get("Version"))
				ctx = appkit.NewXRequestIDContext(ctx, req.Header.Get(echo.HeaderXRequestID))
				ctx = appkit.NewCountryContext(ctx, req.Header.Get("X-Country"))
				ctx = appkit.NewMethodContext(ctx, method)
			}
			return h(ctx, method, params)
		}
	}
}

// NewIPContext creates new context with IP.
// Deprecated: use appkit.NewIPContext.
func NewIPContext(ctx context.Context, ip string) context.Context {
	return appkit.NewIPContext(ctx, ip)
}

// IPFromContext returns IP from context.
// Deprecated: use appkit.IPFromContext.
func IPFromContext(ctx context.Context) string {
	return appkit.IPFromContext(ctx)
}

// NewUserAgentContext creates new context with User-Agent.
// Deprecated: use appkit.NewUserAgentContext.
func NewUserAgentContext(ctx context.Context, ua string) context.Context {
	return appkit.NewUserAgentContext(ctx, ua)
}

// UserAgentFromContext returns userAgent from context.
// Deprecated: use appkit.UserAgentFromContext.
func UserAgentFromContext(ctx context.Context) string {
	return appkit.UserAgentFromContext(ctx)
}

// NewNotificationContext creates new context with JSONRPC2 notification flag.
// Deprecated: use appkit.NewNotificationContext.
func NewNotificationContext(ctx context.Context) context.Context {
	return appkit.NewNotificationContext(ctx)
}

// NotificationFromContext returns JSONRPC2 notification flag from context.
// Deprecated: use appkit.NotificationFromContext.
func NotificationFromContext(ctx context.Context) bool {
	return appkit.NotificationFromContext(ctx)
}

// NewIsDevelContext creates new context with isDevel flag.
// Deprecated: use appkit.NewIsDevelContext.
func NewIsDevelContext(ctx context.Context, isDevel bool) context.Context {
	return appkit.NewIsDevelContext(ctx, isDevel)
}

// IsDevelFromContext returns isDevel flag from context.
// Deprecated: use appkit.IsDevelFromContext.
func IsDevelFromContext(ctx context.Context) bool {
	return appkit.IsDevelFromContext(ctx)
}

// NewPlatformContext creates new context with platform.
// Deprecated: use appkit.NewPlatformContext.
func NewPlatformContext(ctx context.Context, platform string) context.Context {
	return appkit.NewPlatformContext(ctx, platform)
}

// PlatformFromContext returns platform from context.
// Deprecated: use appkit.PlatformFromContext.
func PlatformFromContext(ctx context.Context) string {
	return appkit.PlatformFromContext(ctx)
}

// NewVersionContext creates new context with version.
// Deprecated: use appkit.NewVersionContext.
func NewVersionContext(ctx context.Context, version string) context.Context {
	return appkit.NewVersionContext(ctx, version)
}

// VersionFromContext returns version from context.
// Deprecated: use appkit.VersionFromContext.
func VersionFromContext(ctx context.Context) string {
	return appkit.VersionFromContext(ctx)
}

// NewCountryContext creates new context with country.
// Deprecated: use appkit.NewCountryContext.
func NewCountryContext(ctx context.Context, country string) context.Context {
	return appkit.NewCountryContext(ctx, country)
}

// CountryFromContext returns country from context.
// Deprecated: use appkit.CountryFromContext.
func CountryFromContext(ctx context.Context) string {
	return appkit.CountryFromContext(ctx)
}

// NewMethodContext creates new context with Method.
// Deprecated: use appkit.NewMethodContext.
func NewMethodContext(ctx context.Context, method string) context.Context {
	return appkit.NewMethodContext(ctx, method)
}

// MethodFromContext returns Method from context.
// Deprecated: use appkit.MethodFromContext.
func MethodFromContext(ctx context.Context) string {
	return appkit.MethodFromContext(ctx)
}

// fullMethodName returns namespace.method or serverName.namespace.method.
func fullMethodName(serverName, namespace, method string) string {
	name := namespace + "." + method
	if serverName != "" {
		name = serverName + "." + name
	}

	return name
}

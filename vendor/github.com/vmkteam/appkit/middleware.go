package appkit

import (
	"context"
)

const (
	isDevelCtx         contextKey = "isDevel"
	ctxPlatformKey     contextKey = "platform"
	ctxVersionKey      contextKey = "version"
	ctxMethodKey       contextKey = "method"
	ctxIPKey           contextKey = "ip"
	ctxUserAgentKey    contextKey = "userAgent"
	ctxCountryKey      contextKey = "country"
	ctxNotificationKey string     = "JSONRPC2-Notification"
	debugIDCtx         contextKey = "debugID"
	sqlGroupCtx        contextKey = "sqlGroup"

	maxUserAgentLength = 2048
	maxVersionLength   = 64
	maxCountryLength   = 16
	EmptyDebugID       = 0
)

// DebugIDFromContext returns debug ID from context.
func DebugIDFromContext(ctx context.Context) uint64 {
	if ctx == nil {
		return EmptyDebugID
	}

	if id, ok := ctx.Value(debugIDCtx).(uint64); ok {
		return id
	}

	return EmptyDebugID
}

// NewDebugIDContext creates new context with debug ID.
func NewDebugIDContext(ctx context.Context, debugID uint64) context.Context {
	return context.WithValue(ctx, debugIDCtx, debugID)
}

// NewSQLGroupContext creates new context with SQL Group for debug SQL logging.
func NewSQLGroupContext(ctx context.Context, group string) context.Context {
	groups, _ := ctx.Value(sqlGroupCtx).(string)
	if groups != "" {
		groups += ">"
	}
	groups += group
	return context.WithValue(ctx, sqlGroupCtx, groups)
}

// SQLGroupFromContext returns sql group from context.
func SQLGroupFromContext(ctx context.Context) string {
	r, _ := ctx.Value(sqlGroupCtx).(string)
	return r
}

// NewIPContext creates new context with IP.
func NewIPContext(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, ctxIPKey, ip)
}

// IPFromContext returns IP from context.
func IPFromContext(ctx context.Context) string {
	r, _ := ctx.Value(ctxIPKey).(string)
	return r
}

// NewUserAgentContext creates new context with User-Agent.
func NewUserAgentContext(ctx context.Context, ua string) context.Context {
	return context.WithValue(ctx, ctxUserAgentKey, cutString(ua, maxUserAgentLength))
}

// UserAgentFromContext returns userAgent from context.
func UserAgentFromContext(ctx context.Context) string {
	r, _ := ctx.Value(ctxUserAgentKey).(string)
	return r
}

// NewNotificationContext creates new context with JSONRPC2 notification flag.
func NewNotificationContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, ctxNotificationKey, true) //nolint:staticcheck
}

// NotificationFromContext returns JSONRPC2 notification flag from context.
func NotificationFromContext(ctx context.Context) bool {
	r, _ := ctx.Value(ctxNotificationKey).(bool)
	return r
}

// NewIsDevelContext creates new context with isDevel flag.
func NewIsDevelContext(ctx context.Context, isDevel bool) context.Context {
	return context.WithValue(ctx, isDevelCtx, isDevel)
}

// IsDevelFromContext returns isDevel flag from context.
func IsDevelFromContext(ctx context.Context) bool {
	if isDevel, ok := ctx.Value(isDevelCtx).(bool); ok {
		return isDevel
	}
	return false
}

// NewPlatformContext creates new context with platform.
func NewPlatformContext(ctx context.Context, platform string) context.Context {
	return context.WithValue(ctx, ctxPlatformKey, cutString(platform, 64))
}

// PlatformFromContext returns platform from context.
func PlatformFromContext(ctx context.Context) string {
	r, _ := ctx.Value(ctxPlatformKey).(string)
	return r
}

// NewVersionContext creates new context with version.
func NewVersionContext(ctx context.Context, version string) context.Context {
	return context.WithValue(ctx, ctxVersionKey, cutString(version, maxVersionLength))
}

// VersionFromContext returns version from context.
func VersionFromContext(ctx context.Context) string {
	r, _ := ctx.Value(ctxVersionKey).(string)
	return r
}

// NewCountryContext creates new context with country.
func NewCountryContext(ctx context.Context, country string) context.Context {
	return context.WithValue(ctx, ctxCountryKey, cutString(country, maxCountryLength))
}

// CountryFromContext returns country from context.
func CountryFromContext(ctx context.Context) string {
	r, _ := ctx.Value(ctxCountryKey).(string)
	return r
}

// NewMethodContext creates new context with Method.
func NewMethodContext(ctx context.Context, method string) context.Context {
	return context.WithValue(ctx, ctxMethodKey, method)
}

// MethodFromContext returns Method from context.
func MethodFromContext(ctx context.Context) string {
	r, _ := ctx.Value(ctxMethodKey).(string)
	return r
}

// cutString cuts string with given length.
func cutString(s string, length int) string {
	if len(s) > length {
		return s[:length]
	}
	return s
}

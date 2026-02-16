package middleware

import (
	"context"

	"github.com/vmkteam/appkit"
)

// XRequestIDFromContext returns X-Request-ID from context.
// Deprecated: use appkit.XRequestIDFromContext.
func XRequestIDFromContext(ctx context.Context) string {
	return appkit.XRequestIDFromContext(ctx)
}

// NewXRequestIDContext creates new context with X-Request-ID.
// Deprecated: use appkit.NewXRequestIDContext.
func NewXRequestIDContext(ctx context.Context, requestID string) context.Context {
	return appkit.NewXRequestIDContext(ctx, requestID)
}

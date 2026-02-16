package appkit

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"regexp"

	"github.com/labstack/echo/v4"
)

const (
	ctxXRequestIDKey string = echo.HeaderXRequestID
	headerXRequestID string = echo.HeaderXRequestID
)

var xRequestIDre = regexp.MustCompile(`[a-zA-Z0-9-]+`)

func generateXRequestID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func isValidXRequestID(requestID string) bool {
	return requestID != "" && len(requestID) <= 32 && xRequestIDre.MatchString(requestID)
}

// XRequestIDFromContext returns X-Request-ID from context.
func XRequestIDFromContext(ctx context.Context) string {
	r, _ := ctx.Value(ctxXRequestIDKey).(string)
	return r
}

// NewXRequestIDContext creates new context with X-Request-ID.
func NewXRequestIDContext(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, ctxXRequestIDKey, requestID) //nolint:staticcheck
}

package appkit

import (
	"context"
	"net/http"
	"slices"
	"strings"

	"github.com/getsentry/sentry-go"
	sentryecho "github.com/getsentry/sentry-go/echo"
	"github.com/labstack/echo/v4"
)

var (
	defaultHeaders = []string{
		"Authorization", "Authorization2", "Origin", "X-Requested-With", "Content-Type",
		"Accept", "Platform", "Version", "X-Request-ID",
	}
)

// CORS allows certain CORS headers.
func CORS(next http.Handler, headers ...string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", strings.Join(slices.Concat(defaultHeaders, headers), ", "))
		if r.Method == http.MethodOptions {
			return
		}

		next.ServeHTTP(w, r)
	})
}

// SetXRequestIDFromCtx adds X-Request-ID to request headers from context.
func SetXRequestIDFromCtx(ctx context.Context, req *http.Request) {
	xRequestID := XRequestIDFromContext(ctx)
	if xRequestID != "" && req.Header.Get(headerXRequestID) == "" {
		req.Header.Add(headerXRequestID, xRequestID)
	}
}

// XRequestID add X-Request-ID header if not exists.
func XRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get(headerXRequestID)
		if !isValidXRequestID(requestID) {
			requestID = generateXRequestID()
			r.Header.Add(echo.HeaderXRequestID, requestID)
		}
		w.Header().Set(echo.HeaderXRequestID, requestID)

		next.ServeHTTP(w, r)
	})
}

// EchoHandler is wrapper for Echo.
func EchoHandler(next http.Handler) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		ctx = applySentryHubToContext(ctx)
		ctx = applyIPToContext(ctx)
		req := ctx.Request()
		CORS(XRequestID(next)).ServeHTTP(ctx.Response(), req)
		return nil
	}
}

// EchoSentryHubContext middleware applies sentry hub to context for zenrpc middleware.
func EchoSentryHubContext() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			return next(applySentryHubToContext(c))
		}
	}
}

// EchoIPContext middleware applies client ip to context for zenrpc middleware.
func EchoIPContext() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			return next(applyIPToContext(c))
		}
	}
}

func applySentryHubToContext(c echo.Context) echo.Context {
	if hub := sentryecho.GetHubFromContext(c); hub != nil {
		req := c.Request()
		c.SetRequest(req.WithContext(sentry.SetHubOnContext(req.Context(), hub)))
	}
	return c
}

func applyIPToContext(c echo.Context) echo.Context {
	req := c.Request()
	c.SetRequest(req.WithContext(NewIPContext(req.Context(), c.RealIP())))
	return c
}

package reviewctl

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"

	"reviewsrv/pkg/db"
	"reviewsrv/pkg/reviewer"

	"github.com/vmkteam/embedlog"
	zm "github.com/vmkteam/zenrpc-middleware"
	"github.com/vmkteam/zenrpc/v2"
)

var (
	ErrInvalidProjectKey = zenrpc.NewStringError(http.StatusBadRequest, "invalid project key")
	ErrNoRunnerProfile   = zenrpc.NewStringError(http.StatusNotFound, "no runner profile for project (and no default)")
)

func newInternalError(err error) *zenrpc.Error {
	return zenrpc.NewError(http.StatusInternalServerError, err)
}

var allowDebugFn = func() zm.AllowDebugFunc {
	return func(req *http.Request) bool {
		return req != nil && req.FormValue("__level") == "5"
	}
}

//go:generate go tool zenrpc

// New returns the internal reviewctl zenrpc Server. It is mounted only on the
// network-internal /v1/reviewctl/rpc/ path; its ReviewConfig method returns the
// real profile token, so it must never be folded into the public RPC server.
func New(dbo db.DB, logger embedlog.Logger, isDevel bool) *zenrpc.Server {
	rpc := zenrpc.NewServer(zenrpc.Options{
		ExposeSMD: true,
		AllowCORS: true,
	})

	rpc.Use(
		zm.WithDevel(isDevel),
		zm.WithHeaders(),
		zm.WithSentry(zm.DefaultServerName),
		zm.WithNoCancelContext(),
		zm.WithMetrics("reviewctl"),
		zm.WithTiming(isDevel, allowDebugFn()),
		zm.WithSQLLogger(dbo.DB, isDevel, allowDebugFn(), allowDebugFn()),
	)

	pf := maskKeys(logger.Print)
	rpc.Use(
		zm.WithSLog(pf, zm.DefaultServerName, nil),
		zm.WithErrorSLog(pf, zm.DefaultServerName, nil),
	)

	// services
	rpc.RegisterAll(map[string]zenrpc.Invoker{
		"reviewctl": NewService(dbo),
	})

	return rpc
}

// maskKeys wraps the printer of the RPC log middlewares, which log each call's
// raw params: every method here takes the project key.
func maskKeys(pf zm.Print) zm.Print {
	return func(ctx context.Context, msg string, args ...any) {
		args = slices.Clone(args)
		for i := 1; i < len(args); i += 2 {
			if args[i-1] != "params" {
				continue
			}
			if p, ok := args[i].(json.RawMessage); ok {
				args[i] = json.RawMessage(reviewer.MaskKeys(string(p)))
			}
		}
		pf(ctx, msg, args...)
	}
}

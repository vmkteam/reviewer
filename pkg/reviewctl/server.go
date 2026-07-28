package reviewctl

import (
	"net/http"

	"reviewsrv/pkg/db"

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

	rpc.Use(
		zm.WithSLog(logger.Print, zm.DefaultServerName, nil),
		zm.WithErrorSLog(logger.Print, zm.DefaultServerName, nil),
	)

	// services
	rpc.RegisterAll(map[string]zenrpc.Invoker{
		"reviewctl": NewService(dbo),
	})

	return rpc
}

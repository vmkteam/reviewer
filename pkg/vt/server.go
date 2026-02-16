package vt

import (
	"net/http"

	"reviewsrv/pkg/db"

	"github.com/vmkteam/embedlog"
	zm "github.com/vmkteam/zenrpc-middleware"
	"github.com/vmkteam/zenrpc/v2"
)

//go:generate go tool zenrpc

const (
	AuthKey = "Authorization2"
)

const (
	NSAuth         = "auth"
	NSUser         = "user"
	NSProject      = "project"
	NSPrompt       = "prompt"
	NSSlackChannel = "slackChannel"
	NSTaskTracker  = "taskTracker"
)

var (
	ErrUnauthorized   = httpAsRPCError(http.StatusUnauthorized)
	ErrForbidden      = httpAsRPCError(http.StatusForbidden)
	ErrNotFound       = httpAsRPCError(http.StatusNotFound)
	ErrInternal       = httpAsRPCError(http.StatusInternalServerError)
	ErrNotImplemented = httpAsRPCError(http.StatusNotImplemented)
)

var allowDebugFn = func() zm.AllowDebugFunc {
	return func(req *http.Request) bool {
		return req != nil && req.FormValue("__level") == "5"
	}
}

func httpAsRPCError(code int) *zenrpc.Error {
	return zenrpc.NewStringError(code, http.StatusText(code))
}

// New returns new zenrpc Server.
func New(dbo db.DB, logger embedlog.Logger, isDevel bool, baseURL string) *zenrpc.Server {
	rpc := zenrpc.NewServer(zenrpc.Options{
		ExposeSMD: true,
		AllowCORS: true,
	})

	commonRepo := db.NewCommonRepo(dbo)

	// middleware
	rpc.Use(
		zm.WithHeaders(),
		zm.WithDevel(isDevel),
		zm.WithNoCancelContext(),
		zm.WithMetrics("vt"),
		zm.WithSLog(logger.Print, zm.DefaultServerName, nil),
		zm.WithErrorSLog(logger.Error, zm.DefaultServerName, nil),
		zm.WithSQLLogger(dbo.DB, isDevel, allowDebugFn(), allowDebugFn()),
		zm.WithTiming(isDevel, allowDebugFn()),
		zm.WithSentry(zm.DefaultServerName),
		authMiddleware(&commonRepo, logger),
	)

	// services
	rpc.RegisterAll(map[string]zenrpc.Invoker{
		NSAuth:         NewAuthService(dbo, logger),
		NSUser:         NewUserService(dbo, logger),
		NSProject:      NewProjectService(dbo, logger, baseURL),
		NSPrompt:       NewPromptService(dbo, logger),
		NSSlackChannel: NewSlackChannelService(dbo, logger),
		NSTaskTracker:  NewTaskTrackerService(dbo, logger),
	})

	return rpc
}

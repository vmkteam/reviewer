package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-pg/pg/v10"
	"github.com/vmkteam/appkit"
	"github.com/vmkteam/zenrpc/v2"
)

const (
	eventStartedAt = "queryStartedAt"
)

type AllowDebugFunc func(*http.Request) bool

// DebugIDFromContext returns debug id from context.
// Deprecated: use appkit.DebugIDFromContext.
func DebugIDFromContext(ctx context.Context) uint64 {
	return appkit.DebugIDFromContext(ctx)
}

// NewDebugIDContext creates new context with debug ID.
// Deprecated: use appkit.NewDebugIDContext.
func NewDebugIDContext(ctx context.Context, debugID uint64) context.Context {
	return appkit.NewDebugIDContext(ctx, debugID)
}

// NewSQLGroupContext creates new context with SQL Group for debug SQL logging.
// Deprecated: use appkit.NewSQLGroupContext.
func NewSQLGroupContext(ctx context.Context, group string) context.Context {
	return appkit.NewSQLGroupContext(ctx, group)
}

// SQLGroupFromContext returns sql group from context.
// Deprecated: use appkit.SQLGroupFromContext.
func SQLGroupFromContext(ctx context.Context) string {
	return appkit.SQLGroupFromContext(ctx)
}

// WithTiming adds timings in JSON-RPC 2.0 Response via `extensions` field (not in spec).
// Middleware is active when `isDevel=true` or AllowDebugFunc returns `true` and http request is set.
// `DurationLocal` – total method execution time in ms.
// If `DurationRemote` or `DurationDiff` are set then `DurationLocal` excludes these values.
func WithTiming(isDevel bool, allowDebugFunc AllowDebugFunc) zenrpc.MiddlewareFunc {
	return func(h zenrpc.InvokeFunc) zenrpc.InvokeFunc {
		return func(ctx context.Context, method string, params json.RawMessage) zenrpc.Response {
			// check for debug id
			if !isDevel {
				req, ok := zenrpc.RequestFromContext(ctx)
				if !ok || req == nil {
					return h(ctx, method, params)
				}

				reqClone := req.Clone(ctx)
				if reqClone == nil || !allowDebugFunc(reqClone) {
					return h(ctx, method, params)
				}
			}

			now := time.Now()

			resp := h(ctx, method, params)
			if resp.Extensions == nil {
				resp.Extensions = make(map[string]interface{})
			}

			total := int64(time.Since(now) / 1e6) // .Milliseconds() 1.13
			if remote, ok := resp.Extensions["DurationRemote"]; ok {
				total -= remote.(int64) //nolint:errcheck // must be int64
			}
			if diff, ok := resp.Extensions["DurationDiff"]; ok {
				total -= diff.(int64) //nolint:errcheck // must be int64
			}

			// detect remote only duration
			if resp.Extensions["DurationLocal"] != -1 {
				resp.Extensions["DurationLocal"] = total
			}

			return resp
		}
	}
}

// WithSQLLogger adds `SQL` or `DurationSQL` fields in JSON-RPC 2.0 Response `extensions` field (not in spec).
// `DurationSQL` field is set then `isDevel=true` or AllowDebugFunc(allowDebugFunc) returns `true` and http request is set.
// `SQL` field is set then `isDevel=true` or AllowDebugFunc(allowDebugFunc, allowSQLDebugFunc) returns `true` and http request is set.
func WithSQLLogger(db *pg.DB, isDevel bool, allowDebugFunc, allowSQLDebugFunc AllowDebugFunc) zenrpc.MiddlewareFunc {
	// init sql logger
	ql := NewSQLQueryLogger()
	db.AddQueryHook(ql)

	return func(h zenrpc.InvokeFunc) zenrpc.InvokeFunc {
		return func(ctx context.Context, method string, params json.RawMessage) zenrpc.Response {
			logQuery := true

			// check for debug id
			if !isDevel {
				req, ok := zenrpc.RequestFromContext(ctx)
				if !ok || req == nil {
					return h(ctx, method, params)
				}

				reqClone := req.Clone(ctx)
				if reqClone == nil || !allowDebugFunc(reqClone) {
					return h(ctx, method, params)
				}

				if !allowSQLDebugFunc(reqClone) {
					logQuery = false
				}
			}

			debugID := ql.NextID()
			ctx = appkit.NewDebugIDContext(ctx, debugID)
			ql.Push(debugID)

			resp := h(ctx, method, params)
			if resp.Extensions == nil {
				resp.Extensions = make(map[string]interface{})
			}

			qq := ql.Pop(debugID)

			// calculate total duration
			var totalSQL time.Duration
			for i := range qq {
				totalSQL += qq[i].Duration.Duration
			}
			// set sql and duration to extensions
			if len(qq) > 0 {
				if logQuery {
					resp.Extensions["SQL"] = qq
				}
				resp.Extensions["DurationSQL"] = int64(totalSQL / 1e6)
			}

			return resp
		}
	}
}

type sqlQueryLogger struct {
	nextID uint64
	data   map[uint64][]sqlQuery
	dataMu *sync.Mutex
}

type sqlQuery struct {
	Query    string
	Group    string
	Duration Duration
}

type Duration struct {
	time.Duration
}

func (d Duration) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`"%s"`, d.Round(time.Millisecond).String())), nil
}

func NewSQLQueryLogger() *sqlQueryLogger {
	return &sqlQueryLogger{
		data:   make(map[uint64][]sqlQuery),
		dataMu: &sync.Mutex{},
	}
}

func (ql *sqlQueryLogger) BeforeQuery(ctx context.Context, event *pg.QueryEvent) (context.Context, error) {
	if event.Stash == nil {
		event.Stash = make(map[interface{}]interface{})
	}

	if appkit.DebugIDFromContext(ctx) != appkit.EmptyDebugID {
		event.Stash[eventStartedAt] = time.Now()
	}

	return ctx, nil
}

func (ql *sqlQueryLogger) AfterQuery(ctx context.Context, event *pg.QueryEvent) error {
	debugID := appkit.DebugIDFromContext(ctx)
	if debugID == appkit.EmptyDebugID {
		return nil
	}

	// get query
	query, err := event.FormattedQuery()
	if err != nil {
		return fmt.Errorf("formatted query failed: %w", err)
	}
	sq := sqlQuery{Query: string(query)}

	// calculate duration
	if event.Stash != nil {
		if v, ok := event.Stash[eventStartedAt]; ok {
			if startAt, k := v.(time.Time); k {
				sq.Duration = Duration{Duration: time.Since(startAt)}
			}
		}
	}

	sq.Group = strings.Trim(appkit.SQLGroupFromContext(ctx), ">")

	ql.Store(debugID, sq)

	return nil
}

// Push is a function that init capturing session for debug ID.
func (ql *sqlQueryLogger) Push(debugID uint64) {
	ql.dataMu.Lock()
	defer ql.dataMu.Unlock()

	ql.data[debugID] = []sqlQuery{}
}

// Store saves sql query for debug ID.
func (ql *sqlQueryLogger) Store(debugID uint64, sq sqlQuery) {
	ql.dataMu.Lock()
	defer ql.dataMu.Unlock()

	// skip unknown queries
	if _, ok := ql.data[debugID]; !ok {
		return
	}

	ql.data[debugID] = append(ql.data[debugID], sq)
}

// Pop returns all sql queries for debugID and removes from store.
func (ql *sqlQueryLogger) Pop(debugID uint64) []sqlQuery {
	ql.dataMu.Lock()
	defer ql.dataMu.Unlock()

	qq, ok := ql.data[debugID]
	if ok {
		delete(ql.data, debugID)
	}

	return qq
}

// NextID returns next debug ID.
func (ql *sqlQueryLogger) NextID() uint64 {
	return atomic.AddUint64(&ql.nextID, 1)
}

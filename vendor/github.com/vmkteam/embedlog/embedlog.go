package embedlog

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"reflect"
	"runtime"
	"time"

	"github.com/lmittmann/tint"
	"github.com/prometheus/client_golang/prometheus"
)

// statEvents is a global prometheus events for all loggers in apps (app_log_events_total).
var statEvents = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "app",
	Subsystem: "log",
	Name:      "events_total",
	Help:      "Log events distributions.",
}, []string{"type"})

// init registers statEvents.
func init() {
	prometheus.MustRegister(statEvents)
}

type splitLevelHandler struct {
	out slog.Handler
	err slog.Handler
}

// newSplitLevelHandler returns new splitLevel handlers by stdout and stderr.
func newSplitLevelHandler(stdout, stderr io.Writer, opts *slog.HandlerOptions, isJSON bool) *splitLevelHandler {
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}

	if isJSON {
		return &splitLevelHandler{
			out: slog.NewJSONHandler(stdout, opts),
			err: slog.NewJSONHandler(stderr, opts),
		}
	}

	return &splitLevelHandler{
		out: slog.NewTextHandler(stdout, opts),
		err: slog.NewTextHandler(stderr, opts),
	}
}

// Enabled reports whether the handler handles records at the given level.
func (h *splitLevelHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.out.Enabled(ctx, level) || h.err.Enabled(ctx, level)
}

// Handle methods that produce output should observe the following rules in [slog.Handler].
func (h *splitLevelHandler) Handle(ctx context.Context, r slog.Record) error {
	if r.Level >= slog.LevelError {
		return h.err.Handle(ctx, r)
	}
	return h.out.Handle(ctx, r)
}

// WithAttrs returns a new Handler whose attributes consist of
// both the receiver's attributes and the arguments.
func (h *splitLevelHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &splitLevelHandler{
		out: h.out.WithAttrs(attrs),
		err: h.err.WithAttrs(attrs),
	}
}

// WithGroup returns a new Handler with the given group appended to
// the receiver's existing groups.
func (h *splitLevelHandler) WithGroup(name string) slog.Handler {
	return &splitLevelHandler{
		out: h.out.WithGroup(name),
		err: h.err.WithGroup(name),
	}
}

// Logger is a struct for embedding std loggers.
type Logger struct {
	slog *slog.Logger
}

// NewDevLogger returns colored dev logger.
func NewDevLogger() Logger {
	return Logger{
		slog: slog.New(tint.NewHandler(os.Stdout, &tint.Options{
			AddSource: true,
			Level:     slog.LevelDebug,
			ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
				// colorize errors
				if err, ok := a.Value.Any().(error); ok {
					if reflect.ValueOf(err).IsValid() && !isNil(err) {
						aErr := tint.Err(err)
						aErr.Key = a.Key
						return aErr
					}
				}

				// show source
				if a.Key == slog.SourceKey {
					var file string
					s, ok := a.Value.Any().(*slog.Source)
					if ok {
						file = fmt.Sprintf("%s:%d", path.Base(s.File), s.Line)
						a = slog.String("@source", file+"\t | ")
					}
				}

				if v, ok := a.Value.Any().(json.RawMessage); ok {
					a.Value = slog.StringValue(string(v))
				}

				return a
			},
		})),
	}
}

// NewLogger returns new Logger wrapper for slog.
// verbose sets [slog.LevelInfo] instead of  [slog.LevelError].
// isJson uses [slog.JSONHandler] instead of [slog.TextHandler].
func NewLogger(verbose, isJSON bool) Logger {
	// set Level
	level := slog.LevelError
	if verbose {
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: true,
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.SourceKey {
				var file string
				s, ok := a.Value.Any().(*slog.Source)
				if ok {
					file = fmt.Sprintf("%s:%d", path.Base(s.File), s.Line)
					a = slog.String("@source", file)
				}
			}

			if !isJSON && a.Key == slog.TimeKey {
				t := a.Value.Time().Format(time.DateTime)
				a.Value = slog.StringValue(t)
			}

			return a
		},
	}

	return Logger{
		slog: slog.New(newSplitLevelHandler(os.Stdout, os.Stderr, opts, isJSON)),
	}
}

// isNul checks for nil interface.
func isNil(i any) bool {
	if i == nil {
		return true
	}

	//nolint:exhaustive // we need to check only this types
	switch reflect.TypeOf(i).Kind() {
	case reflect.Ptr, reflect.Map, reflect.Array, reflect.Chan, reflect.Slice:
		return reflect.ValueOf(i).IsNil()
	default:
		return false
	}
}

// Printf logs at [slog.LevelInfo] with the given context with [fmt.Sprintf].
func (l Logger) Printf(format string, v ...any) {
	l.logCtx(context.Background(), slog.LevelInfo, fmt.Sprintf(format, v...))
}

// Print logs at [slog.LevelInfo] with the given context.
func (l Logger) Print(ctx context.Context, msg string, args ...any) {
	l.logCtx(ctx, slog.LevelInfo, msg, args...)
}

// Errorf logs at [slog.LevelError] with the given context with [fmt.Sprintf].
func (l Logger) Errorf(format string, v ...any) {
	l.logCtx(context.Background(), slog.LevelError, fmt.Sprintf(format, v...))
}

// Error logs at [slog.LevelError] with the given context.
func (l Logger) Error(ctx context.Context, msg string, args ...any) {
	l.logCtx(ctx, slog.LevelError, msg, args...)
}

// PrintOrErr logs an Error if err is not nil, otherwise logs an Info message with args.
func (l Logger) PrintOrErr(ctx context.Context, msg string, err error, args ...any) {
	if err == nil {
		l.logCtx(ctx, slog.LevelInfo, msg, args...)
	} else {
		l.logCtx(ctx, slog.LevelError, msg, "err", err)
	}
}

// With returns a Logger that includes the given attributes
// in each output operation. Arguments are converted to
// attributes as if by [Logger.Log].
func (l Logger) With(args ...any) Logger {
	if l.slog == nil {
		return l
	}

	return Logger{slog: l.slog.With(args...)}
}

// WithGroup returns a Logger that starts a group, if name is non-empty.
// The keys of all attributes added to the Logger will be qualified by the given
// name. (How that qualification happens depends on the [Handler.WithGroup]
// method of the Logger's Handler.)
//
// If name is empty, WithGroup returns the receiver.
func (l Logger) WithGroup(name string) Logger {
	if l.slog == nil {
		return l
	}

	return Logger{slog: l.slog.WithGroup(name)}
}

// Log is a function that returns underlying slog.Logger.
func (l Logger) Log() *slog.Logger {
	return l.slog
}

// logCtx is logging function that wraps slog.
// The log record contains the source position of the caller of Infof/Errorf and increase Prometheus counter metrics.
func (l Logger) logCtx(ctx context.Context, level slog.Level, msg string, args ...any) {
	if l.slog == nil {
		return
	}

	if !l.slog.Enabled(ctx, level) {
		return
	}

	// inc stat for level.
	statEvents.WithLabelValues(level.String()).Inc()

	// set correct file and line, handle
	var pcs [1]uintptr
	runtime.Callers(3, pcs[:]) // skip [Callers, Print/Error]

	r := slog.NewRecord(time.Now(), level, msg, pcs[0])
	r.Add(args...)
	_ = l.slog.Handler().Handle(ctx, r)
}

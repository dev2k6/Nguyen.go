// Package observe provides structured logging and request-scoped
// telemetry for Nguyen.go applications.
//
// It builds on Go 1.21+'s log/slog package: every request gets a
// per-request logger that automatically carries trace_id, request_id,
// and any other values registered through ngctx. Handlers retrieve the
// logger via observe.Logger(ctx) and emit structured events without
// having to rebuild the attribute set every time.
package observe

import (
	"context"
	"io"
	"log/slog"
	"os"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/dev2k6/Nguyen.go/pkg/ngctx"
)

type loggerKey struct{}

// Default returns the framework's root logger. Applications may override
// it with SetDefault before any requests are served.
var defaultLogger atomic.Pointer[slog.Logger]

func init() {
	defaultLogger.Store(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))
}

// SetDefault swaps the framework's root logger. Pass nil to reset to the
// init-time default.
func SetDefault(l *slog.Logger) {
	if l == nil {
		l = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))
	}
	defaultLogger.Store(l)
}

// Default returns the current root logger.
func Default() *slog.Logger {
	return defaultLogger.Load()
}

// NewJSON builds a JSON slog handler suitable for production. Use it as
// the argument to SetDefault when running behind log aggregators
// (Datadog, Loki, GCP Logging) that expect structured input.
func NewJSON(w io.Writer, level slog.Level) *slog.Logger {
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level}))
}

// WithLogger attaches a logger to ctx. Subsequent calls to Logger(ctx)
// return this logger; if not set, Logger(ctx) returns Default with
// trace/request attributes derived from ngctx.
func WithLogger(ctx context.Context, l *slog.Logger) context.Context {
	if l == nil {
		return ctx
	}
	return context.WithValue(ctx, loggerKey{}, l)
}

// Logger returns the request-scoped logger. If no logger was attached,
// Default() is enriched with trace_id, request_id, locale and user_id
// (when available) before being returned.
func Logger(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(loggerKey{}).(*slog.Logger); ok && l != nil {
		return l
	}
	l := Default()
	var attrs []any
	if v := ngctx.TraceID(ctx); v != "" {
		attrs = append(attrs, slog.String("trace_id", v))
	}
	if v := ngctx.RequestID(ctx); v != "" {
		attrs = append(attrs, slog.String("request_id", v))
	}
	if v := ngctx.Locale(ctx); v != "" {
		attrs = append(attrs, slog.String("locale", v))
	}
	if u := ngctx.User(ctx); u != nil && u.ID != "" {
		attrs = append(attrs, slog.String("user_id", u.ID))
	}
	if len(attrs) == 0 {
		return l
	}
	return l.With(attrs...)
}

// Span is a lightweight tracing span — start with StartSpan, end with
// span.End(). It logs a structured event when ended so a request's
// timeline can be reconstructed from logs alone (without an external
// tracing backend). Applications that already export OpenTelemetry can
// ignore this and use otel directly.
type Span struct {
	ctx    context.Context
	name   string
	start  time.Time
	attrs  []slog.Attr
	logger *slog.Logger
}

// StartSpan begins a new span. Spans are not nested in ctx — they are a
// timing+logging convenience, not a full tracer.
func StartSpan(ctx context.Context, name string, attrs ...slog.Attr) *Span {
	return &Span{
		ctx:    ctx,
		name:   name,
		start:  time.Now(),
		attrs:  attrs,
		logger: Logger(ctx),
	}
}

// SetAttr attaches an attribute that is recorded when the span ends.
func (s *Span) SetAttr(key string, value any) {
	s.attrs = append(s.attrs, slog.Any(key, value))
}

// End records the span duration and emits a structured log event at
// info level. Pass an error to record a failed span at error level.
func (s *Span) End(err error) {
	dur := time.Since(s.start)
	attrs := make([]any, 0, len(s.attrs)+3)
	attrs = append(attrs,
		slog.String("span", s.name),
		slog.Duration("duration", dur),
	)
	for _, a := range s.attrs {
		attrs = append(attrs, a)
	}
	if err != nil {
		attrs = append(attrs, slog.String("error", err.Error()))
		s.logger.LogAttrs(s.ctx, slog.LevelError, "span", attrSlice(attrs)...)
		return
	}
	s.logger.LogAttrs(s.ctx, slog.LevelInfo, "span", attrSlice(attrs)...)
}

func attrSlice(in []any) []slog.Attr {
	out := make([]slog.Attr, 0, len(in))
	for _, v := range in {
		if a, ok := v.(slog.Attr); ok {
			out = append(out, a)
		}
	}
	return out
}

// Caller returns "file:line" for the caller `skip` frames above. Useful
// when adding source markers to logs without paying for full slog source
// locations on every event.
func Caller(skip int) string {
	_, file, line, ok := runtime.Caller(skip + 1)
	if !ok {
		return "?"
	}
	short := file
	for i := len(file) - 1; i > 0; i-- {
		if file[i] == '/' || file[i] == '\\' {
			short = file[i+1:]
			break
		}
	}
	return short + ":" + itoa(line)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

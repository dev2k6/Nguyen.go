// Package ngctx defines the standard context keys and helpers Nguyen.go
// uses to thread request-scoped values through loaders, actions and
// middleware.
//
// Go's context.Context is the framework's first-class request scope —
// rather than passing the Fiber Ctx everywhere, handlers receive a
// context.Context and read typed values from it via this package. The
// keys are unexported sentinel types so application code cannot collide
// with the framework's keys; access goes through the typed helpers.
package ngctx

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"
)

// User describes the authenticated principal attached to a request.
// Auth middleware populates it; loaders and actions read it via User().
type UserInfo struct {
	ID    string
	Email string
	Roles []string
	// Extra is for application-specific data such as tenant id or
	// session-scoped flags. Keys should be lowercase ASCII.
	Extra map[string]string
}

// HasRole reports whether the user carries the given role.
func (u *UserInfo) HasRole(name string) bool {
	if u == nil {
		return false
	}
	for _, r := range u.Roles {
		if r == name {
			return true
		}
	}
	return false
}

type ctxKey int

const (
	keyTraceID ctxKey = iota
	keyRequestID
	keyLocale
	keyUser
	keyDeadline
)

// WithTraceID attaches a trace identifier to ctx. Trace IDs propagate to
// downstream services via the X-Trace-Id header by default.
func WithTraceID(ctx context.Context, id string) context.Context {
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, keyTraceID, id)
}

// TraceID returns the trace ID stored on ctx, or "" if none.
func TraceID(ctx context.Context) string {
	v, _ := ctx.Value(keyTraceID).(string)
	return v
}

// WithRequestID attaches a per-request identifier to ctx.
func WithRequestID(ctx context.Context, id string) context.Context {
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, keyRequestID, id)
}

// RequestID returns the request ID stored on ctx, or "" if none.
func RequestID(ctx context.Context) string {
	v, _ := ctx.Value(keyRequestID).(string)
	return v
}

// WithLocale attaches a BCP-47 locale tag to ctx (e.g. "en-US", "vi-VN").
func WithLocale(ctx context.Context, locale string) context.Context {
	if locale == "" {
		return ctx
	}
	return context.WithValue(ctx, keyLocale, locale)
}

// Locale returns the locale stored on ctx, or "" if none.
func Locale(ctx context.Context) string {
	v, _ := ctx.Value(keyLocale).(string)
	return v
}

// WithUser attaches the authenticated user to ctx.
func WithUser(ctx context.Context, u *UserInfo) context.Context {
	if u == nil {
		return ctx
	}
	return context.WithValue(ctx, keyUser, u)
}

// User returns the authenticated user attached to ctx, or nil.
func User(ctx context.Context) *UserInfo {
	v, _ := ctx.Value(keyUser).(*UserInfo)
	return v
}

// WithBudget attaches an absolute deadline marker to ctx for use by the
// observability layer. This does not replace context.WithDeadline — call
// WithDeadline as well; this value is metadata for telemetry.
func WithBudget(ctx context.Context, t time.Time) context.Context {
	return context.WithValue(ctx, keyDeadline, t)
}

// Budget returns the deadline marker attached to ctx, or zero time.
func Budget(ctx context.Context) time.Time {
	v, _ := ctx.Value(keyDeadline).(time.Time)
	return v
}

// NewID returns a 16-byte random hex identifier suitable for trace and
// request IDs. It uses crypto/rand and never panics — on failure it
// returns the empty string so callers can fall back gracefully.
func NewID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return ""
	}
	return hex.EncodeToString(b[:])
}

// FromHTTP populates a context with trace + request IDs read from the
// supplied request headers, generating new ones if absent. Locale is
// read from the Accept-Language header's first preference.
func FromHTTP(ctx context.Context, h http.Header) context.Context {
	tid := h.Get("X-Trace-Id")
	if tid == "" {
		tid = NewID()
	}
	rid := h.Get("X-Request-Id")
	if rid == "" {
		rid = NewID()
	}
	ctx = WithTraceID(ctx, tid)
	ctx = WithRequestID(ctx, rid)
	if al := h.Get("Accept-Language"); al != "" {
		ctx = WithLocale(ctx, primaryLanguage(al))
	}
	return ctx
}

// primaryLanguage parses an Accept-Language header value and returns the
// first language tag without quality. Falls back to the entire string if
// it cannot find a comma-separated tag.
func primaryLanguage(al string) string {
	for i := 0; i < len(al); i++ {
		switch al[i] {
		case ',', ';', ' ':
			return al[:i]
		}
	}
	return al
}

# Context & Observability

Nguyen.go threads request-scoped values through `context.Context` and
emits structured logs with attributes derived from the same context.
Two packages cooperate: `pkg/ngctx` defines the standard keys and
`pkg/observe` builds slog-based logging on top of them.

```go
import (
    "github.com/dev2k6/Nguyen.go/pkg/ngctx"
    "github.com/dev2k6/Nguyen.go/pkg/observe"
)
```

## Why context

Loaders, actions and middleware are plain Go functions that take a
`context.Context` as their first argument. Anything request-scoped
(trace ID, request ID, locale, authenticated user, deadline) lives on
the context, not on a Fiber-specific `Locals` map. This keeps handlers
testable in isolation and lets cross-service calls propagate the
same identifiers automatically.

## Standard keys

```go
ctx = ngctx.WithTraceID(ctx, "...")
ctx = ngctx.WithRequestID(ctx, "...")
ctx = ngctx.WithLocale(ctx, "vi-VN")
ctx = ngctx.WithUser(ctx, &ngctx.UserInfo{ID: "u1", Roles: []string{"admin"}})
ctx = ngctx.WithBudget(ctx, deadline)

tid := ngctx.TraceID(ctx)
user := ngctx.User(ctx)
if user.HasRole("admin") { ... }
```

`UserInfo` carries `ID`, `Email`, `Roles []string` and an optional
`Extra map[string]string` for tenant or session metadata.

`ngctx.NewID()` returns a 32-character hex identifier suitable for
trace and request IDs.

## Populating context from a request

`ngctx.FromHTTP(ctx, headers)` reads `X-Trace-Id`, `X-Request-Id` and
`Accept-Language` from a `http.Header`, generating new IDs when
missing. Application code rarely calls this directly — the bundled
Fiber middleware does it.

The framework's `serve()` automatically registers two pieces of
middleware:

- `server.ContextMiddleware()` — populates the user context with
  trace/request IDs and locale, sets the same IDs on response headers.
- `server.AccessLog()` — emits one structured slog event per request
  with method, path, status, duration and the IDs.

You see them in action by running any project — every response has
`X-Trace-Id` and `X-Request-Id` headers and every request prints
one log line.

## Structured logging

```go
log := observe.Logger(ctx)
log.Info("checkout.completed", "order_id", id, "amount_cents", total)
```

`observe.Logger(ctx)` returns the request-scoped logger. If no logger
was attached explicitly, it returns the default logger enriched with
`trace_id`, `request_id`, `locale` and `user_id` (when present).

Override the default logger at startup, e.g. for JSON output to
Datadog/Loki/GCP Logging:

```go
observe.SetDefault(observe.NewJSON(os.Stderr, slog.LevelInfo))
```

To attach a custom logger to a single request scope:

```go
ctx = observe.WithLogger(ctx, baseLogger.With("tenant", t))
```

## Spans

A lightweight timing helper that emits a structured event when
`End` is called. Useful for profiling loader steps without an external
tracing backend.

```go
sp := observe.StartSpan(ctx, "load.posts")
posts, err := db.Query(ctx, "SELECT ...")
sp.SetAttr("count", len(posts))
sp.End(err)
```

Output (text format):

```
level=INFO msg=span span=load.posts duration=22ms count=12 trace_id=...
```

Failed spans (`End(err)` with non-nil err) emit at error level with an
`error` attribute. Spans are not nested into context — they are a
timing+logging convenience, not a full distributed tracer. Applications
that already export OpenTelemetry can ignore Spans and use otel
directly while still benefiting from `Logger(ctx)`.

## Patterns

### Per-request logger ready to log

Inside a loader, you do not have to rebuild attributes:

```go
func Load(ctx context.Context, params Params) (PageData, error) {
    log := observe.Logger(ctx)
    log.Info("load.start", "id", params["id"])
    ...
}
```

The trace and request IDs are already on `log` because `ContextMiddleware`
attached them upstream.

### Cross-service trace propagation

When calling another service, forward `X-Trace-Id` and `X-Request-Id`:

```go
req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
req.Header.Set("X-Trace-Id", ngctx.TraceID(ctx))
req.Header.Set("X-Request-Id", ngctx.NewID())
```

The downstream Nguyen.go service reads these headers via
`ContextMiddleware` and the trees stitch together in your log
aggregator.

### Auth middleware populates the user

```go
func Auth(ctx context.Context, c *fiber.Ctx) error {
    user, err := decodeJWT(c.Cookies("session"))
    if err != nil {
        return c.SendStatus(401)
    }
    c.SetUserContext(ngctx.WithUser(c.UserContext(), &ngctx.UserInfo{
        ID:    user.ID,
        Email: user.Email,
        Roles: user.Roles,
    }))
    return c.Next()
}
```

Loaders downstream can read `ngctx.User(ctx)` and trust the value.

## See also

- `docs/concurrency.md` — Pool, Race, Parallel, Lifecycle
- `docs/cli.md` — `nguyen doctor`, `nguyen vet`, `nguyen generate`,
  `nguyen dist`

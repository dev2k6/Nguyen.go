# Concurrency Primitives

Nguyen.go applications run on Go's CSP runtime — goroutines are cheap,
channels are typed, and `context.Context` cancels in a tree. The
`pkg/concurrent` package wraps the most common patterns so loaders,
actions and Spores can use them directly.

```go
import "github.com/dev2k6/Nguyen.go/pkg/concurrent"
```

## Pool

A bounded worker pool. Workers consume jobs from a typed channel and
emit typed results. Useful for CPU-bound work like image resizing,
SSR rendering of expensive components, or batch validation.

```go
pool := concurrent.NewPool(ctx, 4, func(ctx context.Context, in ResizeJob) (ResizeResult, error) {
    return resize(ctx, in)
})
defer pool.Close()

result, err := pool.SubmitAndWait(ctx, ResizeJob{Path: "/img.png", Width: 800})
```

`NewPool` clamps `workers` to `runtime.NumCPU()` when zero or negative.
`Submit` returns a result channel for fully async use; `SubmitAndWait`
blocks. `Close` drains in-flight work and rejects new submissions
with `ErrPoolClosed`.

Workers recover from panics and return them as errors, so a single bad
job cannot kill the pool.

## Map

Run a function across every input element with a fixed worker count and
collect results in input order. The first error cancels the rest.

```go
thumbs, err := concurrent.Map(ctx, 4, posts, func(ctx context.Context, p Post) (Thumb, error) {
    return generateThumb(ctx, p)
})
```

## Parallel

Fan-out helper for independent loaders. Every function runs concurrently;
the first non-nil error cancels the rest and is returned.

```go
err := concurrent.Parallel(ctx,
    func(ctx context.Context) error { user, _ = loadUser(ctx, id); return nil },
    func(ctx context.Context) error { posts, _ = loadPosts(ctx, id); return nil },
    func(ctx context.Context) error { profile, _ = loadProfile(ctx, id); return nil },
)
```

This is the typical Nguyen.go loader shape — three queries in flight
at once, total latency bound by the slowest of the three.

## Race

Run several sources concurrently and return the first successful value.
Slower sources have their context cancelled. If every source fails, the
returned error wraps `ErrAllSourcesFailed` and includes the individual
errors.

```go
data, err := concurrent.Race(ctx,
    func(ctx context.Context) (Data, error) { return fetchPrimary(ctx) },
    func(ctx context.Context) (Data, error) { return fetchCache(ctx) },
    func(ctx context.Context) (Data, error) {
        select {
        case <-time.After(50 * time.Millisecond):
            return staleFallback(), nil
        case <-ctx.Done():
            return Data{}, ctx.Err()
        }
    },
)
```

## Deadline

Run a function with a derived context that expires after `d`. The
returned error is `context.DeadlineExceeded` if the function did not
return in time.

```go
out, err := concurrent.Deadline(ctx, 200*time.Millisecond, func(ctx context.Context) (Result, error) {
    return slowAPI.Call(ctx)
})
```

## Lifecycle

Coordinate ordered startup and shutdown of long-lived components: HTTP
server, WebSocket hub, DB pool, cron schedulers, message brokers.
Components register `start` and `stop` callbacks; `Run` blocks until
the context cancels and then runs `stop` in reverse insertion order.

```go
lc := concurrent.NewLifecycle(15 * time.Second)

lc.Add("db",
    func(ctx context.Context) error { return db.Ping(ctx) },
    func(ctx context.Context) error { return db.Close() },
)
lc.Add("http",
    func(ctx context.Context) error { return server.ListenAndServe() },
    func(ctx context.Context) error { return server.Shutdown(ctx) },
)
lc.Add("cron",
    func(ctx context.Context) error { return cron.Run(ctx) },
    nil,
)

ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()
if err := lc.Run(ctx); err != nil {
    log.Fatal(err)
}
```

`Add` panics if called after `Run`. The `stopGrace` argument bounds the
time allowed for any single `stop` call; pass `0` to disable.

## Patterns

### Speculative loading

Race a primary source against a fast cached fallback so users see a
result even when upstream is slow.

```go
data, err := concurrent.Race(ctx, fetchLive, fetchCache)
```

### Bounded background work

A pool with `workers = NumCPU` keeps CPU pinned without overcommit.
For I/O-bound work, set `workers` higher (e.g. `runtime.NumCPU() * 4`).

### Graceful shutdown with grace period

A 15-second grace period gives in-flight requests time to drain while
preventing a stuck connection from blocking the process forever.

```go
lc := concurrent.NewLifecycle(15 * time.Second)
```

## See also

- `pkg/ngctx` — request-scoped context values
- `pkg/observe` — structured logging + Span timing

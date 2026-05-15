# v1.1.0 — Foundation Primitives

Released: 2026-05-15

This release lays the foundation for the next generation of Nguyen.go.
Five Go primitives — goroutines, `context.Context`, `go/ast`, `embed.FS`,
and stdlib observability — are now first-class citizens of the
framework, exposed through new packages, middleware and CLI tooling.

## Highlights

### Concurrency primitives — `pkg/concurrent`

Typed wrappers around the CSP patterns Nguyen.go applications use most.

- **`Pool[J, R]`** — bounded worker pool with typed jobs/results,
  panic recovery and ctx-aware cancellation.
- **`Map`** — fan-out across N workers preserving input order, first
  error cancels the rest.
- **`Parallel`** — fire-and-collect for independent loaders.
- **`Race`** — first-success-wins across multiple sources, with
  `ErrAllSourcesFailed` reporting every failure.
- **`Deadline`** — bound any function with a timeout.
- **`Lifecycle`** — coordinate ordered start/stop of long-lived
  components (HTTP, WS, DB, cron, brokers) with stop-grace.

### Request-scoped context — `pkg/ngctx`

Standard typed keys for request-scoped values:

- `WithTraceID` / `TraceID`
- `WithRequestID` / `RequestID`
- `WithLocale` / `Locale`
- `WithUser` / `User` (carries `ID`, `Email`, `Roles`, `Extra`)
- `WithBudget` / `Budget`
- `FromHTTP(ctx, headers)` populates trace/request/locale from a request

These keys are populated automatically by the new
`server.ContextMiddleware` so loaders and actions read them via
`context.Context` rather than Fiber's Locals map.

### Structured logging + spans — `pkg/observe`

Built on Go 1.21+'s `log/slog`.

- `observe.Default()` / `SetDefault()` — swap the root logger
  (text or JSON) at startup.
- `observe.Logger(ctx)` — returns a logger automatically enriched with
  `trace_id`, `request_id`, `locale`, `user_id` from `ngctx`.
- `observe.NewJSON(w, level)` — production-ready JSON handler for
  Datadog / Loki / GCP Logging.
- `observe.StartSpan(ctx, name)` / `Span.End(err)` — lightweight
  timing helper that emits a structured event when ended.

The framework's HTTP server now emits one `http` event per request
with method, path, status, duration and trace IDs, and writes
`X-Trace-Id` / `X-Request-Id` response headers.

### `nguyen doctor`

Pre-flight diagnostics for the host environment and project layout.
Reports Go version, TinyGo availability, Tailwind binary, presence of
`pages/`, `public/`, `styles/`, `config/nguyen.config.yml`, and git
status. Exits non-zero only on hard prerequisites — safe for CI.

### `nguyen vet`

Static analysis for `.gox` files. Six rules ship in the box:

| Rule | Severity |
|---|---|
| `context-first-param` | warning |
| `no-time-sleep` | warning |
| `goroutine-cancellation` | warning |
| `no-hardcoded-secrets` | error |
| `no-fmt-print` | warning |
| `event-handler-exists` | error |

`--strict` promotes warnings to errors for CI gating. The rule engine is
the seed of the future Effect Wall.

### `nguyen generate routes`

Reads the pages directory, sorts routes deterministically and emits a
typed Go file with one accessor per route. Static routes return
constants; dynamic routes accept typed parameters.

```go
<a href={routes.BlogBySlug(post.Slug)}>{post.Title}</a>
```

A typo becomes a compile error rather than a 404 at runtime. The
generated file is byte-identical across machines, so it diffs cleanly
in code review and works inside reproducible builds.

### `nguyen dist`

Cross-compile reproducible single-binary releases.

```bash
nguyen dist --targets linux/amd64,linux/arm64,darwin/arm64
```

When `--reproducible` (default on), dist sets `-trimpath`,
`-buildvcs=false`, strips symbols and the build ID, and disables CGo.
Two builds from the same source on different machines produce
byte-identical binaries — supply-chain verifiable, air-gap deployable.

## Documentation

Three new guides:

- [docs/concurrency.md](./docs/concurrency.md) — Pool, Map, Parallel, Race, Deadline, Lifecycle
- [docs/observability.md](./docs/observability.md) — context keys, logging, spans
- [docs/tooling.md](./docs/tooling.md) — doctor, vet, generate, dist

[docs/cli.md](./docs/cli.md) and the README are extended with the new
commands.

## Compatibility

This release is fully backward compatible with v1.0.x. No `.gox`
syntax changed. All new packages live under `pkg/concurrent`,
`pkg/ngctx`, `pkg/observe` and do not affect existing imports.

The new `ContextMiddleware` and `AccessLog` are wired into the default
Fiber stack. If you were calling `.Fiber().Use(...)` to register your
own logging middleware, both stacks coexist without conflict, but you
may want to disable yours and read `observe.Logger(ctx)` instead.

## What's next

v1.1.0 is the first half of the Foundation Primitives series. v1.2.0
will land the second half:

- **Spore** — component-as-process model with bounded mailboxes and
  supervision trees
- **Effect Wall** — full effect-class inference using `go/types` so
  build failures replace runtime "DB call from client component"
  surprises
- **Telegram Protocol** — typed streaming wire format for SSR with
  out-of-order rendering and per-frame error recovery

## Install

```bash
go install github.com/dev2k6/Nguyen.go/cmd/nguyen@v1.1.0
```

Or upgrade an existing project:

```bash
go get -u github.com/dev2k6/Nguyen.go@v1.1.0
```

## Author

Thái Nguyên <thainguyen.junior@gmail.com>

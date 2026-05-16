# v1.4.0 — .live.gox Codegen, Effect Wall, Reattach Wire-up

Released: 2026-05-17

v1.4.0 closes the gaps between v1.3.0's release notes and its actual
implementation, ships the `.live.gox` codegen that was the primary
v1.3.1 roadmap item, and adds the first Effect Wall rules to
`nguyen vet`.

---

## Highlights

### `.live.gox` codegen — zero-boilerplate Live pages

Write a Live page as a single file. No `RegisterLivePage` call, no
separate Go struct file:

```
---
var Count int

func inc(state *CounterState) { state.Count++ }
func dec(state *CounterState) { state.Count-- }
---

<section>
  <h1>Count: {Count}</h1>
  <button @click="dec()">-</button>
  <button @click="inc()">+</button>
</section>
```

Save as `pages/counter.live.gox`. The compiler:

1. Detects `File.IsLive` (`.live.gox` suffix or `//+nguyen:live` directive)
2. Calls `parser.TranspileLive` — generates a `livepage.Page` implementation
3. Writes the Go source to `<buildDir>/live/<name>.go`
4. Skips TinyGo entirely — no WASM compilation for live pages

The server scans `<buildDir>/live/` on startup and registers each page
automatically.

### Session reattach — wired end-to-end

`Hub.Reattach` was implemented in v1.3.0 but never called by the
WebSocket handler. v1.4.0 completes the wire-up:

- `readLoop` now checks `evt.Kind == "reattach"` before dispatching
- On valid token: swaps the session reference, logs `live.session.reattach`
- On expired token: sends `{t:"error"}` and closes the connection
- `tokenIndex` is kept alive after `sess.Close()` so reconnects within
  `IdleTimeout` always find the session

### Live stats in `/_nguyen/health`

```json
{
  "status": "ok",
  "version": "1.4.0",
  "live": {
    "sessions": 42,
    "dropped_messages": 0
  }
}
```

`HealthHandler` now accepts a `*live.Hub` and exposes session count
and cumulative dropped message count.

### `WithHubOptions` on `pkg/nguyen.App`

Hub tuning is now fully exposed:

```go
app := nguyen.New(
    nguyen.WithHubOptions(live.Options{
        MaxSessions:    50_000,
        IdleTimeout:    90 * time.Second,
        OutboundBuffer: 32,
    }),
)
```

Previously `live.NewHub` was always called with empty options.

---

## Effect Wall — first live-aware vet rules

`nguyen vet` gains two new rules for `.live.gox` files:

**`live-render-pure`** — flags DB, HTTP, and FS calls inside `Render`
methods. These belong in `Init` or `Handle`:

```
pages/dashboard.live.gox:14: error [live-render-pure]
  Render calls "db." — move side effects to Init or Handle
```

**`live-handler-returns-state`** — flags handler functions bound via
`@event` that have no `return` statement. Live handlers must return
the (possibly mutated) state:

```
pages/counter.live.gox:8: error [live-handler-returns-state]
  handler "inc" in .live.gox has no return statement — it must return the new state
```

These are the first rules that use `go/ast` to inspect `.live.gox`
frontmatter with live-mode awareness, forming the foundation of the
Effect Wall.

---

## Bug fixes / correctness

- `tokenIndex` is no longer cleared on `sess.Close()` — only on reap
  or successful reattach. This was the root cause of reattach always
  returning `ErrTokenExpired` in v1.3.0.
- `TestReattach` and `TestDroppedMessagesCounter` added to
  `internal/live/live_test.go`.

---

## Compatibility

Fully backward compatible with v1.3.x.

New public API:
- `nguyen.WithHubOptions(live.Options)` — new option function
- `server.HealthHandler(*live.Hub)` — signature change (nil-safe; pass
  `nil` for the no-hub case used by `internal/server/fiber_app.go`)
- `parser.TranspileLive(*File) *LiveComponentInfo` — new function
- `parser.LiveComponentInfo` — new type
- `vet.DefaultRules()` — two new rules added

---

## What's next

v1.5 will land:

- Telegram Protocol — typed streaming wire format for SSR with
  out-of-order rendering and per-frame error recovery
- Parallel routes — render multiple independent route segments
  simultaneously (Next.js `@slot` pattern)
- Spore — component-as-process model with bounded mailboxes and
  supervision trees

---

## Install

```bash
go install github.com/dev2k6/Nguyen.go/cmd/nguyen@v1.4.0
go get github.com/dev2k6/Nguyen.go@v1.4.0
```

## Author

Thái Nguyên <thainguyen.junior@gmail.com>

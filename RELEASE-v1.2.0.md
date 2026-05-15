# v1.2.0 — Live Mode

Released: 2026-05-15

Live Mode renders an entire page on the server, opens a WebSocket back
to the same route, and pushes a small VDOM patch each time state
changes. **Interactive UI ships with 0 KB of compiled Go in the
browser** — no TinyGo, no WASM bundle, no client-side state library.

The architecture takes advantage of two Go primitives JavaScript runtimes
cannot match: cheap goroutines (one per WebSocket connection) and
`context.Context` cancellation that automatically cleans up every
in-flight DB query, timer and helper when a session ends. Phoenix
LiveView demonstrated this pattern for Elixir; v1.2.0 brings it
natively to Go without copying its API or jargon.

## Highlights

### `pkg/live` and `pkg/livepage` — public Live Mode contract

```go
// livepage.Page is the contract for a live route.
type Page interface {
    Init(ctx context.Context, params map[string]string) (state any, err error)
    Render(ctx context.Context, state any) (string, error)
    Handle(ctx context.Context, state any, evt live.Event) (any, error)
}
```

State stays on the server, Render emits HTML, Handle reacts to one
client event. Register pages from your `WithSetup` hook:

```go
app := nguyen.New(nguyen.WithPort(3000))
app.RegisterLivePage("/counter", &CounterPage{})
app.Listen("")
```

### `internal/live` — Hub, Session and VDOM differ

- `live.Hub` owns sessions process-wide with `MaxSessions` (default
  10000), `IdleTimeout` (60s), `HeartbeatInterval` (30s) and
  `OutboundBuffer` (16) options.
- `live.Session` runs one goroutine per connection, drains an outbound
  channel, and recovers from panic in handlers (turning them into
  structured error frames so the session stays open).
- `live.Diff(prev, next)` produces minimal patch ops: `text`, `attr`,
  `replace`, `insert`, `remove`, plus a `root` fallback when the tree
  shape changes too much.

### `/_nguyen/live.js` — tiny browser runtime

A ~6 KB script auto-mounted by `App.serve()` discovers
`<div data-nguyen-live="/route">` elements, opens a WebSocket, applies
patches and forwards events. Built-in features:

- Event delegation for `click`, `submit`, `input`, `change`, `keydown`,
  `keyup`, `focus`, `blur`.
- Auto-reconnect with exponential backoff (capped at 30s).
- Server-driven heartbeat (30s ping) keeps NAT/proxy intermediaries
  from idle-closing.
- Form submits and intra-page anchors have their default action
  prevented so the session stays alive.

### `.gox` parser detection

A page opts into Live Mode by either:
- file suffix `.live.gox`, or
- frontmatter directive `//+nguyen:live`.

The parser sets `File.IsLive = true`. A future codegen pass (v1.3) will
turn the frontmatter struct + methods into a `livepage.Page`
automatically.

### Wire protocol

Client → server (one inbound event):

```json
{"t":"event","name":"increment","target":"0/1","value":null}
```

Server → client:

```json
{"t":"replace","html":"<...>"}
{"t":"diff","ops":[{"op":"text","path":"0/1","value":"Count: 4"}]}
{"t":"ping"}
{"t":"error","error":"..."}
```

JSON is used in v1.2.0 for debuggability; binary framing is on the
roadmap once we have real deployments to benchmark against.

## Example

`examples/hello-nguyen` ships a working Live counter:

- `pages/live-counter.gox` — host page with the `<div data-nguyen-live>`
  mount and the `<script src="/_nguyen/live.js">` tag.
- `livedemo/counter.go` — implements `livepage.Page` for `/counter`.
- `cmd/server/main.go` — wires the page in via `app.RegisterLivePage`.

Run it with:

```bash
cd examples/hello-nguyen
go run ./cmd/server
```

Then open `http://localhost:3000/live-counter`.

## Documentation

- [docs/live-mode.md](./docs/live-mode.md) — full guide: architecture,
  page contract, event delegation, patch ops, hub tuning, failure modes
- [docs/agent-reference.md](./docs/agent-reference.md) — Live Mode
  section so AI assistants emit the correct boilerplate
- [README.md](./README.md) — updated feature list

## Compatibility

This release is fully backward compatible with v1.1.x. No `.gox`
syntax changed. Live Mode is opt-in: applications that do not call
`RegisterLivePage` see no extra goroutines, no extra HTTP handlers
beyond `/_nguyen/live.js` (served lazily) and no behavioural change.

## Performance and limits

- One Live session ≈ 10–20 KB of Go state. 10K concurrent sessions
  ≈ 100–200 MB of RAM, well within a single-VM budget.
- Round-trip per interaction is bounded by network latency (typically
  30–100 ms over the public internet, sub-millisecond over a LAN).
- The differ is structural; child-count changes fall back to a
  `replace` of the parent. Keyed reconciliation (`<li :key="...">`)
  is on the v1.3 roadmap.

## When to use Live Mode

- Admin panels, dashboards, internal tooling.
- Form flows, multi-step wizards, anything mostly server-driven.
- Pages where you want **zero client-side bundle** for ergonomic and
  security reasons.

Skip it for sub-frame UI (drag, canvas, animation) or offline-capable
pages — keep the existing CSR / SSR + WASM model for those.

## What's next

v1.3 will land:
- `.live.gox` codegen so a single file with frontmatter + template
  becomes a registered live page.
- Keyed reconciliation in the differ.
- Persistent session reattach (state survives reconnect).
- Streaming partial renders (Telegram Protocol foundation).

## Install

```bash
go install github.com/dev2k6/Nguyen.go/cmd/nguyen@v1.2.0
go get github.com/dev2k6/Nguyen.go@v1.2.0
```

## Author

Thái Nguyên <thainguyen.junior@gmail.com>

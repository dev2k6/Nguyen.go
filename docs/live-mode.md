# Live Mode

Live Mode renders an entire page on the server, opens a WebSocket back
to the same route, and pushes a small VDOM patch each time state
changes. The result: interactive UI with **0 KB of compiled Go in the
browser** — no TinyGo, no WASM bundle, no client state management.

It works because Go is exceptionally good at what Node.js is bad at.
One goroutine per connection plus `context.Context` cancellation makes
holding tens of thousands of live sessions in a single process
affordable. Phoenix LiveView demonstrated the architecture for Elixir;
Live Mode brings the same primitive to Go without copying its API.

## Anatomy

```
┌─────────────┐                       ┌────────────────┐
│   Browser   │  initial SSR HTML <───│  Server        │
│  (~6 KB JS) │  /                    │  + live.Hub    │
│             │  WebSocket  <────────>│  + livepage.   │
│             │  /_nguyen/live/<r>    │    Registry    │
└─────────────┘                       └────────────────┘
       ▲                                     │
       │ event {t:"event", name:"inc"}       │ Page.Handle
       │                                     │ Page.Render
       │ frame {t:"diff", ops:[...]}         │ Diff(prev, next)
       └─────────────────────────────────────┘
```

1. The browser loads the page over normal HTTP. The HTML root contains
   `<div data-nguyen-live="<route>"></div>` plus `<script src="/_nguyen/live.js"></script>`.
2. `live.js` opens a WebSocket to `/_nguyen/live/<route>`.
3. The server upgrades the connection, calls `Page.Init(ctx, params)`,
   spawns a `live.Session` and renders the first frame.
4. User events bubble through delegation in the bridge and ship as
   `{t:"event", name, target, value}` JSON.
5. The session calls `Page.Handle`, then `Page.Render`, diffs against
   the previous output, and sends a list of patch ops back.
6. The bridge applies the ops in place — `text`, `attr`, `replace`,
   `insert`, `remove`, or a full `root` swap.

## Defining a Live page in Go

Implement the `livepage.Page` contract:

```go
package pages

import (
    "context"
    "fmt"

    "github.com/dev2k6/Nguyen.go/internal/live"
)

type CounterState struct {
    Count int
}

type CounterPage struct{}

func (CounterPage) Init(_ context.Context, _ map[string]string) (any, error) {
    return &CounterState{}, nil
}

func (CounterPage) Render(_ context.Context, state any) (string, error) {
    s := state.(*CounterState)
    return fmt.Sprintf(`
        <section>
          <h1>Count: %d</h1>
          <button @click="dec">-</button>
          <button @click="inc">+</button>
        </section>`, s.Count), nil
}

func (CounterPage) Handle(_ context.Context, state any, evt live.Event) (any, error) {
    s := state.(*CounterState)
    switch evt.Name {
    case "inc":
        s.Count++
    case "dec":
        s.Count--
    }
    return s, nil
}
```

Register it before the server starts:

```go
app := nguyen.New(
    nguyen.WithPort(3000),
)
app.RegisterLivePage("/counter", pages.CounterPage{})
app.Listen("")
```

The host page must mount the live root and load the bridge. A simple
`pages/counter.gox` that delegates to the live session:

```html
---
export const metadata = {
    title: "Live Counter",
}
---

<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8" />
  <title>Live Counter</title>
</head>
<body>
  <div data-nguyen-live="/counter"></div>
  <script src="/_nguyen/live.js" defer></script>
</body>
</html>
```

## Defining a Live page in `.gox`

A future codegen pass will let you write a Live page as a single
`.live.gox` file with a struct + methods in the frontmatter and an HTML
template below. Today the parser already detects the file (via the
`.live.gox` suffix or a `//+nguyen:live` directive) and exposes
`File.IsLive`. The runtime side that converts that into a registered
`livepage.Page` ships in v1.3.

For v1.2.0, register pages in Go.

## Event bindings

The bridge handles event delegation. Any element with `@event="name"`
inside a live root sends a frame to the server when the event fires:

```html
<button @click="increment">+</button>
<input @input="search" />
<form @submit="save">...</form>
```

Forms have their default action prevented; anchors with `href` inside a
live root also have their click default prevented so the session does
not navigate away. To break out of the session, use a normal anchor
outside the `data-nguyen-live` element, or call `window.location =`
from a server-driven full reload.

## VDOM patch ops

| Op | Meaning |
|---|---|
| `text` | Replace text content of the node at `path` |
| `attr` | Replace attributes of the node at `path` with `attrs` |
| `replace` | Replace the entire element at `path` with `html` |
| `insert` | Append `html` as the last child of the parent at `path` |
| `remove` | Delete the node at `path` |
| `root` | Replace the live root's `innerHTML` (full re-render) |

`path` is a slash-separated chain of child indices from the live root.
Empty path = root.

The differ today emits `replace` when child counts diverge; keyed
reconciliation (`<li :key="...">`) is on the v1.3 roadmap.

## Tuning the hub

`live.Hub` accepts these options when an application wants more
control:

```go
hub := live.NewHub(live.Options{
    MaxSessions:       100_000, // soft cap; Spawn returns ErrTooManySessions past this
    IdleTimeout:       2 * time.Minute,
    HeartbeatInterval: 30 * time.Second,
    OutboundBuffer:    32,
})
```

Today `pkg/nguyen.App` constructs a hub with defaults. A future
release exposes a `WithLiveOptions` option for full configurability.

## Failure modes

- **Slow client**: outbound buffer fills up; older messages are
  dropped (Session.send falls through). The transport eventually
  observes a write error and closes the session.
- **Network drop**: the bridge auto-reconnects with exponential
  backoff (capped at 30s). On reconnect the server treats it as a
  fresh session — state from the previous session is gone (v1.2 is
  in-memory only).
- **Server panic in handler**: the `live.Session.Dispatch` path
  recovers and emits `{t:"error", error:"..."}`; the session stays
  open so the client can recover.
- **Idle timeout**: sessions that go `IdleTimeout` without an inbound
  event are reaped. `Page.Init` runs again on reconnect.

## When to use Live Mode

Use Live Mode when:

- You want interactive UI with **zero client-side bundle**.
- The page is mostly server-driven (admin panel, dashboard, form
  flows, internal tooling).
- You can tolerate a 30-100 ms round-trip per interaction.
- You want server-only data access without exposing it to a client.

Skip Live Mode when:

- The page needs sub-frame latency (drag-and-drop, canvas, animation).
- You expect heavy concurrent users on a tiny VM (each session = ~10-20
  KB; budget accordingly).
- The page needs to work offline.

For those cases, keep using the existing CSR / SSR + WASM model.

## See also

- `docs/concurrency.md` — `pkg/concurrent.Lifecycle` for graceful
  hub shutdown.
- `docs/observability.md` — how `observe.Logger(ctx)` carries the
  `live_session` and `live_route` attributes.
- `internal/live` — session, hub, diff source.
- `internal/livepage` — Page contract and registry.

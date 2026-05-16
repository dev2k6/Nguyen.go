# v1.3.0 — Keyed Diff, Session Reattach, Bug Fixes

Released: 2026-05-16

v1.3.0 ships the Live Mode improvements roadmapped in v1.2.0, fixes
four correctness bugs that could silently break generated code, and
hardens several security and reliability edges found during codebase
review.

---

## Highlights

### Keyed reconciliation in the server-side differ

The Live Mode VDOM differ now performs O(n) keyed reconciliation when
children carry a `data-key` attribute. Previously any change in child
count caused a full `replace` of the parent element. Now only the
changed nodes are patched:

```html
<ul>
  <li data-key="1">Alice</li>
  <li data-key="2">Bob</li>
</ul>
```

Adding, removing or reordering keyed children emits minimal `insert`
and `remove` ops instead of replacing the whole list. Unkeyed lists
fall back to the previous behaviour — fully backward compatible.

### Persistent session reattach

Live Mode sessions now survive WebSocket reconnects. On first connect
the server issues a 32-hex-char reattach token:

```json
{"t":"session","token":"a3f8..."}
```

The browser stores the token and sends it on reconnect:

```json
{"t":"reattach","token":"a3f8..."}
```

`Hub.Reattach(ctx, token)` looks up the session, resets its transport
context, and hands the existing state back to the new connection. If
the token has expired (session was reaped after `IdleTimeout`) the
server returns `ErrTokenExpired` and the client falls back to a fresh
session gracefully.

### Extended protocol event kinds

`live.Event` now carries three kinds beyond the original `"event"`:

| Kind | New fields | Use case |
|------|-----------|----------|
| `"form"` | `FormData map[string]string` | Full form submission in one frame |
| `"navigate"` | `Path string` | Client-side navigation within a live page |
| `"reattach"` | `Token string` | Reconnect with existing session |

`DecodeEvent` validates the `Kind` field against the known set and
returns an error for unknown kinds, preventing malformed frames from
reaching handlers.

### `WithLiveOptions` on `pkg/nguyen.App`

Hub tuning is now exposed via the programmatic API:

```go
app := nguyen.New(
    nguyen.WithLiveOptions(server.LiveOptions{
        AllowedOrigins: []string{"https://example.com"},
        AuthFunc: func(c *fiber.Ctx) error {
            return myAuth.ValidateSession(c)
        },
    }),
)
```

### Dropped message observability

`Session.DroppedMessages` (atomic counter) increments whenever the
outbound buffer is full and a message is dropped. `Hub.DroppedTotal()`
aggregates across all live sessions and is surfaced in
`/_nguyen/health`:

```json
{
  "live": {
    "sessions": 42,
    "dropped_messages": 0
  }
}
```

---

## Bug fixes

### `ng-for` unused index variable — compile error

`<ng-for>` without an `index` attribute previously emitted
`for i, item := range items` where `i` was never used, causing a Go
compile error in every generated WASM file that used `ng-for`. Fixed:
the loop now emits `for _, item := range items` when `index` is absent.

### Named slot regex — multi-line content silently dropped

`replaceNamedSlots` used `(.*?)` without the `(?s)` flag, so any slot
content spanning more than one line was silently discarded. Fixed with
`(?s)(.*?)`.

### `tokenizeHTML` — `<script>`/`<style>` content corrupted

The transpiler's HTML tokenizer treated `<` inside `<script>` and
`<style>` blocks as tag opens, corrupting the parse tree. Fixed: after
opening a raw-text tag the tokenizer now scans forward to the matching
close tag and emits the inner content as a single text token.

### WASM reconciler — event delegation not wired

`mountToDOM` in `pkg/core/reconciler.go` silently skipped `onClick`,
`onChange` and other `on*` props with a bare `continue`. Fixed: these
props are now written as `data-nguyen-{event}` attributes so the
existing `SetupEventDelegation` listener picks them up correctly.

---

## Security / correctness

| Item | Detail |
|------|--------|
| `WrapHTML` locale | Accepts a `locale string` param; no longer hardcodes `lang="en"`. Callers pass the i18n locale from request context. |
| CSP nonce | `injectHydrationMarkers` accepts a `nonce string`; emits `<script nonce="...">` when non-empty. Pass `c.Locals("csp_nonce")` from your middleware. |
| `pathToHash` | Extended from 48-bit (12 hex chars) to 64-bit (16 hex chars), eliminating birthday collision risk at scale. |
| `RevalidateTag` | Now calls `BackgroundRevalidate` instead of the synchronous `RevalidatePath`, so a tag shared by many pages no longer blocks the caller. |
| ISR Lifecycle | `NewISRWithContext(ctx, dir)` integrates the cleanup goroutine with `pkg/concurrent.Lifecycle`. `NewISR` remains as a backward-compatible wrapper. |

---

## Compatibility

Fully backward compatible with v1.2.x. The only public API additions are:

- `live.ErrTokenExpired` — new sentinel error
- `live.Event.FormData`, `live.Event.Path`, `live.Event.Token` — new fields (zero-value safe)
- `live.Outbound.Token` — new field
- `live.Hub.Reattach(ctx, token)` — new method
- `live.Hub.DroppedTotal()` — new method
- `live.Session.DroppedMessages` — exported atomic field
- `live.Session.ReattachToken` — exported field
- `render.WrapHTML(body, result, locale)` — added `locale` param (pass `""` for `"en"`)
- `render.injectHydrationMarkers` — internal, nonce param (empty = no nonce)
- `cache.NewISRWithContext(ctx, dir)` — new constructor

---

## What's next

v1.3.1 will land:

- `.live.gox` codegen — a single file with frontmatter struct + methods
  + HTML template becomes a registered `livepage.Page` automatically.
  The parser already sets `File.IsLive = true`; the compiler pass is
  the remaining work.
- Streaming partial renders (Telegram Protocol foundation).

---

## Install

```bash
go install github.com/dev2k6/Nguyen.go/cmd/nguyen@v1.3.0
go get github.com/dev2k6/Nguyen.go@v1.3.0
```

## Author

Thái Nguyên <thainguyen.junior@gmail.com>

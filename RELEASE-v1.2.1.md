# v1.2.1 — Security Patch

Released: 2026-05-15

This is a security patch release for v1.2.0. All users running Live
Mode should upgrade immediately. No API changes; fully backward
compatible.

## Security fixes

### Critical: Cross-Site WebSocket Hijacking (CSWSH) — CVE class

**File**: `internal/server/live_handler.go`

`LiveUpgradeMiddleware` previously accepted WebSocket upgrades from any
origin. A malicious website could open `wss://victim/_nguyen/live/<route>`
in a victim's browser, which would automatically send the victim's
session cookies. The attacker could then receive full page HTML and
push arbitrary events to the server.

**Fix**: `LiveUpgradeMiddleware` now accepts a `LiveOptions` struct with
an `AllowedOrigins` field. The default (empty slice) allows only
same-origin connections. Pass `[]string{"*"}` in development to restore
the old permissive behaviour.

```go
app := nguyen.New(
    nguyen.WithLiveOptions(server.LiveOptions{
        AllowedOrigins: []string{"https://example.com"},
    }),
)
```

### Critical: No authentication on live WebSocket endpoint

**File**: `internal/server/live_handler.go`, `pkg/nguyen/option.go`

The `/_nguyen/live/<route>` endpoint had no authentication hook. Any
unauthenticated client could open a live session.

**Fix**: `LiveOptions.AuthFunc` is a new optional hook called before a
session is spawned. Return a non-nil error to reject the connection
with 403.

```go
nguyen.WithLiveOptions(server.LiveOptions{
    AuthFunc: func(c *fiber.Ctx) error {
        user := ngctx.User(c.UserContext())
        if user == nil {
            return errors.New("unauthenticated")
        }
        return nil
    },
})
```

### High: WebSocket keepalive used JSON text frame instead of control frame

**File**: `internal/server/live_handler.go`, `internal/server/live.js`

The server sent `{"t":"ping"}` as a text frame for keepalive. This did
not reset idle timers on nginx/ALB proxies (typically 60s), causing
silent connection drops. The browser also had to parse and respond to
the JSON frame manually.

**Fix**: The server now sends a WebSocket `PingMessage` control frame
every 25 seconds. The browser responds with a Pong automatically; all
intermediate proxies reset their idle timers. The JSON `ping` case is
removed from `live.js`.

### High: JWT secret had no minimum length validation

**File**: `internal/auth/auth.go`

`auth.New()` accepted any string as `JWTSecret`, including empty
strings. An empty or short secret makes HMAC-SHA256 trivially
brute-forceable.

**Fix**: `auth.New()` now panics at startup if `JWTSecret` is shorter
than 32 characters with a clear message:
`auth: JWTSecret must be at least 32 characters`.

### High: Hub.Run goroutine and Spawn race with Stop

**File**: `internal/live/live.go`

Two related issues:
1. `Hub.Spawn` did not check `rootCtx.Err()` under the lock, so a
   session could be inserted into the map after `Stop()` had cancelled
   `rootCtx`. The session's context would be immediately cancelled but
   the session would still appear in the map until the cleanup goroutine
   ran.
2. `Hub.Run` documentation was unclear about the relationship between
   the external `ctx` parameter and `rootCtx`.

**Fix**: `Spawn` now checks `h.rootCtx.Err()` while holding `h.mu`,
returning `"live: hub is stopped"` immediately. `Run` documentation
clarifies that the external ctx stops the reaper only; `Stop()` is
required to close sessions.

### Medium: Session.state mutation was not protected by a lock

**File**: `internal/live/live.go`

`Session.Dispatch` wrote to `s.state` and `s.lastHTML` without holding
`s.mu`. While `Dispatch` is called from a single goroutine in the
current transport, the exported API made concurrent calls possible.

**Fix**: `Dispatch` now holds `s.mu` around state and lastHTML
mutations. The lock is released before calling `Render` (which may be
slow) to avoid holding it longer than necessary.

## Upgrade

```bash
go get github.com/dev2k6/Nguyen.go@v1.2.1
```

If you are using Live Mode, add `WithLiveOptions` to your app
configuration. At minimum, set `AllowedOrigins` to your production
domain and provide an `AuthFunc` if your pages require authentication.

## Author

Thái Nguyên <thainguyen.junior@gmail.com>

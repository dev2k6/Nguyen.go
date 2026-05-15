package server

import (
	"context"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/dev2k6/Nguyen.go/internal/live"
	internallivepage "github.com/dev2k6/Nguyen.go/internal/livepage"
	"github.com/dev2k6/Nguyen.go/pkg/ngctx"
	"github.com/dev2k6/Nguyen.go/pkg/observe"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
)

// LiveOptions configures the Live Mode WebSocket endpoint.
type LiveOptions struct {
	// AllowedOrigins is the list of origins permitted to open a live
	// WebSocket. Each entry is matched against the request's Origin
	// header. An empty list defaults to same-origin only (derived from
	// the Host header). Pass []string{"*"} to allow all origins — only
	// do this in development.
	AllowedOrigins []string

	// AuthFunc is an optional hook called before a session is spawned.
	// Return a non-nil error to reject the connection with a 403. Use
	// it to validate JWT cookies, session tokens, or any other
	// application-level credential. The context carries ngctx values
	// (TraceID, RequestID) populated by ContextMiddleware.
	AuthFunc func(c *fiber.Ctx) error
}

// LiveUpgradeMiddleware validates the WebSocket upgrade request:
//  1. Rejects non-upgrade requests.
//  2. Validates the Origin header against opts.AllowedOrigins (CSWSH
//     protection — CVE class: Cross-Site WebSocket Hijacking).
//  3. Runs opts.AuthFunc when provided so callers can enforce JWT /
//     session auth before the socket is opened.
//  4. Captures query params and tracing IDs into c.Locals for the
//     downstream LiveHandler.
func LiveUpgradeMiddleware(opts LiveOptions) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if !websocket.IsWebSocketUpgrade(c) {
			return fiber.ErrUpgradeRequired
		}

		// --- Origin validation (CSWSH protection) ---
		origin := c.Get("Origin")
		if !isOriginAllowed(origin, c.Hostname(), opts.AllowedOrigins) {
			return c.Status(fiber.StatusForbidden).
				JSON(fiber.Map{"error": "origin not allowed"})
		}

		// --- Application-level auth ---
		if opts.AuthFunc != nil {
			if err := opts.AuthFunc(c); err != nil {
				return c.Status(fiber.StatusForbidden).
					JSON(fiber.Map{"error": "unauthorized"})
			}
		}

		// Capture query params before the upgrade so LiveHandler can
		// pass them to Page.Init without depending on websocket.Conn.
		queries := map[string]string{}
		c.Context().QueryArgs().VisitAll(func(k, v []byte) {
			queries[string(k)] = string(v)
		})
		c.Locals("liveAllowed", true)
		c.Locals("liveQueries", queries)
		if tid := ngctx.TraceID(c.UserContext()); tid != "" {
			c.Locals("traceID", tid)
		}
		if rid := ngctx.RequestID(c.UserContext()); rid != "" {
			c.Locals("requestID", rid)
		}
		return c.Next()
	}
}

// isOriginAllowed reports whether origin is permitted.
//
//   - If allowedOrigins is empty, only same-origin requests are allowed
//     (origin host == serverHost).
//   - If allowedOrigins contains "*", all origins are allowed (dev only).
//   - Otherwise the origin's host is matched against each entry.
func isOriginAllowed(origin, serverHost string, allowed []string) bool {
	// Requests without an Origin header (e.g. curl, server-to-server)
	// are allowed — browsers always send Origin on WS upgrades.
	if origin == "" {
		return true
	}

	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	originHost := u.Hostname()

	if len(allowed) == 0 {
		// Default: same-origin only.
		return strings.EqualFold(originHost, serverHost)
	}

	for _, a := range allowed {
		if a == "*" {
			return true
		}
		if strings.EqualFold(originHost, a) {
			return true
		}
	}
	return false
}

// ngctxRequestIDKey is intentionally unexported — ngctx exposes typed
// accessors; we never look up its private keys directly.
type ngctxRequestIDKey struct{}

// LiveHandler returns a Fiber handler that upgrades the request to a
// WebSocket and binds it to one Live Mode session. Each connection
// runs three goroutines (reader, writer, heartbeat) and exits cleanly
// when any of them detects a closed socket or a cancelled context.
func LiveHandler(hub *live.Hub, registry *internallivepage.Registry) fiber.Handler {
	return websocket.New(func(c *websocket.Conn) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		if v, ok := c.Locals("traceID").(string); ok && v != "" {
			ctx = ngctx.WithTraceID(ctx, v)
		}
		if v, ok := c.Locals("requestID").(string); ok && v != "" {
			ctx = ngctx.WithRequestID(ctx, v)
		}
		log := observe.Logger(ctx)

		route := normaliseRoute(c.Params("*"))
		page := registry.Get(route)
		if page == nil {
			writeFatal(c, "no live page registered for route "+route)
			return
		}

		params := map[string]string{}
		if rawQueries, ok := c.Locals("liveQueries").(map[string]string); ok {
			for k, v := range rawQueries {
				params[k] = v
			}
		}

		state, err := page.Init(ctx, params)
		if err != nil {
			writeFatal(c, "init: "+err.Error())
			return
		}

		adapter := internallivepage.Adapter{Page: page}
		sess, err := hub.Spawn(ctx, ngctx.NewID(), route, state, adapter, adapter)
		if err != nil {
			writeFatal(c, "spawn: "+err.Error())
			return
		}
		log = log.With("live_session", sess.ID(), "live_route", route)
		log.Info("live.session.open")
		defer func() {
			sess.Close()
			log.Info("live.session.close")
		}()

		sess.Dispatch(live.Event{Kind: "init"})

		writeDone := make(chan struct{})
		go writeLoop(c, sess, writeDone, log)
		readLoop(c, sess, log)
		<-writeDone
	})
}

func writeFatal(c *websocket.Conn, msg string) {
	out, _ := live.EncodeOutbound(live.Outbound{Kind: "error", Err: msg})
	_ = c.WriteMessage(websocket.TextMessage, out)
	_ = c.Close()
}

func readLoop(c *websocket.Conn, sess *live.Session, log *slog.Logger) {
	for {
		select {
		case <-sess.Closed():
			return
		default:
		}
		// Reset read deadline on every iteration so the connection
		// stays alive as long as the client is active.
		_ = c.SetReadDeadline(time.Now().Add(2 * time.Minute))
		mt, data, err := c.ReadMessage()
		if err != nil {
			sess.Close()
			return
		}
		if mt != websocket.TextMessage {
			continue
		}
		evt, err := live.DecodeEvent(data)
		if err != nil {
			log.Info("live.decode.error", "error", err.Error())
			continue
		}
		sess.Dispatch(evt)
	}
}

func writeLoop(c *websocket.Conn, sess *live.Session, done chan<- struct{}, log *slog.Logger) {
	defer close(done)
	// Use a 25s ticker so the control-frame ping fires before most
	// proxy idle timeouts (typically 60s). A WebSocket PingMessage
	// control frame is used — not a JSON text frame — so the browser
	// and any intermediate proxy reset their idle timers automatically.
	ping := time.NewTicker(25 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-sess.Closed():
			return
		case <-ping.C:
			// Send a WebSocket control-frame ping. The browser responds
			// with a Pong automatically; proxies reset their idle timer.
			if err := c.WriteMessage(websocket.PingMessage, nil); err != nil {
				sess.Close()
				return
			}
		case msg, ok := <-sess.OutboundCh():
			if !ok {
				return
			}
			frame, err := live.EncodeOutbound(msg)
			if err != nil {
				log.Info("live.encode.error", "error", err.Error())
				continue
			}
			if err := c.WriteMessage(websocket.TextMessage, frame); err != nil {
				sess.Close()
				return
			}
		}
	}
}

func normaliseRoute(p string) string {
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	if p != "/" {
		p = strings.TrimRight(p, "/")
	}
	return p
}

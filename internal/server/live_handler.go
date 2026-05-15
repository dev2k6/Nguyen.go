package server

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/dev2k6/Nguyen.go/internal/live"
	internallivepage "github.com/dev2k6/Nguyen.go/internal/livepage"
	"github.com/dev2k6/Nguyen.go/pkg/ngctx"
	"github.com/dev2k6/Nguyen.go/pkg/observe"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
)

// LiveUpgradeMiddleware allows the WebSocket upgrade only on requests
// that target a Live Mode endpoint. Mount it before LiveHandler. It
// also captures the request's query parameters into c.Locals so the
// handler can pass them to Page.Init without depending on the
// websocket-bound Conn.
func LiveUpgradeMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if !websocket.IsWebSocketUpgrade(c) {
			return fiber.ErrUpgradeRequired
		}
		queries := map[string]string{}
		c.Context().QueryArgs().VisitAll(func(k, v []byte) {
			queries[string(k)] = string(v)
		})
		c.Locals("liveAllowed", true)
		c.Locals("liveQueries", queries)
		if rid, ok := c.UserContext().Value(ngctxRequestIDKey{}).(string); ok {
			c.Locals("requestID", rid)
		}
		if tid := ngctx.TraceID(c.UserContext()); tid != "" {
			c.Locals("traceID", tid)
		}
		if rid := ngctx.RequestID(c.UserContext()); rid != "" {
			c.Locals("requestID", rid)
		}
		return c.Next()
	}
}

// ngctxRequestIDKey is intentionally unused — kept to document that
// ngctx exposes typed accessors and we should not look up its private
// keys directly. The middleware uses the public ngctx.TraceID /
// RequestID helpers instead.
type ngctxRequestIDKey struct{}

// LiveHandler returns a Fiber handler that upgrades the request to a
// WebSocket and binds it to one Live Mode session. Each connection
// runs three goroutines (reader, writer, heartbeat) and exits cleanly
// when any of them detects a closed socket or a cancelled context.
//
// The path captured at "*" decides which page is served. Pages must
// already be registered in the supplied registry; an unknown route
// receives a single error frame and the socket closes.
func LiveHandler(hub *live.Hub, registry *internallivepage.Registry) fiber.Handler {
	return websocket.New(func(c *websocket.Conn) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Carry tracing IDs from the upgraded request when present.
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

		// Initial render so the client gets a frame immediately.
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
	ping := time.NewTicker(30 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-sess.Closed():
			return
		case <-ping.C:
			pingFrame, _ := live.EncodeOutbound(live.Outbound{Kind: "ping"})
			if err := c.WriteMessage(websocket.TextMessage, pingFrame); err != nil {
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

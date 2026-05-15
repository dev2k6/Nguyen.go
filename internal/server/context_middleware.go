package server

import (
	"time"

	"github.com/dev2k6/Nguyen.go/pkg/ngctx"
	"github.com/dev2k6/Nguyen.go/pkg/observe"
	"github.com/gofiber/fiber/v2"
)

// ContextMiddleware threads ngctx values onto every request's user
// context so loaders, actions and downstream services can read trace
// IDs, request IDs and locales without the Fiber-specific Locals API.
//
// It also writes the trace and request IDs back as response headers so
// clients and edge proxies see consistent identifiers.
func ContextMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := c.UserContext()

		tid := c.Get("X-Trace-Id")
		if tid == "" {
			tid = ngctx.NewID()
		}
		rid := c.Get("X-Request-Id")
		if rid == "" {
			rid = ngctx.NewID()
		}
		ctx = ngctx.WithTraceID(ctx, tid)
		ctx = ngctx.WithRequestID(ctx, rid)
		if al := c.Get("Accept-Language"); al != "" {
			ctx = ngctx.WithLocale(ctx, primaryAcceptLang(al))
		}

		c.SetUserContext(ctx)
		c.Set("X-Trace-Id", tid)
		c.Set("X-Request-Id", rid)
		c.Locals("requestID", rid)
		return c.Next()
	}
}

// AccessLog emits one structured slog event per request with method,
// path, status, duration and the trace/request IDs threaded through
// ContextMiddleware. Call after ContextMiddleware so the trace id is
// already populated.
func AccessLog() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()

		ctx := c.UserContext()
		log := observe.Logger(ctx)
		log.Info("http",
			"method", c.Method(),
			"path", c.Path(),
			"status", c.Response().StatusCode(),
			"duration_ms", time.Since(start).Milliseconds(),
			"ip", c.IP(),
		)
		return err
	}
}

func primaryAcceptLang(al string) string {
	for i := 0; i < len(al); i++ {
		switch al[i] {
		case ',', ';', ' ':
			return al[:i]
		}
	}
	return al
}

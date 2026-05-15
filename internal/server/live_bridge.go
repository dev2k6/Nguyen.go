package server

import (
	_ "embed"

	"github.com/gofiber/fiber/v2"
)

//go:embed live.js
var liveJS []byte

// LiveBridgeHandler serves the small JavaScript runtime that Live Mode
// pages embed in their HTML. Mounted at /_nguyen/live.js by App.serve.
func LiveBridgeHandler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Set("Content-Type", "application/javascript; charset=utf-8")
		c.Set("Cache-Control", "public, max-age=3600")
		return c.Send(liveJS)
	}
}

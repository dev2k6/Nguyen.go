package server

import (
	"github.com/dev2k6/Nguyen.go/internal/parser"
	"github.com/dev2k6/Nguyen.go/internal/render"
	"github.com/dev2k6/Nguyen.go/internal/router"

	"github.com/gofiber/fiber/v2"
)

// NavigateHandlerConfig holds dependencies for the SPA navigate endpoint.
type NavigateHandlerConfig struct {
	Routes  []router.Route
	Layouts []router.LayoutInfo
	Version string
}

// NavigateHandler returns a Fiber handler for /_nguyen/navigate.
// It renders partial HTML for client-side page transitions.
func NavigateHandler(cfg NavigateHandlerConfig) fiber.Handler {
	return func(c *fiber.Ctx) error {
		targetPath := c.Query("path")
		fromPath := c.Query("from")

		if targetPath == "" {
			return c.Status(400).JSON(fiber.Map{"error": "missing path parameter"})
		}

		// Match target route
		matched, _, found := router.MatchRoute(cfg.Routes, targetPath)
		if !found {
			return c.Status(404).JSON(fiber.Map{"error": "route not found"})
		}

		// Parse .nguyen file
		ngFile, err := parser.Parse(matched.FilePath)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "parse failed"})
		}

		// Render partial
		partial := render.RenderPartial(ngFile, cfg.Version, cfg.Layouts, fromPath, cfg.Routes)

		// Encode binary frame
		frame, err := render.EncodePartialFrame(partial)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "encode failed"})
		}

		c.Set("Content-Type", "application/x-nguyen-partial")
		c.Set("Cache-Control", "private, max-age=30")
		c.Set("X-Nguyen-Outlet", render.Itoa(partial.Meta.Outlet))
		return c.Send(frame)
	}
}

package api

import (
	"github.com/gofiber/fiber/v2"
)

// Register mounts all API routes under /api.
func Register(app fiber.Router) {
	api := app.Group("/api")

	api.Get("/health", healthHandler)
	api.Get("/version", versionHandler)

	posts := api.Group("/posts")
	posts.Get("/", listPostsHandler)
	posts.Get("/:slug", getPostHandler)
}

func healthHandler(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status":  "ok",
		"version": "1.0.0",
	})
}

func versionHandler(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"version":    "0.1.0",
		"go_version": "1.23",
		"features": []string{
			"CSR via WebAssembly",
			"File-based routing",
			"Reactive hooks",
			"Scoped CSS",
			"CLI tooling",
			"Hot Module Replacement",
			"GEO optimization",
		},
	})
}

var posts = []fiber.Map{
	{
		"slug":    "getting-started",
		"title":   "Getting Started with Nguyen.go",
		"date":    "2026-05-01",
		"author":  "Nguyen.go Team",
		"excerpt": "Learn how to set up your first Nguyen.go project in under 5 minutes.",
	},
	{
		"slug":    "why-go-wasm",
		"title":   "Why Go + WebAssembly is the Future",
		"date":    "2026-04-28",
		"author":  "Nguyen.go Team",
		"excerpt": "Exploring the performance and DX benefits of Go in the browser.",
	},
	{
		"slug":    "ssr-vs-csr",
		"title":   "SSR vs CSR: When to Use Which",
		"date":    "2026-04-15",
		"author":  "Nguyen.go Team",
		"excerpt": "A practical guide to choosing the right rendering strategy.",
	},
}

func listPostsHandler(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"posts": posts,
		"total": len(posts),
	})
}

func getPostHandler(c *fiber.Ctx) error {
	slug := c.Params("slug")
	for _, p := range posts {
		if p["slug"] == slug {
			return c.JSON(fiber.Map{"post": p})
		}
	}
	return c.Status(404).JSON(fiber.Map{"error": "Post not found"})
}

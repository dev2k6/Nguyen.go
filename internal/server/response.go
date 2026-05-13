package server

import (
	"github.com/gofiber/fiber/v2"
)

// Response provides chainable response helpers for API routes.
// Usage in api/routes.go:
//
//	app.Get("/api/users", func(c *fiber.Ctx) error {
//	    users := fetchUsers()
//	    return server.OK(c, users)
//	})
type Response struct{}

// OK sends a 200 JSON response with data.
func OK(c *fiber.Ctx, data interface{}) error {
	return c.JSON(fiber.Map{"data": data})
}

// Created sends a 201 JSON response.
func Created(c *fiber.Ctx, data interface{}) error {
	return c.Status(201).JSON(fiber.Map{"data": data})
}

// NoContent sends a 204 with no body.
func NoContent(c *fiber.Ctx) error {
	return c.SendStatus(204)
}

// BadRequest sends a 400 error response.
func BadRequest(c *fiber.Ctx, message string) error {
	return c.Status(400).JSON(fiber.Map{"error": message})
}

// Unauthorized sends a 401 error response.
func Unauthorized(c *fiber.Ctx, message string) error {
	if message == "" {
		message = "Unauthorized"
	}
	return c.Status(401).JSON(fiber.Map{"error": message})
}

// Forbidden sends a 403 error response.
func Forbidden(c *fiber.Ctx, message string) error {
	if message == "" {
		message = "Forbidden"
	}
	return c.Status(403).JSON(fiber.Map{"error": message})
}

// NotFound sends a 404 error response.
func NotFound(c *fiber.Ctx, message string) error {
	if message == "" {
		message = "Not Found"
	}
	return c.Status(404).JSON(fiber.Map{"error": message})
}

// ServerError sends a 500 error response.
func ServerError(c *fiber.Ctx, message string) error {
	if message == "" {
		message = "Internal Server Error"
	}
	return c.Status(500).JSON(fiber.Map{"error": message})
}

// Redirect sends a 302 redirect.
func Redirect(c *fiber.Ctx, url string) error {
	return c.Redirect(url, 302)
}

// RedirectPermanent sends a 301 redirect.
func RedirectPermanent(c *fiber.Ctx, url string) error {
	return c.Redirect(url, 301)
}

// Paginated sends a paginated JSON response with metadata.
func Paginated(c *fiber.Ctx, data interface{}, page, perPage, total int) error {
	totalPages := total / perPage
	if total%perPage != 0 {
		totalPages++
	}
	return c.JSON(fiber.Map{
		"data": data,
		"meta": fiber.Map{
			"page":        page,
			"per_page":    perPage,
			"total":       total,
			"total_pages": totalPages,
			"has_next":    page < totalPages,
			"has_prev":    page > 1,
		},
	})
}
